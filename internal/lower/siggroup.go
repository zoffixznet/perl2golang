package lower

import (
	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// sigGroup ties together the function values that share one slot: the
// closures in a dispatch table, the stages of a pipeline held in one array,
// the two subs a conditional can leave in one variable. Go gives that slot
// exactly one type, so everything that can land in it has to agree on a
// signature, and the group is where the agreement is worked out.
//
// A group grows two ways. Every anonymous sub starts in a group of its own,
// and whenever two function types meet in a join, their groups merge,
// because a join happens exactly where two values flow into one place. Call
// sites through the slot feed the group the types they pass, which is how a
// closure whose body never says what it takes gets its parameters typed:
// `$counters{hit}->(1)` is the only evidence about $by there is.
type sigGroup struct {
	// parent is set when this group has been absorbed into another; find
	// follows it, union-find style, because type values across the file
	// still point at the old group.
	parent  *sigGroup
	members []*Sub

	// argEv holds the types call sites passed at each fixed position, and
	// spreadEv the element types of whole lists passed at once.
	argEv    [][]observation
	spreadEv []observation
	// sawSpread records that some call passes a run of values whose length
	// only the program knows, which rules out a fixed parameter list.
	sawSpread bool

	// The settled decision, renewed at the end of every discovery round.
	decided bool
	// fixed says the members take a fixed parameter list rather than a
	// variadic slice.
	fixed bool
	// demoted records that the members' parameters would not settle on one
	// type per position, so the fixed list was given up for good in favour
	// of the variadic slice. It never unsets: flapping between the two
	// shapes would keep discovery from settling.
	demoted bool
	// arity is the fixed list's width.
	arity int
	// params are the fixed positions' types as the call sites showed them,
	// and settledParams the types the members' own bindings agreed on,
	// which is what a padded blank position is declared as.
	params        []*ir.Type
	settledParams []*ir.Type
	// elem is the variadic slice's element type.
	elem *ir.Type
	// result is the first result position every member answers with,
	// unified is the whole result shape, and returns says whether any
	// member answers at all.
	result  *ir.Type
	unified []*ir.Type
	returns bool
}

// group follows a sub's signature group to the one that speaks for it.
func (s *Sub) group() *sigGroup {
	if s.Group == nil {
		return nil
	}
	return s.Group.find()
}

// settledParam is the declared type of a fixed position, preferring what
// the members' own parameters settled on over what the call sites passed.
func (g *sigGroup) settledParam(i int) *ir.Type {
	if i < len(g.settledParams) && g.settledParams[i] != nil {
		return g.settledParams[i]
	}
	if i < len(g.params) && g.params[i] != nil && !isUnresolved(g.params[i]) {
		return g.params[i]
	}
	return ir.TAny
}

// find follows absorbed groups to the one that speaks for them now.
func (g *sigGroup) find() *sigGroup {
	for g.parent != nil {
		g = g.parent
	}
	return g
}

// subFuncType is the function type a reference to a named sub carries: the
// signature the sub has settled on so far, tagged with its signature group
// so that storing the reference beside closures merges their groups.
//
// Methods and constructors stay untyped here: their Go signature carries a
// receiver, which a plain function value cannot spell without the method
// expression form, and no corpus script has asked for one yet.
func (l *Lowerer) subFuncType(s *Sub) *ir.Type {
	if s.Class != nil {
		return nil
	}
	if s.Group == nil {
		s.Group = &sigGroup{members: []*Sub{s}}
	}
	var params []*ir.Type
	for _, b := range s.Params {
		if b == nil {
			return nil
		}
		params = append(params, b.Type)
	}
	t := ir.FuncOf(params, s.Results)
	if s.VarArgs != nil {
		t.Params = append(t.Params, elemOf(s.VarArgs.Type))
		t.Variadic = true
	}
	t.Group = s.Group
	return t
}

// sansGroups strips the signature groups off function types, for the joins
// that ask "what one type could hold all of these" without the values
// actually sharing a slot. A sub returning two closures returns them in two
// result positions; flattening the results into a list joins their types to
// find the list's element type, and that join must not marry the closures'
// signatures for good on the strength of a question.
func sansGroups(ts []*ir.Type) []*ir.Type {
	out := make([]*ir.Type, len(ts))
	for i, t := range ts {
		if t != nil && t.Kind == ir.Func && t.Group != nil {
			c := *t
			c.Group = nil
			out[i] = &c
			continue
		}
		out[i] = t
	}
	return out
}

// groupOf reads the signature group off a function type, if it carries one.
func groupOf(t *ir.Type) *sigGroup {
	if t == nil || t.Kind != ir.Func {
		return nil
	}
	g, _ := t.Group.(*sigGroup)
	if g == nil {
		return nil
	}
	return g.find()
}

// mergeSigGroups is the union half of the union-find: two function types
// meeting in a join means their values share a slot, so their groups become
// one. Either side may be missing, since function types are built in plenty
// of places that know nothing of groups.
func mergeSigGroups(a, b any) any {
	ga, _ := a.(*sigGroup)
	gb, _ := b.(*sigGroup)
	if ga != nil {
		ga = ga.find()
	}
	if gb != nil {
		gb = gb.find()
	}
	switch {
	case ga == nil:
		return gb
	case gb == nil, ga == gb:
		return ga
	}
	for _, m := range gb.members {
		m.Group = ga
	}
	ga.members = append(ga.members, gb.members...)
	gb.members = nil
	for i, evs := range gb.argEv {
		for len(ga.argEv) <= i {
			ga.argEv = append(ga.argEv, nil)
		}
		ga.argEv[i] = append(ga.argEv[i], evs...)
	}
	ga.spreadEv = append(ga.spreadEv, gb.spreadEv...)
	ga.sawSpread = ga.sawSpread || gb.sawSpread
	gb.parent = ga
	return ga
}

// observeCall records what one call through the shared slot passed: the
// leading single values by position, and any whole list by its element
// type, after which the positions stop being knowable.
func (g *sigGroup) observeCall(l *Lowerer, args []ir.Expr) {
	if l.pass != 1 {
		return
	}
	g = g.find()
	pos := 0
	spread := false
	for _, a := range args {
		at := typeOrAny(a)
		// A slice-typed argument stands for a run of values, exactly as the
		// call spread machinery treats it.
		if at.Kind == ir.Slice {
			spread = true
			g.spreadEv = l.recordObservation(g.spreadEv, elemOf(at))
			continue
		}
		if spread {
			g.spreadEv = l.recordObservation(g.spreadEv, at)
			continue
		}
		for len(g.argEv) <= pos {
			g.argEv = append(g.argEv, nil)
		}
		g.argEv[pos] = l.recordObservation(g.argEv[pos], at)
		pos++
	}
	if spread {
		g.sawSpread = true
	}
}

// eachGroup visits every signature group once. Groups hang off anonymous
// subs and off the named subs the file takes references to.
func (l *Lowerer) eachGroup(f func(*sigGroup)) {
	seen := map[*sigGroup]bool{}
	visit := func(s *Sub) {
		if s == nil || s.Group == nil {
			return
		}
		g := s.Group.find()
		if seen[g] {
			return
		}
		seen[g] = true
		f(g)
	}
	for _, s := range l.anonOrd {
		visit(s)
	}
	for _, name := range l.subOrd {
		visit(l.subs[name])
	}
}

// settleGroups renews every group's decision from this round's evidence and
// feeds what the call sites showed into the members' parameter bindings, so
// the ordinary settling that follows sees it as evidence like any other.
func (l *Lowerer) settleGroups() {
	l.eachGroup(func(g *sigGroup) { l.settleGroup(g) })
}

func (l *Lowerer) settleGroup(g *sigGroup) {
	if l.stickyEvidence {
		for i := range g.argEv {
			g.argEv[i] = compactFuncEvidence(g.argEv[i])
		}
		g.spreadEv = compactFuncEvidence(g.spreadEv)
	} else {
		for i := range g.argEv {
			g.argEv[i] = compactEvidence(g.argEv[i])
		}
		g.spreadEv = compactEvidence(g.spreadEv)
	}

	// The members' own bodies say how wide a fixed parameter list would be
	// and whether one is possible at all: a member that reads @_ raw, or a
	// call that passes a whole list, leaves the width to the program.
	arity := 0
	named := true
	for _, m := range g.members {
		a, ok := m.naturalShape()
		if !ok {
			named = false
		}
		if a > arity {
			arity = a
		}
	}
	for i, evs := range g.argEv {
		if len(evs) > 0 && i+1 > arity {
			arity = i + 1
		}
	}

	g.arity = arity
	g.fixed = named && !g.sawSpread && !g.demoted && len(g.members) > 1
	g.params = make([]*ir.Type, arity)
	for i := range g.params {
		if i < len(g.argEv) {
			g.params[i] = joinAll(observedTypes(g.argEv[i]))
		}
	}
	var all []*ir.Type
	for _, evs := range g.argEv {
		all = append(all, observedTypes(evs)...)
	}
	all = append(all, observedTypes(g.spreadEv)...)
	g.elem = joinAll(all)
	g.decided = true

	// What the call sites passed at a position is evidence about the
	// parameter that reads it, for every member that names one there.
	for _, m := range g.members {
		for i, b := range m.Params {
			if i >= len(g.argEv) || b == nil {
				continue
			}
			b.Evidence = mergeObservations(b.Evidence, g.argEv[i])
		}
		if m.VarArgs != nil && len(g.members) == 1 && g.elem != nil && !isUnresolved(g.elem) {
			b := m.VarArgs
			b.Evidence = mergeObservations(b.Evidence, []observation{{t: ir.SliceOf(g.elem), site: nil, round: l.round}})
		}
	}
}

// linkAlias records that two bindings name one structure: a parameter and
// the variable whose reference a call passes it. Perl's references make the
// callee's writes the caller's writes, so what either side learns about the
// structure is evidence about both.
func (l *Lowerer) linkAlias(a, b *Binding) {
	if a == nil || b == nil || a == b || l.pass != 1 {
		return
	}
	if l.aliasLinks == nil {
		l.aliasLinks = map[*Binding][]*Binding{}
	}
	for _, have := range l.aliasLinks[a] {
		if have == b {
			return
		}
	}
	l.aliasLinks[a] = append(l.aliasLinks[a], b)
	l.aliasLinks[b] = append(l.aliasLinks[b], a)
}

// shareAliasEvidence pools the evidence of the bindings that name one
// structure, so each settles on what all of them saw. It runs to a fixpoint
// over the links because a reference can travel through more than one call.
func (l *Lowerer) shareAliasEvidence() {
	for range 4 {
		moved := false
		for a, bs := range l.aliasLinks {
			for _, b := range bs {
				before := len(b.Evidence)
				b.Evidence = mergeObservations(b.Evidence, a.Evidence)
				if len(b.Evidence) != before {
					moved = true
				}
			}
		}
		if !moved {
			break
		}
	}
}

// mergeObservations folds src into dst without duplicating what an earlier
// round of the same feed already added.
func mergeObservations(dst, src []observation) []observation {
	have := map[observation]bool{}
	for _, o := range dst {
		have[o] = true
	}
	for _, o := range src {
		if have[o] {
			continue
		}
		dst = append(dst, o)
	}
	return dst
}

// unifyGroupResults gives every member of a shared slot the result shape
// they can all answer with: as many positions as the widest member returns,
// each position the join of what the members put there. It runs after each
// member's own returns have been settled, and only a group that really
// shares a slot, two or more members, is unified: a lone closure keeps
// whatever shape its body implies. A member returning fewer values than the
// widest one pads the rest with zero values, which is Perl's undef made
// explicit at the one place Go can say it, the return statement.
func (l *Lowerer) unifyGroupResults() {
	l.eachGroup(func(g *sigGroup) {
		if len(g.members) < 2 {
			return
		}
		width := 0
		for _, m := range g.members {
			fillResults(m)
			if len(m.Results) > width {
				width = len(m.Results)
			}
		}
		g.returns = width > 0
		if width == 0 {
			g.result = nil
			for _, m := range g.members {
				m.Results = nil
			}
			return
		}
		results := make([]*ir.Type, width)
		for _, m := range g.members {
			for i, t := range m.Results {
				results[i] = join(results[i], t)
			}
		}
		for i := range results {
			if results[i] == nil || isUnresolved(results[i]) {
				results[i] = ir.TAny
			}
		}
		g.result = results[0]
		g.unified = results
		for _, m := range g.members {
			m.Results = append([]*ir.Type(nil), results...)
		}
	})
}

// checkGroupAgreement runs once the parameters have settled and asks, for
// every fixed-signature group, whether the members' parameters really did
// land on one type per position. A position two members disagree about
// means the fixed list cannot be written, and the group falls back to the
// variadic slice for good.
func (l *Lowerer) checkGroupAgreement() {
	l.eachGroup(func(g *sigGroup) {
		if !g.fixed {
			return
		}
		g.settledParams = make([]*ir.Type, g.arity)
		for _, m := range g.members {
			for i, b := range m.Params {
				if b == nil || i >= g.arity {
					continue
				}
				switch {
				case g.settledParams[i] == nil:
					g.settledParams[i] = b.Type
				case !g.settledParams[i].Equal(b.Type):
					g.demoted = true
					g.fixed = false
				}
			}
		}
	})
}

// naturalShape reports how many leading scalar parameters the sub's body
// accounts for, and whether that accounting is complete. The `my (...) = @_`
// and `my $x = shift` idioms name parameters outright; a body whose only
// other reach into the argument list is `$_[k]` with the position written
// as a number is positional, which a fixed parameter list can still carry.
// Anything else, @_ whole, a slice of it, a computed index, leaves the
// arity to the program and rules the fixed list out.
func (s *Sub) naturalShape() (arity int, named bool) {
	if s.Decl == nil && s.Body == nil {
		return 0, false
	}
	body := s.Body
	if body == nil {
		body = s.Decl.Body
	}
	rest := valueTail(body)
	for len(rest) > 0 {
		es, ok := rest[0].(*ast.ExprStmt)
		if !ok {
			break
		}
		as, ok := es.X.(*ast.Assign)
		if !ok || as.Op != "=" {
			break
		}
		my, ok := as.LHS.(*ast.My)
		if !ok || my.Keyword != "my" {
			break
		}
		if isArgsVar(as.RHS) {
			vars := declaredVars(my)
			for _, v := range vars {
				if v.Sigil != '$' {
					return 0, false
				}
			}
			arity += len(vars)
			rest = rest[1:]
			continue
		}
		if isShiftArgs(as.RHS) {
			vars := declaredVars(my)
			if len(vars) == 1 && vars[0].Sigil == '$' {
				arity++
				rest = rest[1:]
				continue
			}
		}
		break
	}
	if !usesArgs(rest) {
		return arity, true
	}
	top, ok := positionalArgsOnly(rest)
	if !ok {
		return arity, false
	}
	if top+1 > arity {
		arity = top + 1
	}
	return arity, true
}

// positionalArgsOnly reports whether every reach into the argument list in
// these statements is `$_[k]` with k written as a number, and the largest k.
func positionalArgsOnly(body []ast.Stmt) (top int, ok bool) {
	top, ok = -1, true
	var walkE func(ast.Expr)
	var walkS func([]ast.Stmt)
	walkE = func(e ast.Expr) {
		switch n := e.(type) {
		case nil:
			return
		case *ast.Var:
			if n.Sigil == '@' && n.Name == "_" {
				ok = false
			}
		case *ast.Index:
			if v, isVar := n.Base.(*ast.Var); isVar && !n.Arrow && v.Sigil == '$' && v.Name == "_" {
				if k, num := staticIndex(n.Idx); num {
					if k > top {
						top = k
					}
				} else {
					ok = false
				}
				return
			}
			walkE(n.Base)
			walkE(n.Idx)
		case *ast.Slice:
			if v, isVar := n.Base.(*ast.Var); isVar && v.Sigil == '@' && v.Name == "_" {
				ok = false
				return
			}
			walkE(n.Base)
			for _, i := range n.Idx {
				walkE(i)
			}
		case *ast.RefGen:
			if isArgsVar(n.X) {
				ok = false
				return
			}
			walkE(n.X)
		case *ast.Deref:
			walkE(n.X)
		case *ast.HashIndex:
			walkE(n.Base)
			walkE(n.Key)
		case *ast.Match:
			walkE(n.Bound)
			walkE(n.PatternExpr)
		case *ast.Subst:
			walkE(n.Bound)
			walkE(n.Repl)
		case *ast.Trans:
			walkE(n.Bound)
		case *ast.Assign:
			walkE(n.LHS)
			walkE(n.RHS)
		case *ast.BinOp:
			walkE(n.L)
			walkE(n.R)
		case *ast.UnOp:
			walkE(n.X)
		case *ast.Ternary:
			walkE(n.Cond)
			walkE(n.A)
			walkE(n.B)
		case *ast.List:
			for _, el := range n.Elems {
				walkE(el)
			}
		case *ast.Call:
			if n.Name == "shift" && len(n.Args) == 0 {
				ok = false
			}
			for _, a := range n.Args {
				walkE(a)
			}
			walkS(n.Block)
		case *ast.FuncCallRef:
			walkE(n.Ref)
			for _, a := range n.Args {
				walkE(a)
			}
		case *ast.MethodCall:
			walkE(n.Invocant)
			walkE(n.Dynamic)
			for _, a := range n.Args {
				walkE(a)
			}
		case *ast.AnonArray:
			for _, el := range n.Elems {
				walkE(el)
			}
		case *ast.AnonHash:
			for _, el := range n.Elems {
				walkE(el)
			}
		case *ast.My:
			for _, v := range n.Vars {
				walkE(v)
			}
		case *ast.InterpLit:
			for _, p := range n.Parts {
				walkE(p)
			}
		case *ast.AnonSub:
			// A nested sub's @_ is its own argument list, not this one.
			return
		}
	}
	walkS = func(list []ast.Stmt) {
		for _, st := range list {
			switch n := st.(type) {
			case *ast.ExprStmt:
				walkE(n.X)
			case *ast.If:
				walkE(n.Cond)
				walkS(n.Then)
				for _, ei := range n.ElseIfs {
					walkE(ei.Cond)
					walkS(ei.Then)
				}
				walkS(n.Else)
			case *ast.While:
				walkE(n.Cond)
				walkS(n.Body)
			case *ast.ForC:
				walkE(n.Init)
				walkE(n.Cond)
				walkE(n.Post)
				walkS(n.Body)
			case *ast.Foreach:
				for _, e := range n.List {
					walkE(e)
				}
				walkS(n.Body)
			case *ast.Block:
				walkS(n.Body)
			case *ast.Return:
				for _, e := range n.Exprs {
					walkE(e)
				}
			}
		}
	}
	walkS(body)
	return top, ok
}

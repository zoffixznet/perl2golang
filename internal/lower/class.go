package lower

import (
	"strings"

	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
)

// This file turns Perl's package-plus-bless object system into Go types.
//
// A Perl class is a package whose constructor blesses a hash reference. The
// hash keys are the object's fields, the package's subs are its methods, and
// @ISA names its parent. None of that is declared anywhere: it is discovered by
// reading the code, which is exactly what this pass does before any lowering
// happens.
//
// The result is an ordinary Go type per class: a struct with one field per hash
// key, methods on a pointer receiver, and a parent embedded by value. What that
// mapping cannot express is Perl's late binding, and where a program depends on
// it the call site says so rather than quietly resolving the wrong way.

// Class is one Perl package that the program uses as a class.
type Class struct {
	// Perl is the package name, for example "Inventory::Item".
	Perl string
	// Go is the generated type name, for example "InventoryItem".
	Go string
	// ParentName is the single parent named by @ISA, `use parent` or
	// `use base`, empty when the class has none.
	ParentName string
	Parent     *Class
	// Children are the classes that name this one as their parent.
	Children []*Class
	// Fields are the struct fields, in the order they were discovered.
	Fields  []*ClassField
	fieldBy map[string]*ClassField
	// Subs are the package's subroutines, in declaration order.
	Subs  []*Sub
	subBy map[string]*Sub
	// Ctor is `sub new`, or whichever sub blesses.
	Ctor *Sub
	// Blessed records that something in the package blesses a reference,
	// which is what distinguishes a class from a plain namespace.
	Blessed bool
	// Overloads names the operators `use overload` gave the class, which Go
	// has no way to hook at all.
	Overloads map[string]bool
	// ArrayBacked records that the class blesses an array reference rather
	// than a hash. Its fields are positions rather than names, so there is
	// nothing for a struct's field list to be built from.
	ArrayBacked bool
	// IsType records the final decision: this package becomes a Go type.
	IsType bool
	Line   int
	// Ptr and Value are the two spellings of the generated type.
	Ptr   *ir.Type
	Value *ir.Type
	// Doc is the comment block above the package statement.
	Doc []string
	// recv is the receiver identifier its methods use.
	recv string

	decls []*ast.SubDecl
}

// ClassField is one struct field discovered from a hash key.
type ClassField struct {
	// Perl is the hash key, for example "retry_after".
	Perl string
	// Go is the field name, for example "RetryAfter".
	Go string
	// Type is the field's settled Go type.
	Type     *ir.Type
	Evidence []*ir.Type
	// Owner is the class that declares it, which matters once a parent is
	// embedded and its fields are reachable through promotion.
	Owner *Class
	Line  int
}

// SubKind says how a package's subroutine is turned into Go.
type SubKind int

const (
	// SubPlain is an ordinary function, which is what every sub in package
	// main is and what a sub in a namespace-only package becomes.
	SubPlain SubKind = iota
	// SubMethod is a method with a pointer receiver.
	SubMethod
	// SubCtor is a constructor returning a pointer to the class.
	SubCtor
	// SubClass is a class method: it takes the class name rather than an
	// object, so it becomes a plain function.
	SubClass
	// SubSpecial is a method perl calls on the program's behalf, DESTROY or
	// AUTOLOAD, neither of which Go has anywhere to put.
	SubSpecial
)

// classFor returns the class record for a package name, creating it on first
// sight.
func (l *Lowerer) classFor(name string, at ast.Node) *Class {
	if c, ok := l.classes[name]; ok {
		return c
	}
	c := &Class{
		Perl:    name,
		fieldBy: map[string]*ClassField{},
		subBy:   map[string]*Sub{},
		Line:    posLine(at),
	}
	l.classes[name] = c
	l.classOrd = append(l.classOrd, name)
	return c
}

// pkgStmt is one statement together with the package it was written in.
type pkgStmt struct {
	pkg string
	st  ast.Stmt
}

// splitPackages flattens a statement list into statements tagged with the
// package in force where each one was written.
//
// Perl's `package Foo;` changes the current package for everything after it in
// the enclosing block, and the braced form scopes it to its own body. Both are
// ordinary in a script, and both mean the file holds more than one class.
func splitPackages(list []ast.Stmt, pkg string) []pkgStmt {
	var out []pkgStmt
	cur := pkg
	for _, st := range list {
		switch n := st.(type) {
		case *ast.PackageDecl:
			if n.Body != nil {
				out = append(out, splitPackages(n.Body, n.Name)...)
				continue
			}
			cur = n.Name
			continue
		case *ast.Block:
			// A bare block holding a package statement is how a script
			// declares a small class inline. Its contents belong to that
			// package and are file-scope for it.
			if containsPackage(n.Body) {
				out = append(out, splitPackages(n.Body, cur)...)
				continue
			}
		}
		out = append(out, pkgStmt{pkg: cur, st: st})
	}
	return out
}

func containsPackage(list []ast.Stmt) bool {
	for _, st := range list {
		if _, ok := st.(*ast.PackageDecl); ok {
			return true
		}
	}
	return false
}

// collectClasses reads the whole file once and works out which packages are
// classes, what their parents are, and which subs belong to each.
func (l *Lowerer) collectClasses(prog *ast.Program) {
	l.units = splitPackages(prog.Stmts, "main")
	// How the file calls a sub is the evidence for what it is. A name
	// reached through an arrow is a method; a name written with its package
	// in front is a function.
	walkExprs(prog.Stmts, func(e ast.Expr) {
		switch n := e.(type) {
		case *ast.MethodCall:
			if n.Method != "" {
				l.arrowCalls[n.Method] = true
			}
		case *ast.Call:
			if strings.Contains(n.Name, "::") {
				l.qualCalls[n.Name] = true
			}
		}
	})
	for _, ps := range l.units {
		if ps.pkg == "main" {
			continue
		}
		c := l.classFor(ps.pkg, ps.st)
		switch n := ps.st.(type) {
		case *ast.SubDecl:
			c.decls = append(c.decls, n)
			if blessesSomething(n.Body) {
				c.Blessed = true
			}
			if blessesArray(n.Body) {
				c.ArrayBacked = true
			}
		case *ast.Use:
			if p, ok := parentFromUse(n); ok {
				c.ParentName = p
			}
		case *ast.ExprStmt:
			if p, ok := parentFromISA(n.X); ok {
				c.ParentName = p
			}
		}
	}
	// A bless naming a package makes that package a class even when the
	// package itself declares nothing, and `Class->new` does the same.
	for _, name := range blessTargets(prog.Stmts) {
		if name != "main" {
			l.classFor(name, nil).Blessed = true
		}
	}

	// Link the hierarchy before deciding which packages are types, because a
	// subclass with nothing in it but `our @ISA` is still a class.
	for _, name := range l.classOrd {
		c := l.classes[name]
		if c.ParentName == "" {
			continue
		}
		p, ok := l.classes[c.ParentName]
		if !ok {
			continue
		}
		c.Parent = p
		p.Children = append(p.Children, c)
	}
	for _, name := range l.classOrd {
		c := l.classes[name]
		c.IsType = l.isClass(c, map[*Class]bool{})
	}
	// Naming happens in declaration order so that two runs over one file
	// agree about which class got which name.
	for _, name := range l.classOrd {
		c := l.classes[name]
		if !c.IsType {
			continue
		}
		c.Go = l.names.take(classGoName(name))
		c.Value = ir.NamedType(c.Go, "")
		c.Ptr = ir.PointerTo(c.Value)
		l.byGoType[c.Go] = c
	}
}

// markShared finds the file-scope `my` variables that a sub also reads.
//
// Perl's file-scope `my` is a lexical the whole file closes over, which is how
// a class keeps state shared by every instance. Go has no file-scope lexical
// short of a package-level variable, so those declarations become one and the
// declaration line becomes an assignment to it.
func (l *Lowerer) markShared() {
	declared := map[string]*ast.Var{}
	for _, u := range l.units {
		es, ok := u.st.(*ast.ExprStmt)
		if !ok {
			continue
		}
		var my *ast.My
		switch n := es.X.(type) {
		case *ast.My:
			my = n
		case *ast.Assign:
			if m, is := n.LHS.(*ast.My); is {
				my = m
			}
		}
		if my == nil || my.Keyword != "my" {
			continue
		}
		for _, v := range declaredVars(my) {
			key := varKey(v.Sigil, v.Name)
			if _, dup := declared[key]; !dup {
				declared[key] = v
			}
		}
	}
	if len(declared) == 0 {
		return
	}
	for _, u := range l.units {
		sd, ok := u.st.(*ast.SubDecl)
		if !ok {
			continue
		}
		mark := func(sigil rune, name string) {
			if d, found := declared[varKey(sigil, name)]; found {
				l.hoisted[d] = true
			}
		}
		walkExprs(sd.Body, func(e ast.Expr) {
			switch n := e.(type) {
			case *ast.Var:
				sigil := n.Sigil
				if sigil == '#' {
					sigil = '@'
				}
				mark(sigil, n.Name)
			case *ast.HashIndex:
				// $h{k} names %h, not $h: the sigil describes the element.
				if v, ok := n.Base.(*ast.Var); ok && !n.Arrow && v.Sigil == '$' {
					mark('%', v.Name)
				}
			case *ast.Index:
				if v, ok := n.Base.(*ast.Var); ok && !n.Arrow && v.Sigil == '$' {
					mark('@', v.Name)
				}
			case *ast.Slice:
				if v, ok := n.Base.(*ast.Var); ok {
					if n.Hash {
						mark('%', v.Name)
					} else {
						mark('@', v.Name)
					}
				}
			}
		})
	}
}

// isClass decides whether a package becomes a Go type. Blessing is the direct
// evidence; inheriting from a class is the indirect kind, and a package whose
// subs take a $self is one in all but name.
func (l *Lowerer) isClass(c *Class, seen map[*Class]bool) bool {
	if c == nil || seen[c] {
		return false
	}
	seen[c] = true
	if c.ArrayBacked {
		// The object is a list of positions, not a set of named keys, so
		// there is nothing to name the struct's fields after.
		return false
	}
	if c.Blessed {
		return true
	}
	if c.Parent != nil && l.isClass(c.Parent, seen) {
		return true
	}
	for _, ch := range c.Children {
		if ch.Blessed {
			return true
		}
	}
	for _, sd := range c.decls {
		if _, ok := selfParam(sd.Body); ok {
			return true
		}
	}
	return false
}

// classGoName turns a Perl package name into a Go type name: the colons go and
// each part is capitalised, so Inventory::Item becomes InventoryItem.
func classGoName(perl string) string {
	parts := strings.Split(perl, "::")
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(upperFirst(goName(p)))
	}
	out := b.String()
	if out == "" {
		return "Class"
	}
	return out
}

// ancestors lists a class and everything it inherits from, nearest first.
func (c *Class) ancestors() []*Class {
	var out []*Class
	seen := map[*Class]bool{}
	for x := c; x != nil && !seen[x]; x = x.Parent {
		seen[x] = true
		out = append(out, x)
	}
	return out
}

// isa reports whether the class is the named one or descends from it.
func (c *Class) isa(name string) bool {
	for _, a := range c.ancestors() {
		if a.Perl == name {
			return true
		}
	}
	return false
}

// method finds a method by Perl name anywhere in the inheritance chain.
func (c *Class) method(name string) *Sub {
	for _, a := range c.ancestors() {
		if s, ok := a.subBy[name]; ok {
			return s
		}
	}
	return nil
}

// field finds a field by hash key anywhere in the inheritance chain, which is
// what Go's embedding gives for free through promotion.
func (c *Class) field(name string) *ClassField {
	for _, a := range c.ancestors() {
		if f, ok := a.fieldBy[name]; ok {
			return f
		}
	}
	return nil
}

// declareField records a hash key as a struct field of the class, or of
// whichever ancestor already owns it.
func (l *Lowerer) declareField(c *Class, key string, at ast.Node) *ClassField {
	if c == nil || key == "" {
		return nil
	}
	if f := c.field(key); f != nil {
		return f
	}
	f := &ClassField{
		Perl:  key,
		Go:    l.fieldName(c, key),
		Owner: c,
		Line:  posLine(at),
		Type:  ir.TAny,
	}
	c.fieldBy[key] = f
	c.Fields = append(c.Fields, f)
	return f
}

// fieldName picks the Go name for a field, keeping it clear of the method
// names on the same type. Go has one namespace per type: a field called Name
// and a method called Name cannot both exist, and Perl happily has both.
func (l *Lowerer) fieldName(c *Class, key string) string {
	base := exportedName(key)
	if base == "" {
		base = "Field"
	}
	taken := func(name string) bool {
		for _, a := range c.ancestors() {
			for _, f := range a.Fields {
				if f.Go == name {
					return true
				}
			}
			for _, s := range a.Subs {
				if s.Kind == SubMethod && s.Go == name && s.Accessor == nil {
					return true
				}
			}
			if a.Go == name {
				return true
			}
		}
		return false
	}
	if !taken(base) {
		return base
	}
	for i := 2; ; i++ {
		cand := base + itoa(i)
		if !taken(cand) {
			return cand
		}
	}
}

// observeField records a type the field was seen holding.
func (l *Lowerer) observeField(f *ClassField, t *ir.Type) {
	if f == nil || t == nil || l.pass != 1 {
		return
	}
	if t.Kind == ir.Void || t.Kind == ir.Invalid {
		return
	}
	f.Evidence = append(f.Evidence, t)
}

// settleFields decides each field's Go type from what the file showed it
// holding, exactly as it does for a variable.
func (l *Lowerer) settleFields() {
	for _, name := range l.classOrd {
		c := l.classes[name]
		for _, f := range c.Fields {
			t := joinAll(f.Evidence)
			if t == nil {
				t = ir.TAny
			}
			f.Type = t
		}
	}
}

// classDecl builds the Go type declaration for a class.
func (l *Lowerer) classDecl(c *Class) ir.Decl {
	d := &ir.TypeDecl{Name: c.Go}
	if c.Parent != nil && c.Parent.IsType {
		d.Fields = append(d.Fields, ir.Field{
			Type: c.Parent.Value,
			Doc: c.Parent.Go + " is embedded, so its fields and its methods are " +
				"reachable directly on a " + c.Go + ".",
		})
	}
	for _, f := range c.Fields {
		d.Fields = append(d.Fields, ir.Field{Name: f.Go, Type: f.Type})
	}
	d.Doc = []string{c.Go + " is one " + lastPart(c.Perl) + " and everything it knows about itself."}
	if len(c.Doc) > 0 {
		d.Doc = append([]string{c.Go + " is defined as follows."}, c.Doc...)
	}
	if l.pass == 2 {
		ir.Annotate(d, "Perl builds an object by blessing a hash reference, so the "+
			"field names live in the data and any typo makes a new key. Go writes them "+
			"down once, here, and the compiler checks every use of them.",
			"structs-and-embedding", "static-types-and-zero-values")
		if c.Parent != nil && c.Parent.IsType {
			ir.Annotate(d, "Perl's @ISA makes method lookup walk to the parent class. Go "+
				"has no inheritance; embedding a struct without a field name promotes its "+
				"fields and methods onto this one, which covers the same ground for "+
				"everything except overriding.",
				"structs-and-embedding", "late-binding-vs-embedding")
		}
	}
	return d
}

// ---------------------------------------------------------------------------
// Reading the class out of the source

// blessesSomething reports whether a statement list calls bless.
func blessesSomething(body []ast.Stmt) bool {
	found := false
	walkExprs(body, func(e ast.Expr) {
		if c, ok := e.(*ast.Call); ok && c.Name == "bless" {
			found = true
		}
	})
	return found
}

// blessesArray reports whether a statement list blesses an array reference,
// which makes the object a list of positions rather than a set of named keys.
func blessesArray(body []ast.Stmt) bool {
	found := false
	walkExprs(body, func(e ast.Expr) {
		c, ok := e.(*ast.Call)
		if !ok || c.Name != "bless" || len(c.Args) == 0 {
			return
		}
		switch arg := c.Args[0].(type) {
		case *ast.AnonArray:
			found = true
		case *ast.Var:
			if arg.Sigil == '@' {
				found = true
			}
		case *ast.RefGen:
			if v, ok := arg.X.(*ast.Var); ok && v.Sigil == '@' {
				found = true
			}
		}
	})
	return found
}

// blessTargets lists every class name a literal `bless ..., 'Name'` names.
func blessTargets(body []ast.Stmt) []string {
	var out []string
	walkExprs(body, func(e ast.Expr) {
		c, ok := e.(*ast.Call)
		if !ok || c.Name != "bless" || len(c.Args) < 2 {
			return
		}
		if s, ok := staticString(c.Args[1]); ok {
			out = append(out, s)
		}
	})
	return out
}

// parentFromUse reads the parent class out of `use parent` or `use base`.
func parentFromUse(n *ast.Use) (string, bool) {
	if n.Module != "parent" && n.Module != "base" {
		return "", false
	}
	for _, a := range n.Args {
		for _, one := range flatten(a) {
			names := staticStrings(one)
			for _, name := range names {
				if name == "-norequire" || name == "" {
					continue
				}
				return name, true
			}
		}
	}
	return "", false
}

// parentFromISA reads the parent class out of `our @ISA = ('Base')` or
// `push @ISA, 'Base'`.
func parentFromISA(e ast.Expr) (string, bool) {
	a, ok := e.(*ast.Assign)
	if !ok || a.Op != "=" {
		return "", false
	}
	target := a.LHS
	if my, isMy := target.(*ast.My); isMy && len(my.Vars) == 1 {
		target = my.Vars[0]
	}
	v, ok := target.(*ast.Var)
	if !ok || v.Sigil != '@' || !strings.HasSuffix(v.Name, "ISA") {
		return "", false
	}
	for _, one := range flatten(a.RHS) {
		for _, name := range staticStrings(one) {
			if name != "" {
				return name, true
			}
		}
	}
	return "", false
}

// staticStrings reads the constant strings out of an expression, covering both
// a plain literal and a qw() list.
func staticStrings(e ast.Expr) []string {
	switch n := e.(type) {
	case *ast.StrLit:
		return []string{n.Value}
	case *ast.QwLit:
		return n.Words
	case *ast.List:
		var out []string
		for _, el := range n.Elems {
			out = append(out, staticStrings(el)...)
		}
		return out
	case *ast.UnOp:
		if n.Op == "-" {
			if inner, ok := n.X.(*ast.Call); ok && len(inner.Args) == 0 {
				return []string{"-" + inner.Name}
			}
		}
	case *ast.Call:
		if len(n.Args) == 0 && !n.Paren {
			return []string{n.Name}
		}
	}
	return nil
}

// selfParam reports the name of the first parameter a sub unpacks, which is
// what says whether it is an instance method, a class method, or neither.
func selfParam(body []ast.Stmt) (string, bool) {
	for _, st := range body {
		es, ok := st.(*ast.ExprStmt)
		if !ok {
			return "", false
		}
		as, ok := es.X.(*ast.Assign)
		if !ok || as.Op != "=" {
			return "", false
		}
		my, ok := as.LHS.(*ast.My)
		if !ok {
			return "", false
		}
		if !isArgsVar(as.RHS) && !isShiftArgs(as.RHS) {
			return "", false
		}
		vars := declaredVars(my)
		if len(vars) == 0 {
			return "", false
		}
		name := vars[0].Name
		switch name {
		case "self", "this", "obj":
			return name, true
		case "class", "proto", "pkg":
			return name, false
		}
		// The name is not one of the conventional ones, so what the body does
		// with it decides: a sub that reaches through its first argument with
		// an arrow is a method however it spelled the variable.
		return name, usesAsObject(body, name)
	}
	return "", false
}

// usesAsObject reports whether a sub body reaches through a named scalar with
// an arrow, which is what an object is for.
func usesAsObject(body []ast.Stmt, name string) bool {
	if name == "" {
		return false
	}
	is := func(e ast.Expr) bool {
		v, ok := e.(*ast.Var)
		return ok && v.Sigil == '$' && v.Name == name
	}
	found := false
	walkExprs(body, func(e ast.Expr) {
		switch n := e.(type) {
		case *ast.HashIndex:
			if n.Arrow && is(n.Base) {
				found = true
			}
		case *ast.Index:
			if n.Arrow && is(n.Base) {
				found = true
			}
		case *ast.MethodCall:
			if is(n.Invocant) {
				found = true
			}
		}
	})
	return found
}

// walkExprs visits every expression in a statement list, including the bodies
// of nested subs and blocks.
func walkExprs(body []ast.Stmt, fn func(ast.Expr)) {
	var expr func(ast.Expr)
	var stmts func([]ast.Stmt)
	expr = func(e ast.Expr) {
		if e == nil {
			return
		}
		fn(e)
		switch n := e.(type) {
		case *ast.Assign:
			expr(n.LHS)
			expr(n.RHS)
		case *ast.BinOp:
			expr(n.L)
			expr(n.R)
		case *ast.UnOp:
			expr(n.X)
		case *ast.Ternary:
			expr(n.Cond)
			expr(n.A)
			expr(n.B)
		case *ast.List:
			for _, el := range n.Elems {
				expr(el)
			}
		case *ast.My:
			for _, v := range n.Vars {
				expr(v)
			}
		case *ast.Call:
			for _, a := range n.Args {
				expr(a)
			}
			stmts(n.Block)
		case *ast.MethodCall:
			expr(n.Invocant)
			expr(n.Dynamic)
			for _, a := range n.Args {
				expr(a)
			}
		case *ast.FuncCallRef:
			expr(n.Ref)
			for _, a := range n.Args {
				expr(a)
			}
		case *ast.Index:
			expr(n.Base)
			expr(n.Idx)
		case *ast.HashIndex:
			expr(n.Base)
			expr(n.Key)
		case *ast.Slice:
			expr(n.Base)
			for _, i := range n.Idx {
				expr(i)
			}
		case *ast.Deref:
			expr(n.X)
		case *ast.RefGen:
			expr(n.X)
		case *ast.AnonArray:
			for _, el := range n.Elems {
				expr(el)
			}
		case *ast.AnonHash:
			for _, el := range n.Elems {
				expr(el)
			}
		case *ast.AnonSub:
			stmts(n.Body)
		case *ast.InterpLit:
			for _, p := range n.Parts {
				expr(p)
			}
		case *ast.Match:
			expr(n.Bound)
			expr(n.PatternExpr)
		case *ast.Subst:
			expr(n.Bound)
			expr(n.Repl)
		case *ast.Trans:
			expr(n.Bound)
		}
	}
	stmts = func(list []ast.Stmt) {
		for _, st := range list {
			switch n := st.(type) {
			case *ast.ExprStmt:
				expr(n.X)
			case *ast.If:
				expr(n.Cond)
				stmts(n.Then)
				for _, ei := range n.ElseIfs {
					expr(ei.Cond)
					stmts(ei.Then)
				}
				stmts(n.Else)
			case *ast.While:
				expr(n.Cond)
				stmts(n.Body)
			case *ast.ForC:
				expr(n.Init)
				expr(n.Cond)
				expr(n.Post)
				stmts(n.Body)
			case *ast.Foreach:
				expr(n.Var)
				for _, e := range n.List {
					expr(e)
				}
				stmts(n.Body)
			case *ast.Block:
				stmts(n.Body)
			case *ast.SubDecl:
				stmts(n.Body)
			case *ast.PackageDecl:
				stmts(n.Body)
			case *ast.Return:
				for _, e := range n.Exprs {
					expr(e)
				}
			}
		}
	}
	stmts(body)
}

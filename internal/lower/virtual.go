package lower

import (
	"sort"
	"strings"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file closes the one gap in the struct-and-embedding mapping.
//
// Perl looks a method up on the object's real class every time it is called,
// so a base method calling another method on its own object reaches the
// subclass's version. Go resolves that call against the type the receiver is
// declared as, which inside a base method is always the base, so the override
// is never reached and the program quietly does the wrong thing.
//
// The fix is the one a Go developer writes by hand for a template method:
// declare an interface with the methods the base calls on itself, give the
// base struct a field of that interface type, have every constructor store the
// finished object in it, and call through the field. That is composition plus
// an interface, and it is what the generated code now does.

// virtualRoot returns the class at the top of c's hierarchy.
//
// The back-pointer lives there and nowhere else: one field, promoted through
// the embedding to every class below it, holding one interface that the whole
// hierarchy satisfies.
func virtualRoot(c *Class) *Class {
	for c != nil && c.Parent != nil && c.Parent.IsType {
		c = c.Parent
	}
	return c
}

// markVirtual records that a method has to be reached through the object's own
// class rather than through the type the receiver is declared as, and reports
// whether the class can express that.
//
// Only a method declared at the top of the hierarchy qualifies. A method
// declared halfway down is not on the interface the whole hierarchy satisfies,
// so there is nothing to call it through, and the diagnostic stands instead.
func (l *Lowerer) markVirtual(s *Sub) bool {
	c := s.Class
	if c == nil || !c.IsType || virtualRoot(c) != c || s.Recv == nil {
		return false
	}
	if c.Virtual == nil {
		c.Virtual = map[string]*Sub{}
		base := strings.ToLower(c.Go[:1]) + c.Go[1:]
		c.SelfIface = l.names.take(base + "Self")
		c.SelfField = "whole"
		c.SelfMethod = "self"
	}
	c.Virtual[s.Name] = s
	return true
}

// selfType is the interface a hierarchy's back-pointer holds.
func (c *Class) selfType() *ir.Type {
	if c == nil || c.SelfIface == "" {
		return nil
	}
	return ir.NamedType(c.SelfIface, "")
}

// virtualNames lists a class's virtual methods in a fixed order.
func (c *Class) virtualNames() []string {
	out := make([]string, 0, len(c.Virtual))
	for name := range c.Virtual {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// selfCall rewrites a call a base method makes on its own object so that it
// goes through the back-pointer, and reports whether it did.
func (l *Lowerer) selfCall(s *Sub, recv ir.Expr) (ir.Expr, bool) {
	root := virtualRoot(s.Class)
	if root == nil || root.Virtual == nil || root.Virtual[s.Name] == nil {
		return nil, false
	}
	out := ir.CallOf(selector(recv, root.SelfMethod, nil), root.selfType())
	l.note(out, "Perl looks a method up on the object's real class every time, so a "+
		"base method calling another method on itself reaches a subclass's override. "+
		"Go resolves the call against the type the receiver is declared as, which "+
		"here is the base, so the call goes through the interface the object stored "+
		"in itself when it was built. That is how Go writes a template method.",
		"late-binding-vs-embedding", "implicit-interfaces", "structs-and-embedding")
	return out, true
}

// receiverReturn lowers `return $self` in a method of a hierarchy that has
// subclasses, and reports whether it applied.
//
// Returning the receiver as its declared type would lose the object's real
// class: inside a promoted method the receiver is the embedded base struct,
// so a Timeout error rethrown through the base's with_context came back as a
// plain base error and classified as nothing. The object's back-pointer is
// the one thing that still knows the whole object, so that is what a method
// that hands back $self hands back.
func (l *Lowerer) receiverReturn(e ast.Expr) (ir.Expr, bool) {
	s := l.curSub
	if s == nil || s.Recv == nil || s.Class == nil {
		return nil, false
	}
	v, ok := e.(*ast.Var)
	if !ok || v.Sigil != '$' {
		return nil, false
	}
	b, found := l.scope.lookup(varKey('$', v.Name))
	if !found || b != s.Recv {
		return nil, false
	}
	root := virtualRoot(s.Class)
	if root == nil || len(root.Children) == 0 {
		return nil, false
	}
	if !l.markVirtual(s) {
		return nil, false
	}
	out := ir.CallOf(selector(l.identFor(s.Recv), root.SelfMethod, nil), root.selfType())
	l.note(out, "This method hands back the object it was called on, and the object's "+
		"real class matters to the caller: a subclass going in must come back out as "+
		"itself, not as the "+root.Go+" inside it. The receiver here is only the "+
		"embedded part, so the method returns the whole object through the reference "+
		"it stored in itself when it was built.",
		"late-binding-vs-embedding", "structs-and-embedding", "implicit-interfaces")
	return out, true
}

// dynamicClassName lowers `ref $self` in a method of a hierarchy that has
// subclasses, where the receiver's declared type is not the whole answer: a
// subclass calling a promoted method reaches this code with the base struct
// as the receiver, and Perl would have answered with the subclass's name.
func (l *Lowerer) dynamicClassName(node ast.Expr, c *Class) (ir.Expr, bool) {
	s := l.curSub
	if s == nil || s.Recv == nil || s.Class == nil {
		return nil, false
	}
	v, ok := node.(*ast.Var)
	if !ok || v.Sigil != '$' {
		return nil, false
	}
	b, found := l.scope.lookup(varKey('$', v.Name))
	if !found || b != s.Recv {
		return nil, false
	}
	root := virtualRoot(s.Class)
	if root == nil || len(root.Children) == 0 {
		return nil, false
	}
	if !l.markVirtual(s) {
		return nil, false
	}
	fn, ok := l.classNamePredicate()
	if !ok {
		return nil, false
	}
	self := ir.CallOf(selector(l.identFor(s.Recv), root.SelfMethod, nil), root.selfType())
	out := ir.CallOf(ir.NewIdent(fn, nil), ir.TString, self)
	l.note(out, "ref answers with the class of the object itself, which inside this "+
		"method may be any class built on "+root.Go+": the receiver is only the "+
		"embedded part, and writing its name in would stamp every subclass with the "+
		"base's name. The object's back-pointer still knows the whole object, and the "+
		"generated table maps its type to the name Perl knew it by.",
		"late-binding-vs-embedding", "type-assertions-and-switches")
	return out, true
}

// isSelfIface reports whether a type is the back-pointer interface of some
// hierarchy, which carries an object every bit as much as the object's own
// pointer type does.
func (l *Lowerer) isSelfIface(t *ir.Type) bool {
	if t == nil || t.Kind != ir.Named {
		return false
	}
	for _, name := range l.classOrd {
		if c := l.classes[name]; c != nil && c.SelfIface != "" && c.SelfIface == t.Name {
			return true
		}
	}
	return false
}

// virtualDecls builds the interface and the accessor a hierarchy needs, when
// something in it asked for one.
func (l *Lowerer) virtualDecls(c *Class) []ir.Decl {
	if c.Virtual == nil {
		return nil
	}
	var b strings.Builder
	b.WriteString("type " + c.SelfIface + " interface {\n")
	for _, name := range c.virtualNames() {
		b.WriteString("\t" + methodSignature(c.Virtual[name]) + "\n")
	}
	b.WriteString("}")
	iface := &ir.RawDecl{
		Source: b.String(),
		Doc: []string{c.SelfIface + " is the set of methods " + c.Go + " calls on its " +
			"own object, which a class built on " + c.Go + " may replace."},
	}

	recv := l.receiverName(c)
	acc := &ir.RawDecl{
		Source: "func (" + recv + " *" + c.Go + ") " + c.SelfMethod + "() " + c.SelfIface + " {\n" +
			"\tif " + recv + "." + c.SelfField + " != nil {\n" +
			"\t\treturn " + recv + "." + c.SelfField + "\n" +
			"\t}\n" +
			"\treturn " + recv + "\n" +
			"}",
		Doc: []string{c.SelfMethod + " returns the whole object this " + c.Go + " is part of, " +
			"so that a method here reaches a replacement rather than its own version."},
	}
	if l.pass == 2 {
		ir.Annotate(iface, "Nothing declares that it implements this: a type satisfies an "+
			"interface by having the methods. Every class in the hierarchy has these, "+
			"because they are declared on "+c.Go+" and promoted through the embedding, "+
			"and a class that replaces one still satisfies it.",
			"implicit-interfaces", "late-binding-vs-embedding")
		ir.Annotate(acc, "The fallback matters: a value built as a plain literal rather "+
			"than through a constructor has nothing in the field, and returning the "+
			"receiver then gives exactly the behaviour embedding would have given, "+
			"which is the right answer when there is no subclass in play.",
			"late-binding-vs-embedding", "nil-vs-undef")
	}
	return []ir.Decl{iface, acc}
}

// storeSelf writes the statement that puts a finished object in its own
// back-pointer, for a constructor that built one.
func (l *Lowerer) storeSelf(c *Class, target ir.Expr) ir.Stmt {
	root := virtualRoot(c)
	if root == nil || root.Virtual == nil {
		return nil
	}
	st := assign("=", []ir.Expr{selector(target, root.SelfField, root.selfType())}, []ir.Expr{target})
	l.note(st, "The object keeps a reference to itself, as the interface its base "+
		"class calls through. Perl needed nothing here because it looks every method "+
		"up on the real class; Go decides at compile time, so the one thing that "+
		"still knows the real class is the object itself.",
		"late-binding-vs-embedding", "implicit-interfaces")
	return st
}

// methodSignature renders one method's line in an interface declaration.
func methodSignature(s *Sub) string {
	var b strings.Builder
	b.WriteString(s.Go + "(")
	first := true
	write := func(t *ir.Type, variadic bool) {
		if !first {
			b.WriteString(", ")
		}
		first = false
		if variadic {
			b.WriteString("...")
		}
		b.WriteString(t.String())
	}
	for _, p := range s.Params {
		write(p.Type, false)
	}
	for _, p := range s.NamedParams {
		write(p.Type, false)
	}
	if s.VarArgs != nil {
		write(elemOf(s.VarArgs.Type), true)
	}
	b.WriteString(")")
	switch len(s.Results) {
	case 0:
	case 1:
		b.WriteString(" " + s.Results[0].String())
	default:
		b.WriteString(" (")
		for i, r := range s.Results {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(r.String())
		}
		b.WriteString(")")
	}
	return b.String()
}

// unifyOverrides gives a method and every override of it one signature.
//
// Perl has no signature at all, so a base method that only ever dies and a
// subclass's real implementation can disagree about what they hand back
// without anyone noticing. Go needs the two to agree: whatever an interface
// promises, a type that answers to the name with a different result does not
// satisfy it, and every interface a hierarchy needs is built from these
// methods.
func (l *Lowerer) unifyOverrides() {
	for _, name := range l.classOrd {
		c := l.classes[name]
		if c == nil || !c.IsType || c.Parent == nil || !c.Parent.IsType {
			continue
		}
		for _, perl := range methodKeys(c.subBy) {
			s := c.subBy[perl]
			// A constructor is not an override: each class's builds its own
			// type and hands back a pointer to it, and those really do
			// differ.
			if s.Kind != SubMethod {
				continue
			}
			for _, a := range c.ancestors()[1:] {
				if base, ok := a.subBy[perl]; ok && base != s && base.Kind == SubMethod {
					unifyResults(base, s)
				}
			}
		}
	}
}

// unifyResults settles two versions of one method on a single signature.
func unifyResults(a, b *Sub) {
	// The parameters have to agree too: a base that was only ever called with
	// a list and an override only ever called with one value would otherwise
	// be two different methods wearing one name, and no interface covers
	// both.
	if len(a.Params) == len(b.Params) && a.VarArgs == nil && b.VarArgs == nil {
		for i := range a.Params {
			t := join(a.Params[i].Type, b.Params[i].Type)
			if t == nil || isUnresolved(t) {
				t = ir.TAny
			}
			a.Params[i].Type, b.Params[i].Type = t, t
		}
	}
	switch {
	case len(a.Results) == 0 && len(b.Results) > 0:
		a.Results = b.Results
	case len(b.Results) == 0 && len(a.Results) > 0:
		b.Results = a.Results
	case len(a.Results) != len(b.Results):
	default:
		for i := range a.Results {
			t := join(a.Results[i], b.Results[i])
			if t == nil || isUnresolved(t) {
				t = ir.TAny
			}
			a.Results[i], b.Results[i] = t, t
		}
	}
}

package lower

import (
	"sort"
	"strings"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file handles the values whose class only the running program knows.
//
// Perl looks a method up on whatever the reference was blessed into, at the
// moment the call runs, so a variable can hold a Circle on one pass and a
// Square on the next and `$shape->area` works either way. The commonest place
// this happens is $@, which holds whatever the last die threw. Go decides
// method calls when it compiles, so the value has to be given a type first,
// and the type that several classes can share is an interface.
//
// The translation is therefore: name the interface, assert the value to it,
// and call the method on the result. That is what a Go developer writes for
// the same problem, and it is checked at the point of the assertion rather
// than never.

// answering returns the type to assert a value to before calling a method on
// it, which is the interface the classes answering to that name share, or the
// one class that declares it.
func (l *Lowerer) answering(method string) (*ir.Type, *Class, bool) {
	cs := l.classesAnswering(method)
	if len(cs) == 0 {
		l.wantPromotion(method)
	}
	switch len(cs) {
	case 0:
		return nil, nil, false
	case 1:
		return cs[0].Ptr, cs[0], true
	}
	t, ok := l.commonInterface(cs)
	if !ok {
		return nil, nil, false
	}
	return t, l.classOf(t), true
}

// classesAnswering lists, in declaration order, every class in the file that
// has a method of this name.
//
// An accessor counts only once it has been promoted to a real method, because
// until then it is a struct field and an interface cannot promise a field.
func (l *Lowerer) classesAnswering(method string) []*Class {
	var out []*Class
	for _, name := range l.classOrd {
		c := l.classes[name]
		if c == nil || !c.IsType || c.Interface || c.Options {
			continue
		}
		s := c.method(method)
		if s == nil || s.Kind != SubMethod || s.Go == "" {
			continue
		}
		if s.Accessor != nil && !s.Promoted {
			continue
		}
		out = append(out, c)
	}
	return out
}

// wantPromotion records that a method call landed on an accessor through a
// value whose class did not resolve.
//
// Nothing is renamed while types are still being discovered: an early round
// often has not worked out what a variable holds yet, and a name changed on
// that guess would stay changed. The set is emptied at the start of every
// round, so what survives the loop is what the settled types asked for.
func (l *Lowerer) wantPromotion(method string) bool {
	if l.pass != 1 {
		return false
	}
	if l.promoteWanted == nil {
		l.promoteWanted = map[string]bool{}
	}
	l.promoteWanted[method] = true
	return false
}

// applyPromotions renames the accessors the discovery pass asked for, once,
// before the pass that builds the real tree.
func (l *Lowerer) applyPromotions() {
	names := make([]string, 0, len(l.promoteWanted))
	for name := range l.promoteWanted {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		l.promoteAccessors(name)
	}
}

// promoteAccessors turns the accessors named by a method call into real
// methods, so that an interface can promise them.
//
// Go has one namespace per type, so a field and a method cannot share a name.
// The exported name goes to the method, because that is the half other types
// can promise, and the field steps back to the unexported spelling. This is
// what a Go developer does when several types have to expose the same piece
// of state: the interface names the method, and each type answers it however
// it likes.
func (l *Lowerer) promoteAccessors(method string) bool {
	moved := false
	for _, name := range l.classOrd {
		c := l.classes[name]
		if c == nil || !c.IsType || c.Interface || c.Options {
			continue
		}
		s := c.method(method)
		if s == nil || s.Accessor == nil || s.Promoted || s.Setter {
			continue
		}
		f := s.Accessor
		s.Promoted = true
		s.Go = f.Go
		f.Go = unexportedName(f.Go)
		moved = true
	}
	return moved
}

// unexportedName lowers the first letter of a field name, and keeps it clear
// of the Go keywords a bare identifier cannot be.
func unexportedName(name string) string {
	if name == "" {
		return "field"
	}
	out := strings.ToLower(name[:1]) + name[1:]
	switch out {
	case "type", "func", "range", "map", "chan", "case", "default", "go",
		"var", "const", "interface", "struct", "select", "package", "import",
		"return", "if", "else", "for", "switch", "break", "continue", "defer",
		"goto", "fallthrough":
		return out + "Value"
	}
	return out
}

// dynamicDispatch lowers a method call on a value whose class did not
// resolve, by asserting the value to the type that answers to the method.
func (l *Lowerer) dynamicDispatch(n *ast.MethodCall, method string, recv ir.Expr) (ir.Expr, bool) {
	t, c, ok := l.answering(method)
	if !ok || c == nil {
		return nil, false
	}
	s := c.method(method)
	if s == nil {
		if c.Interface {
			s = c.subBy[method]
		}
		if s == nil {
			return nil, false
		}
	}
	asserted := &ir.TypeAssert{X: recv, Assert: t}
	asserted.T = t
	args, _ := l.listParts(n.Args)
	out := l.invoke(s, asserted, args, n)
	l.approximate(n, "P2G7002", "->"+method,
		"the value is asserted to "+t.String()+" before the call",
		"Nothing in the file pinned down which class this value holds, and Perl "+
			"would have found the method on whatever it turned out to be. Go needs a "+
			"type before it will accept the call, so the value is asserted to "+
			t.String()+", which is what answers to `"+method+"` here.",
		"The assertion panics if the value is something else, which is the moment "+
			"you want to hear about it. Where the value legitimately holds more than "+
			"one kind of thing, a type switch answers and hands back the typed value at "+
			"once.",
		"type-assertions-and-switches", "implicit-interfaces")
	l.note(asserted, "An interface value carries its concrete type with it, and the "+
		"assertion is how you get that type back out. It costs nothing when the "+
		"program is right and stops on the spot when it is not.",
		"type-assertions-and-switches")
	return out, true
}

// classForwarder recognises a class method whose whole body hands the
// arguments straight to the class's own constructor, which is what the throw
// idiom is:
//
//	sub throw { my $class = shift; die $class->new(@_) }
//
// There is nothing to generate for it. `$class` is the name the call was made
// on, so a subclass calling it builds a subclass; a Go function has no such
// name to work from, and one written from this body would build the parent
// every time whoever called it. Resolving the call where it is written is
// what keeps the class.
func classForwarder(s *Sub) (dies bool, ok bool) {
	if s == nil || s.Kind != SubClass || s.Decl == nil {
		return false, false
	}
	body := trimClassAlias(s.Decl.Body)
	if len(body) != 1 {
		return false, false
	}
	e, isExpr := body[0].(*ast.ExprStmt)
	if !isExpr {
		return false, false
	}
	x := e.X
	if c, isCall := x.(*ast.Call); isCall && c.Name == "die" && len(c.Args) == 1 {
		dies, x = true, c.Args[0]
	}
	mc, isMethod := x.(*ast.MethodCall)
	if !isMethod || mc.Method != "new" || len(mc.Args) != 1 {
		return false, false
	}
	if v, isVar := mc.Args[0].(*ast.Var); !isVar || v.Sigil != '@' || v.Name != "_" {
		return false, false
	}
	if _, isVar := mc.Invocant.(*ast.Var); !isVar {
		return false, false
	}
	return dies, true
}

// trimClassAlias drops a leading `my $class = shift` so the rest of the body
// can be looked at on its own.
func trimClassAlias(body []ast.Stmt) []ast.Stmt {
	if len(body) == 0 {
		return body
	}
	es, ok := body[0].(*ast.ExprStmt)
	if !ok {
		return body
	}
	as, ok := es.X.(*ast.Assign)
	if !ok || as.Op != "=" {
		return body
	}
	if _, ok := as.LHS.(*ast.My); !ok {
		return body
	}
	return body[1:]
}

// inlineForwarder lowers Class->throw(...) as the constructor call it stands
// for, at the class the call names.
func (l *Lowerer) inlineForwarder(n *ast.MethodCall, c *Class, s *Sub, dies bool) ir.Expr {
	ctor := c.Ctor
	if ctor == nil {
		return nil
	}
	value := l.callConstructor(n, c, ctor)
	if value == nil {
		return nil
	}
	l.approximate(n, "P2G7004", c.Perl+"->"+s.Name,
		"the forwarding class method is resolved here",
		"`"+s.Name+"` hands its arguments to `$class->new`, and `$class` is "+
			"whatever the call was made on. Go has no such name inside a function, so "+
			"a function written from that body would build "+s.Class.Perl+" however it "+
			"was called. The call is resolved at this line instead, where the class is "+
			"written down.",
		"Give each type a constructor and call it directly. A factory that picks a "+
			"type from a name is a map from string to a function in Go, declared where "+
			"the names are known.",
		"methods-and-receivers", "compile-time-mindset")
	if !dies {
		return value
	}
	for _, st := range l.throwValue(value, n) {
		l.emit(st)
	}
	return ir.BoolLit(true)
}

// classArgument reads the class a question such as isa was asked about, when
// it is written out and names a class this file declares.
func (l *Lowerer) classArgument(n *ast.MethodCall) (*Class, bool) {
	if len(n.Args) != 1 {
		return nil, false
	}
	name, ok := staticString(n.Args[0])
	if !ok {
		return nil, false
	}
	c, ok := l.classes[name]
	if !ok || !c.IsType || c.Interface {
		return nil, false
	}
	return c, true
}

// isaTest lowers ->isa('Class') on a value whose own class is not known.
//
// Perl answers by walking @ISA, so a Timeout is an IO error and an Error at
// once. Go embedding does not work that way: a type that embeds another is
// not assignable to it, and no assertion will say otherwise. The question is
// answered instead by naming every concrete type that inherits from the one
// asked about, which is a predicate the file can carry and the compiler can
// check.
func (l *Lowerer) isaTest(n ast.Node, recv ir.Expr, want *Class) ir.Expr {
	fn := l.isaPredicate(want)
	out := ir.CallOf(ir.NewIdent(fn, nil), ir.TBool, l.assignable(recv, ir.TAny, n))
	l.approximate(n, "P2G7003", "->isa('"+want.Perl+"')",
		"the inheritance test is spelled out as a type switch",
		"isa walks the inheritance chain, so a class counts as every one of its "+
			"ancestors. Go's embedding promotes fields and methods but is not "+
			"subtyping: a value of the embedding type will not satisfy an assertion to "+
			"the embedded one, so no single assertion answers this.",
		"The generated predicate lists the concrete types that inherit from "+
			want.Perl+". Adding a class to the hierarchy means adding it there too, "+
			"which is the cost Go charges for deciding this at compile time.",
		"type-assertions-and-switches", "structs-and-embedding", "late-binding-vs-embedding")
	return out
}

// isaPredicate declares, once per class asked about, the function that
// reports whether a value is that class or one that inherits from it.
func (l *Lowerer) isaPredicate(want *Class) string {
	// Only the pass that builds the real tree declares anything. Naming it
	// twice would take the name twice and leave the second one numbered.
	if l.pass != 2 {
		return "is" + want.Go
	}
	if l.isaFuncs == nil {
		l.isaFuncs = map[*Class]string{}
	}
	if name, ok := l.isaFuncs[want]; ok {
		return name
	}
	name := l.names.take("is" + want.Go)
	l.isaFuncs[want] = name

	var types []string
	var seen func(c *Class)
	seen = func(c *Class) {
		if c.IsType && !c.Interface {
			types = append(types, c.Ptr.String())
		}
		for _, k := range c.Children {
			seen(k)
		}
	}
	seen(want)
	sort.Strings(types)

	var b strings.Builder
	b.WriteString("func " + name + "(v any) bool {\n\tswitch v.(type) {\n\tcase ")
	b.WriteString(strings.Join(types, ", "))
	b.WriteString(":\n\t\treturn true\n\t}\n\treturn false\n}")
	d := &ir.RawDecl{
		Source: b.String(),
		Doc: []string{name + " reports whether v is a " + want.Go +
			" or one of the types that embed it."},
	}
	if l.pass == 2 {
		ir.Annotate(d, "Embedding promotes the embedded type's fields and methods, "+
			"but it is not subtyping: a value of the outer type will not satisfy an "+
			"assertion to the inner one. Listing the types is how a Go program answers "+
			"\"is it one of these\", and the compiler checks the list.",
			"structs-and-embedding", "type-assertions-and-switches")
	}
	l.isaDecls = append(l.isaDecls, d)
	return name
}

package lower

import (
	"strings"

	"perl2go/internal/ir"
	"perl2go/internal/perl/ast"
)

// This file lowers what a Perl package's subroutines become: methods on a
// pointer receiver, a constructor, or a plain function.

// nameMembers classifies a class's subs and gives each one its Go name.
//
// Naming happens before any lowering because Go puts a type's fields and its
// methods in one namespace: a hash key called `name` and a `sub name` are the
// same identifier here, and something has to give way. A sub that only reads
// the key is dropped in favour of the field, which is what Go code does
// anyway; anything else is renamed.
func (l *Lowerer) nameMembers(c *Class) {
	for _, s := range c.Subs {
		l.classifySub(c, s)
	}
	// Accessors settle the field names first: an accessor is not emitted at
	// all, so its name belongs to the field it reads.
	for _, s := range c.Subs {
		if key, setter, ok := accessorField(s); ok {
			f := l.declareField(c, key, s.Decl)
			s.Accessor = f
			s.Setter = setter
			s.Go = f.Go
		}
	}
	for _, s := range c.Subs {
		if s.Accessor != nil || s.Kind == SubSpecial {
			continue
		}
		switch s.Kind {
		case SubCtor:
			base := "New" + c.Go
			if s.Name != "new" {
				base = exportedName(s.Name) + c.Go
			}
			s.Go = l.names.take(base)
		case SubClass:
			s.Go = l.names.take(goName(c.Go + "_" + s.Name))
		case SubPlain:
			s.Go = l.plainName(c.Perl, s.Name)
		default:
			s.Go = l.methodName(c, s.Name)
		}
	}
	c.recv = l.receiverName(c)
}

// methodName picks a method name that no field or other method on the type,
// or on anything it embeds, has already claimed.
func (l *Lowerer) methodName(c *Class, perl string) string {
	base := exportedName(perl)
	if base == "" {
		base = "Method"
	}
	taken := func(name string) bool {
		if name == c.Go {
			return true
		}
		for _, a := range c.ancestors() {
			for _, f := range a.Fields {
				if f.Go == name {
					return true
				}
			}
			for _, s := range a.Subs {
				if s.Go == name && s.Class == a {
					return true
				}
			}
		}
		return false
	}
	if !taken(base) {
		return base
	}
	for i := 2; ; i++ {
		if cand := base + itoa(i); !taken(cand) {
			return cand
		}
	}
}

// receiverName picks the receiver identifier, which Go style keeps to a letter
// or two taken from the type's own name.
func (l *Lowerer) receiverName(c *Class) string {
	lower := strings.ToLower(c.Go)
	for n := 1; n <= len(lower) && n <= 3; n++ {
		cand := lower[:n]
		if !goKeywords[cand] && !goPredeclared[cand] && !l.names.has(cand) {
			return cand
		}
	}
	return l.names.take(goName(c.Go))
}

// classifySub decides what one sub of a class becomes.
func (l *Lowerer) classifySub(c *Class, s *Sub) {
	body := s.Decl.Body
	first, isSelf := selfParam(body)
	switch {
	case s.Name == "DESTROY" || s.Name == "AUTOLOAD":
		// Neither has a Go counterpart, and both are refused where they are
		// declared rather than at every call site.
		s.Kind = SubSpecial
		return
	case s.Name == "new" || blessesSomething(body):
		s.Kind = SubCtor
	case isSelf || usesFirstArgAsSelf(body):
		s.Kind = SubMethod
	case first == "class" || first == "proto" || first == "pkg":
		s.Kind = SubClass
	case l.qualCalls[qualify(c.Perl, s.Name)]:
		// The file calls it as Package::name(...), which is a function call
		// and not a method call.
		s.Kind = SubPlain
	case l.arrowCalls[s.Name]:
		s.Kind = SubMethod
	default:
		s.Kind = SubPlain
	}
	if s.Kind == SubCtor {
		if c.Ctor == nil || s.Name == "new" {
			c.Ctor = s
		}
	}
}

// usesFirstArgAsSelf reports whether a sub reaches into $_[0] as an object,
// which is the compact way Perl writes an accessor.
func usesFirstArgAsSelf(body []ast.Stmt) bool {
	found := false
	walkExprs(body, func(e ast.Expr) {
		switch n := e.(type) {
		case *ast.HashIndex:
			if isFirstArg(n.Base) {
				found = true
			}
		case *ast.MethodCall:
			if isFirstArg(n.Invocant) {
				found = true
			}
		}
	})
	return found
}

// isFirstArg reports whether an expression is $_[0].
func isFirstArg(e ast.Expr) bool {
	n, ok := e.(*ast.Index)
	if !ok {
		return false
	}
	v, ok := n.Base.(*ast.Var)
	if !ok || v.Name != "_" {
		return false
	}
	num, ok := n.Idx.(*ast.NumberLit)
	return ok && num.Text == "0"
}

// accessorField recognises the two shapes of hand-written accessor a Perl
// class has instead of a public field.
//
//	sub sku { $_[0]{sku} }
//	sub price { my $self = shift; $self->{price} = shift if @_; $self->{price} }
//
// Both disappear here. Go exports the field and lets the caller read and write
// it directly, which is why Go code has no getters.
func accessorField(s *Sub) (key string, setter bool, ok bool) {
	if s.Kind != SubMethod || s.Decl == nil {
		return "", false, false
	}
	body := s.Decl.Body
	self := ""
	if len(body) > 0 {
		if name, isSelf := selfParam(body); isSelf {
			self = name
			body = body[1:]
		}
	}
	if len(body) == 0 || len(body) > 2 {
		return "", false, false
	}
	if len(body) == 2 {
		st, is := body[0].(*ast.If)
		if !is || !st.Modifier || len(st.Then) != 1 || len(st.ElseIfs) > 0 || len(st.Else) > 0 {
			return "", false, false
		}
		if !isArgsVar(st.Cond) {
			return "", false, false
		}
		es, is := st.Then[0].(*ast.ExprStmt)
		if !is {
			return "", false, false
		}
		as, is := es.X.(*ast.Assign)
		if !is || as.Op != "=" || !isShiftArgs(as.RHS) {
			return "", false, false
		}
		k, is := selfKey(as.LHS, self)
		if !is {
			return "", false, false
		}
		key, setter = k, true
		body = body[1:]
	}

	var last ast.Expr
	switch st := body[0].(type) {
	case *ast.ExprStmt:
		last = st.X
	case *ast.Return:
		if len(st.Exprs) != 1 {
			return "", false, false
		}
		last = st.Exprs[0]
	default:
		return "", false, false
	}
	k, is := selfKey(last, self)
	if !is || (key != "" && k != key) {
		return "", false, false
	}
	return k, setter, true
}

// selfKey reads the literal hash key out of $self->{k} or $_[0]{k}.
func selfKey(e ast.Expr, self string) (string, bool) {
	h, ok := e.(*ast.HashIndex)
	if !ok {
		return "", false
	}
	key, ok := staticString(h.Key)
	if !ok {
		return "", false
	}
	if isFirstArg(h.Base) {
		return key, true
	}
	v, ok := h.Base.(*ast.Var)
	if !ok || v.Sigil != '$' || self == "" || v.Name != self {
		return "", false
	}
	return key, true
}

// ---------------------------------------------------------------------------
// Declaring the methods

// lowerMethodDecl builds the Go declaration for one sub of a class.
func (l *Lowerer) lowerMethodDecl(s *Sub, sd *ast.SubDecl) {
	c := s.Class
	if s.Kind == SubSpecial {
		l.refuseSpecialMethod(s, sd)
		return
	}
	if s.Accessor != nil {
		// Nothing is emitted: the field took the name and the callers read
		// it directly.
		if l.pass == 2 {
			l.inform(sd, "P2G7040", "sub "+s.Name,
				"This accessor is gone: `"+c.Go+"."+s.Accessor.Go+"` is read and written "+
					"directly. Go code does not write getters and setters for a plain field, "+
					"because an exported field already is the interface, and a method can be "+
					"added later without changing any caller.",
				"methods-and-receivers", "structs-and-embedding")
		}
		return
	}

	if s.Kind == SubCtor {
		s.SelfVar = ctorSelfName(sd.Body)
	}
	params, rest := l.recoverMethodParams(s, valueTail(sd.Body))

	fn := &ir.FuncDecl{Name: s.Go, Params: params}
	if s.Kind == SubMethod && s.Recv != nil {
		fn.Recv = &ir.Param{Name: s.Recv.Go, Type: c.Ptr}
	}
	if l.pass == 2 {
		fn.Results = s.Results
		fn.Doc = l.methodDoc(s)
	}
	fn.Body = l.markUnused(&ir.Block{Stmts: l.stmts(rest)})
	l.addImplicitReturn(s, fn)
	l.ensureReturn(s, fn.Body)
	l.setProv(fn, sd)
	l.explainMethod(fn, s)
	s.irDecl = fn
}

// methodDoc writes the doc comment for a generated method or constructor.
func (l *Lowerer) methodDoc(s *Sub) []string {
	if len(s.Doc) > 0 {
		if rest, ok := cutArticle(s.Doc[0]); ok {
			return append([]string{s.Go + " is " + rest}, s.Doc[1:]...)
		}
		return append([]string{s.Go + " is defined as follows."}, s.Doc...)
	}
	switch s.Kind {
	case SubCtor:
		return []string{s.Go + " builds a " + s.Class.Go + "."}
	case SubClass:
		return []string{s.Go + " is a class method of " + s.Class.Perl + ": it belongs to the " +
			"type rather than to any one value, so it is a plain function here."}
	}
	return []string{s.Go + " is a method of " + s.Class.Go + "."}
}

// explainMethod attaches the lesson a generated method deserves.
func (l *Lowerer) explainMethod(fn *ir.FuncDecl, s *Sub) {
	if l.pass != 2 {
		return
	}
	switch s.Kind {
	case SubCtor:
		l.note(fn, "Perl's constructor is an ordinary sub that blesses a reference and "+
			"hands it back, and the class name arrives as its first argument. Go has no "+
			"constructors at all: a function returning *"+s.Class.Go+" is the whole "+
			"convention, and by custom it is named New followed by the type.",
			"methods-and-receivers", "pointers-vs-references")
		if s.Named != nil {
			l.note(fn, "The Perl took its arguments as a `%args` hash, so a caller could "+
				"name them and leave any of them out. Go has neither named nor optional "+
				"arguments: the keys the constructor actually read have become parameters "+
				"in the order it read them, and a caller that omitted one now passes the "+
				"zero value explicitly.",
				"variadic-and-no-defaults", "static-types-and-zero-values")
		}
	case SubClass:
		l.note(fn, "Perl calls this with the class name in place of an object, and "+
			"inheritance makes that name whichever subclass the caller wrote. Go has no "+
			"such dispatch: the function belongs to this type and nothing else, which is "+
			"why it takes no receiver.",
			"methods-and-receivers")
	case SubMethod:
		l.note(fn, "A Perl method is a sub whose first argument happens to be the "+
			"object. Go writes the receiver before the name, so the compiler knows which "+
			"type the method belongs to. The receiver is a pointer because the method may "+
			"change the object; a value receiver would work on a copy, which `bless` "+
			"never did.",
			"methods-and-receivers", "pointers-vs-references")
	}
}

// recoverMethodParams turns the argument unpacking at the top of a method into
// a receiver and a Go parameter list.
func (l *Lowerer) recoverMethodParams(s *Sub, body []ast.Stmt) ([]ir.Param, []ast.Stmt) {
	rest := body
	var bindings []*Binding
	// A plain function in a package takes no invocant: it is called by name,
	// not through an arrow, so its first argument is an ordinary parameter.
	takesInvocant := s.Kind != SubPlain

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
		if !isArgsVar(as.RHS) && !isShiftArgs(as.RHS) {
			break
		}
		vars := declaredVars(my)
		if len(vars) == 0 {
			break
		}
		for _, v := range vars {
			switch {
			case takesInvocant:
				takesInvocant = false
				b := l.declare(v, KindParam)
				if s.Kind == SubMethod {
					s.Recv = b
					b.Type = s.Class.Ptr
					b.Kind = KindParam
					b.Go = s.Class.recv
				} else {
					// The invocant of a constructor or a class method is the
					// class name, which Go does not pass at all.
					b.Type = ir.TString
					b.Kind = KindSpecial
					l.classVars[b] = s.Class
				}
			case v.Sigil == '%' && (s.Kind == SubCtor || s.Kind == SubClass) && s.VarArgs == nil:
				b := l.declare(v, KindParam)
				b.Kind = KindSpecial
				s.Named = b
			case v.Sigil == '$':
				bindings = append(bindings, l.declare(v, KindParam))
			default:
				b := l.declare(v, KindParam)
				s.Variadic = true
				s.VarArgs = b
			}
		}
		rest = rest[1:]
		if s.VarArgs != nil {
			break
		}
	}

	if takesInvocant && s.Kind == SubMethod {
		// The method reached into $_[0] instead of naming its object, or it
		// ignores the object entirely. Either way it still needs a receiver.
		b := l.declareNamed("self@"+s.Class.Perl+"::"+s.Name, '$', "self", KindParam, s.Decl)
		b.Type = s.Class.Ptr
		b.Go = s.Class.recv
		s.Recv = b
	}
	// `my $class = ref($proto) || $proto;` picks the class to bless into, and
	// works whether the constructor was called on the class or on an object.
	// Go has one type here either way, so the line has nothing left to say.
	rest = l.dropClassAlias(s, rest)

	s.Params = bindings
	var params []ir.Param
	for _, b := range bindings {
		params = append(params, ir.Param{Name: b.Go, Type: b.Type})
	}
	if s.Named != nil {
		for _, b := range s.NamedParams {
			params = append(params, ir.Param{Name: b.Go, Type: b.Type})
		}
	}
	if s.VarArgs != nil {
		params = append(params, ir.Param{Name: s.VarArgs.Go, Type: elemOf(s.VarArgs.Type), Variadic: true})
	}
	return params, rest
}

// dropClassAlias removes the `my $class = ref($proto) || $proto` line and
// records that the name it declared stands for this class.
func (l *Lowerer) dropClassAlias(s *Sub, rest []ast.Stmt) []ast.Stmt {
	for len(rest) > 0 {
		es, ok := rest[0].(*ast.ExprStmt)
		if !ok {
			return rest
		}
		as, ok := es.X.(*ast.Assign)
		if !ok || as.Op != "=" {
			return rest
		}
		my, ok := as.LHS.(*ast.My)
		if !ok || len(my.Vars) != 1 {
			return rest
		}
		v, ok := my.Vars[0].(*ast.Var)
		if !ok || v.Sigil != '$' {
			return rest
		}
		if !l.namesThisClass(as.RHS) {
			return rest
		}
		b := l.declare(v, KindParam)
		b.Kind = KindSpecial
		b.Type = ir.TString
		l.classVars[b] = s.Class
		if l.pass == 2 {
			l.inform(es, "P2G7050", "ref($proto) || $proto",
				"This picks the class to bless into so that the constructor works when "+
					"it is called on an object as well as on the class name. Go has one type "+
					"here whichever way it was called, so the line disappears.",
				"methods-and-receivers")
		}
		rest = rest[1:]
	}
	return rest
}

// namesThisClass reports whether an expression is one of the idioms that
// yields the current class name: `$proto`, `ref($proto) || $proto`, `shift`.
func (l *Lowerer) namesThisClass(e ast.Expr) bool {
	switch n := e.(type) {
	case *ast.Var:
		if n.Sigil != '$' {
			return false
		}
		b, ok := l.scope.lookup(varKey('$', n.Name))
		return ok && l.classVars[b] != nil
	case *ast.BinOp:
		if n.Op == "||" || n.Op == "or" || n.Op == "//" {
			return l.namesThisClass(n.L) && l.namesThisClass(n.R)
		}
	case *ast.Call:
		if n.Name == "ref" && len(n.Args) == 1 {
			return l.namesThisClass(n.Args[0])
		}
	}
	return false
}

// namedParam returns the Go parameter standing in for one key of a `%args`
// hash, creating it the first time the constructor reads that key.
func (l *Lowerer) namedParam(s *Sub, key string, at ast.Node) *Binding {
	if s.namedBy == nil {
		s.namedBy = map[string]*Binding{}
	}
	if b, ok := s.namedBy[key]; ok {
		return b
	}
	b := l.declareNamed("named@"+s.Pkg+"::"+s.Name+"::"+key, '$', key, KindParam, at)
	b.Perl = "$args{" + key + "}"
	s.namedBy[key] = b
	s.NamedParams = append(s.NamedParams, b)
	return b
}

// ---------------------------------------------------------------------------
// Calling methods

// classOf finds the class a value's type belongs to.
func (l *Lowerer) classOf(t *ir.Type) *Class {
	if t == nil {
		return nil
	}
	if t.Kind == ir.Pointer {
		t = t.Elem
	}
	if t == nil || t.Kind != ir.Named {
		return nil
	}
	return l.byGoType[t.Name]
}

// classNamed reads a class out of an expression used as an invocant: a
// bareword class name, or a variable holding this class's name.
func (l *Lowerer) classNamed(e ast.Expr) (*Class, bool) {
	switch n := e.(type) {
	case *ast.FileHandle:
		// A bareword before an arrow is a class name, which the parser hands
		// on without deciding what it is.
		if c, ok := l.classes[n.Name]; ok && c.IsType {
			return c, true
		}
	case *ast.Call:
		if len(n.Args) == 0 && !n.Paren {
			if c, ok := l.classes[n.Name]; ok && c.IsType {
				return c, true
			}
		}
	case *ast.StrLit:
		if c, ok := l.classes[n.Value]; ok && c.IsType {
			return c, true
		}
	case *ast.Var:
		if n.Sigil == '$' {
			if b, ok := l.scope.lookup(varKey('$', n.Name)); ok {
				if c := l.classVars[b]; c != nil {
					return c, true
				}
			}
		}
	}
	return nil, false
}

// methodCall lowers $obj->method(...) and Class->method(...).
func (l *Lowerer) methodCall(n *ast.MethodCall) ir.Expr {
	if n.Dynamic != nil {
		return l.todoExpr(n, "P2G7015", "method call through a variable",
			"the method name is only known at run time",
			"The method to call is held in a variable, so Perl looks it up in the "+
				"symbol table when the line runs. Go resolves method names when it "+
				"compiles, and there is nothing to look up in.",
			"Where the choice is between a few known methods, a switch on the name is "+
				"the direct translation. Where it is open-ended, declare an interface and "+
				"let each type implement it.",
			"implicit-interfaces", "type-assertions-and-switches")
	}

	method := n.Method
	super := false
	if rest, cut := strings.CutPrefix(method, "SUPER::"); cut {
		method, super = rest, true
	}
	if i := strings.LastIndex(method, "::"); i >= 0 {
		// Fully qualified call: Class::method($obj, ...) written with an arrow.
		if c, ok := l.classes[method[:i]]; ok && c.IsType {
			return l.dispatch(n, c, method[i+2:], false, nil)
		}
	}

	// Class->method: the invocant is a name, not an object.
	if c, ok := l.classNamed(n.Invocant); ok {
		return l.classDispatch(n, c, method)
	}

	if bw, ok := n.Invocant.(*ast.FileHandle); ok {
		return l.todoExpr(n, "P2G7041", bw.Name+"->"+method,
			"this class was not declared in this file",
			"The call names "+bw.Name+" as a class, and no `package "+bw.Name+"` appears "+
				"in this file for the converter to build a type from.",
			"Convert the module that declares the class too, so the type and its methods "+
				"land in the same package.",
			"methods-and-receivers", "go-mod-vs-cpan")
	}
	recv := l.expr(n.Invocant)
	if recv == nil {
		return ir.Nil(ir.TAny)
	}
	c := l.classOf(typeOrAny(recv))
	if c == nil {
		return l.unknownInvocant(n, method, recv)
	}
	if super {
		if c.Parent == nil || !c.Parent.IsType {
			return l.todoExpr(n, "P2G7010", "SUPER::"+method,
				"this class has no parent to call up to",
				"SUPER:: calls the version of a method that the parent class defines, "+
					"and the converter could not find a parent for this class in the file.",
				"If the parent lives in another file, convert it too and put both types in "+
					"the same package.",
				"structs-and-embedding")
		}
		return l.dispatch(n, c.Parent, method, true, selector(recv, c.Parent.Go, c.Parent.Value))
	}
	return l.dispatch(n, c, method, false, recv)
}

// unknownInvocant is the answer when nothing in the file said what class the
// object belongs to.
func (l *Lowerer) unknownInvocant(n *ast.MethodCall, method string, recv ir.Expr) ir.Expr {
	return l.todoExpr(n, "P2G7001", "method call on "+method,
		"the class of this object did not resolve",
		"Perl decides which method to run by looking at what the reference was "+
			"blessed into, at the moment the call runs. Nothing in this file pinned "+
			"down which class this value holds, so there is no Go type to hang the "+
			"call on.",
		"Declare the variable with the concrete type where it is created. Where it "+
			"really can hold more than one class, an interface naming the methods they "+
			"share is the Go answer.",
		"methods-and-receivers", "implicit-interfaces")
}

// classDispatch lowers Class->method(...): a constructor, a class method, or
// one of the questions Perl asks of the class itself.
func (l *Lowerer) classDispatch(n *ast.MethodCall, c *Class, method string) ir.Expr {
	switch method {
	case "isa":
		return l.isaCall(n, c)
	case "can":
		return l.canCall(n, c)
	case "new":
		if c.Ctor != nil {
			return l.callConstructor(n, c, c.Ctor)
		}
		return l.defaultConstructor(n, c)
	}
	s := c.method(method)
	if s == nil {
		return l.todoExpr(n, "P2G7041", c.Perl+"->"+method,
			"no such method was found in this file",
			"The converter read every package in this file and found no `sub "+method+
				"` in "+c.Perl+" or in anything it inherits from.",
			"If the class comes from a module, convert that file too so the type and its "+
				"methods land in the same package.",
			"methods-and-receivers")
	}
	switch s.Kind {
	case SubCtor:
		return l.callConstructor(n, c, s)
	case SubClass, SubPlain:
		args, _ := l.listParts(n.Args)
		return l.callFunction(s, args, n)
	}
	return l.todoExpr(n, "P2G7042", c.Perl+"->"+method,
		"an instance method was called on the class itself",
		"`"+method+"` needs an object: it reads the fields of the thing it was called "+
			"on. Perl allows the class name to stand in and fails later; Go needs a "+
			"receiver of the right type at the call.",
		"Build the object first and call the method on it.",
		"methods-and-receivers")
}

// dispatch lowers a call on an object whose class is known.
func (l *Lowerer) dispatch(n *ast.MethodCall, c *Class, method string, super bool, recv ir.Expr) ir.Expr {
	switch method {
	case "isa", "DOES":
		return l.isaCall(n, c)
	case "can":
		return l.canCall(n, c)
	}
	s := c.method(method)
	if s == nil {
		return l.todoExpr(n, "P2G7041", "->"+method,
			"no such method was found in this file",
			"The converter read every package in this file and found no `sub "+method+
				"` in "+c.Perl+" or in anything it inherits from.",
			"If the class comes from a module, convert that file too so the type and its "+
				"methods land in the same package.",
			"methods-and-receivers")
	}
	if s.Accessor != nil {
		return l.accessorRead(n, recv, s)
	}
	args, _ := l.listParts(n.Args)
	switch s.Kind {
	case SubCtor:
		return l.callConstructor(n, c, s)
	case SubClass, SubPlain:
		return l.callFunction(s, args, n)
	}
	if recv == nil {
		return l.todoExpr(n, "P2G7042", "->"+method,
			"an instance method was called without an object",
			"`"+method+"` reads the fields of the object it is called on, and there is "+
				"no object at this call.",
			"Build the object first and call the method on it.",
			"methods-and-receivers")
	}
	l.noteLateBinding(n, c, s, super)
	return l.invoke(s, recv, args, n)
}

// invoke builds the Go method call itself.
func (l *Lowerer) invoke(s *Sub, recv ir.Expr, args []ir.Expr, n ast.Node) ir.Expr {
	s.CallSites++
	out := l.fitArgs(s, args, n)
	ret := ir.TVoid
	if len(s.Results) > 0 {
		ret = s.Results[0]
	}
	c := ir.CallOf(selector(recv, s.Go, nil), ret, out...)
	if len(s.Results) == 0 {
		l.emit(exprStmt(c))
		return ir.Nil(ir.TAny)
	}
	return c
}

// callFunction builds a call to a class method or a package function.
func (l *Lowerer) callFunction(s *Sub, args []ir.Expr, n ast.Node) ir.Expr {
	s.CallSites++
	out := l.fitArgs(s, args, n)
	ret := ir.TVoid
	if len(s.Results) > 0 {
		ret = s.Results[0]
	}
	c := ir.CallOf(ir.NewIdent(s.Go, nil), ret, out...)
	if len(s.Results) == 0 {
		l.emit(exprStmt(c))
		return ir.Nil(ir.TAny)
	}
	return c
}

// fitArgs matches a Perl argument list to a Go signature, observing the types
// on the way and filling any gap with a zero value.
func (l *Lowerer) fitArgs(s *Sub, args []ir.Expr, n ast.Node) []ir.Expr {
	var out []ir.Expr
	for i, a := range args {
		if i < len(s.Params) {
			p := s.Params[i]
			l.observe(p, typeOrAny(a))
			out = append(out, l.assignable(a, p.Type, nil))
			continue
		}
		if s.VarArgs != nil {
			if at := typeOrAny(a); at.Kind == ir.Slice {
				l.observeElem(s.VarArgs, elemOf(at))
				out = append(out, l.assignable(a, ir.SliceOf(elemOf(s.VarArgs.Type)), nil))
				continue
			}
			l.observeElem(s.VarArgs, typeOrAny(a))
			out = append(out, l.assignable(a, elemOf(s.VarArgs.Type), nil))
			continue
		}
		if l.pass == 2 && i == len(s.Params) {
			l.approximate(n, "P2G2130", "call with extra arguments",
				"the extra arguments are dropped",
				"This call passes more values than the sub unpacks. In Perl the extras sit "+
					"in @_ where nothing reads them; a Go call has to match the signature.",
				"If the extras were meant to be used, add parameters for them.",
				"variadic-and-no-defaults")
		}
	}
	for i := len(out); i < len(s.Params); i++ {
		out = append(out, zeroOf(s.Params[i].Type))
	}
	return out
}

// callConstructor lowers Class->new(...), matching named arguments to the
// parameters the constructor turned them into.
func (l *Lowerer) callConstructor(n *ast.MethodCall, c *Class, s *Sub) ir.Expr {
	s.CallSites++
	var out []ir.Expr
	if s.Named != nil {
		out = l.namedArgs(s, n)
	} else {
		args, _ := l.listParts(n.Args)
		out = l.fitArgs(s, args, n)
	}
	ret := c.Ptr
	if len(s.Results) > 0 {
		ret = s.Results[0]
	}
	call := ir.CallOf(ir.NewIdent(s.Go, nil), ret, out...)
	l.note(call, "Perl calls the constructor through the class name and the arrow, "+
		"which is a symbol-table lookup at run time. Go calls an ordinary function, "+
		"resolved when the program is compiled.",
		"methods-and-receivers")
	return call
}

// defaultConstructor covers a class that inherits its constructor from a
// parent, which is the usual shape for an exception subclass.
func (l *Lowerer) defaultConstructor(n *ast.MethodCall, c *Class) ir.Expr {
	if p := c.Parent; p != nil && p.IsType && p.Ctor != nil {
		return l.todoExpr(n, "P2G7047", c.Perl+"->new",
			"this class inherits its constructor",
			c.Perl+" has no constructor of its own and Perl finds the parent's by "+
				"walking @ISA at run time. The parent's constructor blesses into whichever "+
				"class was named at the call, which is a decision Go makes when it compiles.",
			"Write a constructor for this type that fills in the embedded "+p.Go+
				" and returns *"+c.Go+".",
			"structs-and-embedding", "late-binding-vs-embedding")
	}
	lit := composite(c.Value, nil, nil)
	out := ir.Un("&", lit, c.Ptr)
	l.note(out, "The Perl class has no constructor of its own, so the object is the "+
		"struct's zero value. Go guarantees that every field starts zeroed, which is "+
		"why a composite literal with nothing in it is a complete value.",
		"static-types-and-zero-values")
	return out
}

// namedArgs turns a `key => value` call into the positional argument list the
// generated constructor takes.
func (l *Lowerer) namedArgs(s *Sub, n *ast.MethodCall) []ir.Expr {
	flat := make([]ast.Expr, 0, len(n.Args))
	for _, a := range n.Args {
		flat = append(flat, flatten(a)...)
	}
	given := map[string]ir.Expr{}
	var unknown []string
	ok := len(flat)%2 == 0
	for i := 0; ok && i+1 < len(flat); i += 2 {
		key, isLit := staticString(flat[i])
		if !isLit {
			ok = false
			break
		}
		value := l.scalar(flat[i+1])
		p := s.namedBy[key]
		if p == nil {
			// A key the constructor never reads has nowhere to go, but the
			// value may still be one this pass needs to see.
			if l.pass == 1 {
				p = l.namedParam(s, key, n)
			} else {
				unknown = append(unknown, key)
				continue
			}
		}
		l.observe(p, typeOrAny(value))
		given[key] = value
	}
	if !ok {
		if l.pass == 2 {
			l.approximate(n, "P2G7043", "constructor arguments built at run time",
				"the arguments could not be matched to parameters",
				"The Perl passes its constructor a list built at run time rather than a "+
					"written-out set of key/value pairs, so which key goes with which "+
					"parameter is not knowable here.",
				"Pass the values positionally, in the order the generated constructor "+
					"declares them.",
				"variadic-and-no-defaults")
		}
		var out []ir.Expr
		for _, p := range s.NamedParams {
			out = append(out, zeroOf(p.Type))
		}
		return out
	}
	if len(unknown) > 0 && l.pass == 2 {
		l.approximate(n, "P2G7044", "constructor argument "+strings.Join(unknown, ", "),
			"an argument the constructor never reads was dropped",
			"The call names "+strings.Join(unknown, ", ")+", and the constructor never "+
				"looks at that key. In Perl the value sits in %args unread; here there is "+
				"no parameter for it.",
			"Remove it from the call, or read it in the constructor so it becomes a field.")
	}
	out := make([]ir.Expr, 0, len(s.NamedParams))
	for _, p := range s.NamedParams {
		if v, found := given[p.Perl[len("$args{"):len(p.Perl)-1]]; found {
			out = append(out, l.assignable(v, p.Type, nil))
			continue
		}
		out = append(out, zeroOf(p.Type))
	}
	return out
}

// ---------------------------------------------------------------------------
// The questions Perl asks about an object

// isaCall answers ->isa('Class') from the class hierarchy the file declares.
func (l *Lowerer) isaCall(n *ast.MethodCall, c *Class) ir.Expr {
	name, ok := "", false
	if len(n.Args) == 1 {
		name, ok = staticString(n.Args[0])
	}
	if !ok {
		return l.todoExpr(n, "P2G7045", "->isa",
			"the class being asked about is not a literal",
			"isa walks @ISA at run time to answer whether an object belongs to a class. "+
				"Go has no run-time class chain to walk, so the answer has to be known "+
				"when the program is compiled, and here the name is computed.",
			"Where the set of classes is small, a type switch answers the same question "+
				"and the compiler checks the arms.",
			"type-assertions-and-switches")
	}
	out := ir.BoolLit(c.isa(name))
	l.note(out, "Perl answers isa by walking the class's @ISA chain while the program "+
		"runs. Every class in this file is a Go type with a fixed relationship to the "+
		"others, so the answer is settled here: "+c.Perl+" "+isaWord(c.isa(name))+" a "+
		name+". Where the answer really varies, a type switch is what Go uses.",
		"type-assertions-and-switches", "structs-and-embedding")
	l.approximate(n, "P2G7045", "->isa('"+name+"')",
		"the answer was decided at conversion time",
		"The value's class is known here, so isa has one answer and it is written in "+
			"as a constant. Perl would have asked again on every call, which matters "+
			"only if something changes @ISA while the program runs.",
		"If the variable can hold more than one class, give it an interface type and "+
			"use a type switch instead.",
		"type-assertions-and-switches")
	return out
}

func isaWord(v bool) string {
	if v {
		return "is"
	}
	return "is not"
}

// canCall answers ->can('name') from the methods the file declares.
func (l *Lowerer) canCall(n *ast.MethodCall, c *Class) ir.Expr {
	name, ok := "", false
	if len(n.Args) == 1 {
		name, ok = staticString(n.Args[0])
	}
	if !ok {
		return l.todoExpr(n, "P2G7020", "->can",
			"the method being asked about is not a literal",
			"can looks a method up in the symbol table while the program runs and hands "+
				"back a code reference. Go has no symbol table to look in and no way to "+
				"ask a type whether it has a method by name.",
			"Declare an interface with the method on it and use a type assertion: the "+
				"two-result form asks exactly this question and the compiler checks the "+
				"method's signature as well as its name.",
			"implicit-interfaces", "type-assertions-and-switches")
	}
	has := c.method(name) != nil
	out := ir.BoolLit(has)
	l.note(out, "Perl's can searches the class and its parents for a sub of that name "+
		"and returns a code reference. The methods of "+c.Go+" are fixed when this "+
		"program is compiled, so the answer is a constant. The Go way to ask the same "+
		"question of a value whose type varies is a type assertion against an "+
		"interface: v, ok := x.(interface{ "+exportedName(name)+"() }).",
		"implicit-interfaces", "type-assertions-and-switches")
	l.approximate(n, "P2G7020", "->can('"+name+"')",
		"the answer was decided at conversion time",
		"The receiver's type is known here, so whether it has that method is settled "+
			"and written in as a constant. Perl asked the symbol table, which can change "+
			"while the program runs.",
		"Where the value's type varies, assert it against an interface naming the "+
			"method and use the two-result form.",
		"implicit-interfaces")
	return out
}

// noteLateBinding says so where Perl would have dispatched to an override and
// Go's embedding will not.
//
// This is the one place the struct-and-embedding mapping stops being faithful.
// Perl looks the method up on the object's real class every time, so a method
// in the base class calling $self->area reaches the subclass's area. Go
// resolves the call against the type the receiver is declared as, which inside
// a base method is always the base.
func (l *Lowerer) noteLateBinding(n *ast.MethodCall, c *Class, s *Sub, super bool) {
	if l.pass != 2 || super || l.curClass == nil {
		return
	}
	if s.Class == nil || s.Class == l.curClass {
		return
	}
	if !l.overridden(s) {
		return
	}
	l.approximate(n, "P2G7046", "->"+n.Method+" inside a method of "+l.curClass.Perl,
		"an overridden method will not be reached from the base class",
		"Perl looks a method up on the object's real class every time it is called, so "+
			s.Class.Perl+"::"+n.Method+" called from here would run the subclass's "+
			"version. Go resolves the call against the type of the receiver, which "+
			"inside this method is "+l.curClass.Go+", so the embedded "+s.Class.Go+
			"'s version runs and the override is not reached.",
		"Declare an interface with the methods the base calls on itself, give the base "+
			"struct a field of that interface type, and have each constructor store the "+
			"finished object in it. The base then calls through the interface and the "+
			"override is reached. That is composition plus an interface, which is how Go "+
			"expresses a template method.",
		"late-binding-vs-embedding", "implicit-interfaces", "structs-and-embedding")
}

// overridden reports whether any subclass declares its own version of a method.
func (l *Lowerer) overridden(s *Sub) bool {
	if s.Class == nil {
		return false
	}
	var walk func(c *Class) bool
	walk = func(c *Class) bool {
		for _, ch := range c.Children {
			if _, ok := ch.subBy[s.Name]; ok {
				return true
			}
			if walk(ch) {
				return true
			}
		}
		return false
	}
	return walk(s.Class)
}

// ---------------------------------------------------------------------------
// Accessors

// accessorRead lowers $obj->field, which is a plain field read in Go.
func (l *Lowerer) accessorRead(n *ast.MethodCall, recv ir.Expr, s *Sub) ir.Expr {
	if len(n.Args) > 0 {
		if !s.Setter {
			return l.todoExpr(n, "P2G7049", "->"+n.Method+" with an argument",
				"this accessor only reads",
				"The Perl sub behind this call ignores anything passed to it, so the "+
					"argument does nothing.",
				"Drop the argument, or assign to the field directly.")
		}
		// A read/write accessor called with a value is an assignment, and an
		// assignment is not an expression in Go. The statement layer handles
		// the common case; reaching here means it was used for its value.
		value := l.scalar(n.Args[0])
		l.observeField(s.Accessor, typeOrAny(value))
		st := assign("=", []ir.Expr{selector(recv, s.Accessor.Go, s.Accessor.Type)},
			[]ir.Expr{l.assignable(value, s.Accessor.Type, n.Args[0])})
		l.setProv(st, n)
		l.emit(st)
		return selector(recv, s.Accessor.Go, s.Accessor.Type)
	}
	out := selector(recv, s.Accessor.Go, s.Accessor.Type)
	l.note(out, "Perl hides a field behind a sub so that callers go through an "+
		"interface. Go exports the field instead: reading it is the same expression "+
		"the accessor would have returned, and a method can replace the field later "+
		"without changing a single caller.",
		"methods-and-receivers")
	return out
}

// ---------------------------------------------------------------------------
// Fields

// namedArgRead resolves $args{key} inside a constructor to the Go parameter
// that named argument became.
func (l *Lowerer) namedArgRead(n *ast.HashIndex) (*Binding, bool) {
	s := l.curSub
	if s == nil || s.Named == nil || n.Arrow {
		return nil, false
	}
	v, ok := n.Base.(*ast.Var)
	if !ok || v.Sigil != '$' {
		return nil, false
	}
	b, found := l.scope.lookup(varKey('%', v.Name))
	if !found || b != s.Named {
		return nil, false
	}
	key, ok := staticString(n.Key)
	if !ok {
		return nil, false
	}
	return l.namedParam(s, key, n), true
}

// ctorSelfName finds the variable a constructor blesses, which is the one that
// has to be built as the struct.
func ctorSelfName(body []ast.Stmt) string {
	name := ""
	walkExprs(body, func(e ast.Expr) {
		c, ok := e.(*ast.Call)
		if !ok || c.Name != "bless" || len(c.Args) == 0 || name != "" {
			return
		}
		if v, ok := c.Args[0].(*ast.Var); ok && v.Sigil == '$' {
			name = v.Name
		}
	})
	return name
}

// ctorSelf reports the class when a declaration is of the variable a
// constructor blesses.
func (l *Lowerer) ctorSelf(v *ast.Var) *Class {
	s := l.curSub
	if s == nil || s.Kind != SubCtor || s.Class == nil || s.SelfVar == "" {
		return nil
	}
	if v.Sigil != '$' || v.Name != s.SelfVar {
		return nil
	}
	return s.Class
}

// structLit builds the composite literal a blessed hash reference becomes.
func (l *Lowerer) structLit(c *Class, h *ast.AnonHash) ir.Expr {
	var flat []ast.Expr
	for _, e := range h.Elems {
		flat = append(flat, flatten(e)...)
	}
	var keys, vals []ir.Expr
	for i := 0; i+1 < len(flat); i += 2 {
		key, ok := staticString(flat[i])
		if !ok {
			if l.pass == 2 {
				l.approximate(flat[i], "P2G7048", "computed key in a blessed hash",
					"a field name worked out at run time was dropped",
					"Perl builds the object's keys from a list, so a key can be computed. "+
						"A Go struct's fields are fixed when the program is compiled.",
					"Give the object a map field for the part whose keys vary, and keep the "+
						"fixed keys as fields.")
			}
			continue
		}
		f := l.declareField(c, key, flat[i])
		value := l.scalar(flat[i+1])
		l.observeField(f, typeOrAny(value))
		keys = append(keys, ir.NewIdent(f.Go, nil))
		vals = append(vals, l.assignable(value, f.Type, flat[i+1]))
	}
	lit := composite(c.Value, keys, vals)
	out := ir.Un("&", lit, c.Ptr)
	l.note(out, "`bless` marks a reference as belonging to a class, and from then on "+
		"the class is a property of the value that Perl checks at every method call. "+
		"Go has no such marking: the type is the struct itself, decided here and "+
		"checked by the compiler. The & takes the address so that methods with a "+
		"pointer receiver can change the object rather than a copy of it.",
		"methods-and-receivers", "pointers-vs-references", "structs-and-embedding")
	return out
}

// blessCall lowers `bless REF, CLASS`.
func (l *Lowerer) blessCall(n *ast.Call) (ir.Expr, bool) {
	if len(n.Args) == 0 {
		return nil, false
	}
	c := l.blessTarget(n)
	if c == nil {
		return nil, false
	}
	switch arg := n.Args[0].(type) {
	case *ast.AnonHash:
		return l.structLit(c, arg), true
	case *ast.Var:
		x := l.expr(arg)
		if l.classOf(typeOrAny(x)) == c {
			l.note(x, "The reference was built as this type a few lines up, so `bless` "+
				"has nothing left to do: in Go the value already knows what it is.",
				"methods-and-receivers")
			return x, true
		}
	}
	return nil, false
}

// blessTarget works out which class a bless names.
func (l *Lowerer) blessTarget(n *ast.Call) *Class {
	if len(n.Args) > 1 {
		if c, ok := l.classNamed(n.Args[1]); ok {
			return c
		}
		if name, ok := staticString(n.Args[1]); ok {
			if c, found := l.classes[name]; found && c.IsType {
				return c
			}
		}
		return nil
	}
	// A one-argument bless uses the current package.
	if c, ok := l.classes[l.curPkg]; ok && c.IsType {
		return c
	}
	return nil
}

// ---------------------------------------------------------------------------
// Fields as containers

// container is a place the program stores into: a variable, or a struct field.
// A field has no binding to hang type evidence on, so the two are handled
// together wherever a write says what a container holds.
type container struct {
	bind  *Binding
	field *ClassField
}

// containerOf finds the place an expression names, reaching through the
// dereference Perl needs and Go does not.
func (l *Lowerer) containerOf(e ast.Expr) container {
	if f := l.fieldOf(e); f != nil {
		return container{field: f}
	}
	// The binding is looked up from the expression as written. A dereference
	// has no binding of its own, and treating `@{ $h{k} }` as naming %h would
	// say that the hash holds what the list holds rather than lists of it.
	return container{bind: l.bindingOfTarget(e)}
}

// fieldOf reports the struct field an expression names, reaching through the
// dereference Perl writes around a reference and Go does not need.
func (l *Lowerer) fieldOf(e ast.Expr) *ClassField {
	for {
		d, ok := e.(*ast.Deref)
		if !ok {
			break
		}
		e = d.X
	}
	return l.fieldAt[e]
}

// observeIn records the type of what goes into a container.
func (l *Lowerer) observeIn(c container, t *ir.Type, hash bool) {
	if c.field != nil {
		if hash {
			l.observeField(c.field, ir.MapOf(t))
		} else {
			l.observeField(c.field, ir.SliceOf(t))
		}
		return
	}
	l.observeElem(c.bind, t)
}

// refuseSpecialMethod turns down the two methods perl calls on a program's
// behalf. Neither has anything to resolve to in Go, and both are refused where
// they are written rather than at every place they would have fired.
func (l *Lowerer) refuseSpecialMethod(s *Sub, sd *ast.SubDecl) {
	if l.pass != 2 {
		return
	}
	if s.Name == "DESTROY" {
		l.refuse(sd, "P2G7030", "sub DESTROY",
			"a destructor has no Go equivalent",
			"Perl runs DESTROY the instant the last reference to an object goes away, "+
				"which is what makes a guard object work: the release happens at a "+
				"closing brace or at an undef, in order, every time. Go's collector runs "+
				"when it chooses and may never run at all before the program exits.",
			"Give the type a Close method and call it with defer where the object is "+
				"created. That is the same guarantee written where a reader can see it, "+
				"and it is what every Go type holding a resource does.",
			"defer-timing", "methods-and-receivers")
		return
	}
	l.refuse(sd, "P2G7035", "sub AUTOLOAD",
		"a catch-all method has no Go equivalent",
		"AUTOLOAD runs for any method the class does not define, with the name in "+
			"$AUTOLOAD, which is how a class generates accessors it never wrote down. "+
			"Go resolves method names when it compiles and has no hook for a name it "+
			"does not know.",
		"Write the methods out, or generate them with go:generate. Where the set of "+
			"names is genuinely open, a map from name to a function value is the "+
			"honest translation, and the caller indexes it rather than writing a "+
			"method call.",
		"methods-and-receivers", "implicit-interfaces")
}

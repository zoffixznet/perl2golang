// Package idioms is the mechanical half of the project's review checklist for
// generated Go: the tells that make output read as machine-produced, detected
// from the syntax tree alone.
//
// Each rule here is deliberately narrow. A hit is a place a Go developer would
// rewrite on sight, found by a check precise enough that a hit is nearly
// always real: a C-style loop whose index is only ever an index, a min or max
// computed with an if and an else, a variable copied to a second name that
// adds nothing. The checklist's judgement calls, the ones that need type
// information or taste, are not here, so the count this package produces is a
// floor on the clumsiness of a file, never a ceiling.
//
// The count exists to make "the output got less machine-shaped" a claim with
// a number behind it. The scorecard counts hits on the deterministic output
// and on a model-improved rewrite of the same program, and the difference is
// measured improvement that does not depend on any program failing first.
package idioms

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Hit is one place a rule matched.
type Hit struct {
	// Rule is the stable rule name, for aggregation.
	Rule string `json:"rule"`
	// Line is the line in the scanned file.
	Line int `json:"line"`
	// Detail says what was seen, in one phrase.
	Detail string `json:"detail"`
}

// The rules, named for aggregation. Each corresponds to an entry in the
// project's antipattern checklist for generated output.
const (
	// RuleNumberedName is an identifier ending in a digit: content2, item4.
	// A numbered name means one source variable was split without being
	// renamed, and it is the strongest single tell of machine output.
	RuleNumberedName = "numbered-name"
	// RuleAliasCopy is `x := y` where y is a plain variable: a second name
	// for a value that already had one.
	RuleAliasCopy = "alias-copy"
	// RuleReturnImmediately is `x := expr` directly followed by `return x`.
	RuleReturnImmediately = "return-immediately"
	// RuleBlankDiscard is `_ = x` of a plain variable: the compiler silenced
	// instead of the variable deleted.
	RuleBlankDiscard = "blank-discard"
	// RuleCStyleFor is `for i := 0; i < len(xs); i++` where i is used only as
	// xs[i], which is a range loop spelled the long way.
	RuleCStyleFor = "cstyle-for"
	// RuleLegacySort is sort.Slice, sort.Strings and friends where the slices
	// package has owned the job since Go 1.21.
	RuleLegacySort = "legacy-sort"
	// RuleMinMaxByHand is an if and an else assigning one variable the smaller
	// or larger of two values, which is the built-in min or max.
	RuleMinMaxByHand = "minmax-by-hand"
	// RuleSprintfConcat is fmt.Sprintf whose format string is nothing but %s
	// and %v verbs: concatenation wearing a costume.
	RuleSprintfConcat = "sprintf-concat"
	// RuleStringAppendLoop is `s += ...` of string material inside a loop,
	// the quadratic spelling of strings.Builder.
	RuleStringAppendLoop = "string-append-loop"
	// RuleElseAfterExit is an else whose if arm already returned, broke,
	// continued or exited: the happy path deserves the left margin.
	RuleElseAfterExit = "else-after-exit"
	// RuleStringifiedKey is a map indexed with strconv.Itoa(...): an integer
	// key stored as text because the source language's maps knew no better.
	RuleStringifiedKey = "stringified-key"
	// RuleMapStringAny is the map[string]any record: a fixed-key record that
	// should have been a struct, or a documented fallback.
	RuleMapStringAny = "map-string-any"
	// RuleInterfaceBraces is the interface{} spelling where any has been the
	// idiom since Go 1.18.
	RuleInterfaceBraces = "interface-braces"
)

// Scan parses one Go file and returns every rule hit, sorted by line.
func Scan(filename string, src []byte) ([]Hit, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("scanning %s: %w", filename, err)
	}
	s := &scanner{fset: fset}
	s.file(file)
	sort.SliceStable(s.hits, func(i, j int) bool { return s.hits[i].Line < s.hits[j].Line })
	return s.hits, nil
}

// Count scans a set of files and sums the hits per rule. Files that do not
// parse contribute nothing: a file with a syntax error has bigger problems
// than style, and the compile stages already account for it. The runtime
// support file helpers.go is skipped, because it is hand-written, unit-tested
// code copied in unchanged rather than generated output.
func Count(files map[string][]byte) (total int, byRule map[string]int) {
	byRule = map[string]int{}
	for name, src := range files {
		if !strings.HasSuffix(name, ".go") || path.Base(name) == "helpers.go" {
			continue
		}
		hits, err := Scan(name, src)
		if err != nil {
			continue
		}
		total += len(hits)
		for _, h := range hits {
			byRule[h.Rule]++
		}
	}
	return total, byRule
}

type scanner struct {
	fset *token.FileSet
	hits []Hit
}

func (s *scanner) add(rule string, pos token.Pos, detail string) {
	s.hits = append(s.hits, Hit{Rule: rule, Line: s.fset.Position(pos).Line, Detail: detail})
}

func (s *scanner) file(file *ast.File) {
	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.FuncDecl); ok {
			s.declaredName(f.Name)
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			s.assign(node)
		case *ast.BlockStmt:
			s.block(node)
		case *ast.ForStmt:
			s.forLoop(node)
			s.loopBody(node.Body)
		case *ast.RangeStmt:
			if node.Tok == token.DEFINE {
				for _, e := range []ast.Expr{node.Key, node.Value} {
					if id, ok := e.(*ast.Ident); ok {
						s.declaredName(id)
					}
				}
			}
			s.loopBody(node.Body)
		case *ast.FuncType:
			if node.Params != nil {
				for _, field := range node.Params.List {
					for _, name := range field.Names {
						s.declaredName(name)
					}
				}
			}
		case *ast.IfStmt:
			s.ifStmt(node)
		case *ast.CallExpr:
			s.call(node)
		case *ast.IndexExpr:
			s.index(node)
		case *ast.MapType:
			s.mapType(node)
		case *ast.InterfaceType:
			if len(node.Methods.List) == 0 {
				s.add(RuleInterfaceBraces, node.Pos(), "interface{} where any is the modern spelling")
			}
		case *ast.GenDecl:
			for _, spec := range node.Specs {
				switch sp := spec.(type) {
				case *ast.TypeSpec:
					s.declaredName(sp.Name)
				case *ast.ValueSpec:
					for _, name := range sp.Names {
						s.declaredName(name)
					}
				}
			}
		}
		return true
	})
}

// numbered matches an identifier that ends in digits after a letter.
var numbered = regexp.MustCompile(`[A-Za-z]\d+$`)

// numberedExceptions are real words and stdlib vocabulary that end in digits,
// which a name may legitimately echo.
var numberedExceptions = map[string]bool{
	"utf8": true, "utf16": true, "base32": true, "base64": true, "md5": true,
	"sha1": true, "sha256": true, "sha512": true, "sha3": true, "crc32": true,
	"crc64": true, "adler32": true, "x509": true, "ascii85": true,
	"int8": true, "int16": true, "int32": true, "int64": true,
	"uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true, "complex64": true, "complex128": true,
}

func (s *scanner) declaredName(id *ast.Ident) {
	if id == nil || id.Name == "_" || numberedExceptions[strings.ToLower(id.Name)] {
		return
	}
	if numbered.MatchString(id.Name) {
		s.add(RuleNumberedName, id.Pos(), id.Name+" is a numbered name, so one value was split without being renamed")
	}
}

func (s *scanner) assign(a *ast.AssignStmt) {
	if a.Tok == token.DEFINE {
		for _, l := range a.Lhs {
			if id, ok := l.(*ast.Ident); ok {
				s.declaredName(id)
			}
		}
	}
	if len(a.Lhs) != 1 || len(a.Rhs) != 1 {
		return
	}
	lhs, lhsIsIdent := a.Lhs[0].(*ast.Ident)
	rhs, rhsIsIdent := a.Rhs[0].(*ast.Ident)
	if !lhsIsIdent || !rhsIsIdent || literalIdent(rhs.Name) {
		return
	}
	switch {
	case a.Tok == token.DEFINE:
		s.add(RuleAliasCopy, a.Pos(), lhs.Name+" := "+rhs.Name+" gives a second name to a value that already has one")
	case a.Tok == token.ASSIGN && lhs.Name == "_":
		s.add(RuleBlankDiscard, a.Pos(), "_ = "+rhs.Name+" silences a variable instead of deleting it")
	}
}

// literalIdent reports whether a name is a predeclared value rather than a
// variable, so `x := true` is not called an alias of anything.
func literalIdent(name string) bool {
	switch name {
	case "true", "false", "nil", "iota":
		return true
	}
	return false
}

// block looks for `x := expr` immediately followed by `return x`.
func (s *scanner) block(b *ast.BlockStmt) {
	for i := 0; i+1 < len(b.List); i++ {
		def, ok := b.List[i].(*ast.AssignStmt)
		if !ok || def.Tok != token.DEFINE || len(def.Lhs) != 1 {
			continue
		}
		name, ok := def.Lhs[0].(*ast.Ident)
		if !ok {
			continue
		}
		ret, ok := b.List[i+1].(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}
		if res, ok := ret.Results[0].(*ast.Ident); ok && res.Name == name.Name {
			s.add(RuleReturnImmediately, def.Pos(), name.Name+" is declared only to be returned on the next line")
		}
	}
}

// forLoop recognises `for i := 0; i < len(xs); i++` whose body uses i only as
// xs[i].
func (s *scanner) forLoop(f *ast.ForStmt) {
	if f.Init == nil || f.Cond == nil || f.Post == nil {
		return
	}
	init, ok := f.Init.(*ast.AssignStmt)
	if !ok || init.Tok != token.DEFINE || len(init.Lhs) != 1 || len(init.Rhs) != 1 {
		return
	}
	idx, ok := init.Lhs[0].(*ast.Ident)
	if !ok || !isZero(init.Rhs[0]) {
		return
	}
	cond, ok := f.Cond.(*ast.BinaryExpr)
	if !ok || cond.Op != token.LSS {
		return
	}
	condIdx, ok := cond.X.(*ast.Ident)
	if !ok || condIdx.Name != idx.Name {
		return
	}
	lenCall, ok := cond.Y.(*ast.CallExpr)
	if !ok || len(lenCall.Args) != 1 {
		return
	}
	if fun, ok := lenCall.Fun.(*ast.Ident); !ok || fun.Name != "len" {
		return
	}
	subject, ok := lenCall.Args[0].(*ast.Ident)
	if !ok {
		return
	}
	post, ok := f.Post.(*ast.IncDecStmt)
	if !ok || post.Tok != token.INC {
		return
	}
	if postIdx, ok := post.X.(*ast.Ident); !ok || postIdx.Name != idx.Name {
		return
	}

	// Every use of the index inside the body must be exactly subject[i], and
	// the body must not write to the index or the subject.
	onlyIndexes := true
	var uses, indexed int
	ast.Inspect(f.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.IndexExpr:
			base, okBase := node.X.(*ast.Ident)
			i, okIdx := node.Index.(*ast.Ident)
			if okBase && okIdx && base.Name == subject.Name && i.Name == idx.Name {
				// Counted here as one indexed use; returning false keeps the
				// walk out of the children, so the ident case below never
				// sees this occurrence of the index.
				indexed++
				uses++
				return false
			}
		case *ast.Ident:
			if node.Name == idx.Name {
				uses++
			}
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && (id.Name == idx.Name || id.Name == subject.Name) {
					onlyIndexes = false
				}
			}
		case *ast.IncDecStmt:
			if id, ok := node.X.(*ast.Ident); ok && id.Name == idx.Name {
				onlyIndexes = false
			}
		}
		return true
	})
	if onlyIndexes && indexed > 0 && uses == indexed {
		s.add(RuleCStyleFor, f.Pos(), "the index of this loop is only ever "+subject.Name+"["+idx.Name+"], which is a range loop spelled the long way")
	}
}

func isZero(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	return ok && lit.Kind == token.INT && lit.Value == "0"
}

// loopBody looks for string building with += inside a loop.
func (s *scanner) loopBody(body *ast.BlockStmt) {
	for _, stmt := range body.List {
		a, ok := stmt.(*ast.AssignStmt)
		if !ok || a.Tok != token.ADD_ASSIGN || len(a.Rhs) != 1 {
			continue
		}
		if containsStringMaterial(a.Rhs[0]) {
			s.add(RuleStringAppendLoop, a.Pos(), "building a string with += in a loop is quadratic; strings.Builder is the idiom")
		}
	}
}

// containsStringMaterial reports whether an expression visibly produces string
// content: a string literal, fmt.Sprintf, or a string(...) conversion.
func containsStringMaterial(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.BasicLit:
			if node.Kind == token.STRING {
				found = true
			}
		case *ast.CallExpr:
			if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "fmt" && strings.HasPrefix(sel.Sel.Name, "Sprint") {
					found = true
				}
			}
			if id, ok := node.Fun.(*ast.Ident); ok && id.Name == "string" {
				found = true
			}
		}
		return !found
	})
	return found
}

func (s *scanner) ifStmt(f *ast.IfStmt) {
	s.minMax(f)
	if f.Else == nil {
		return
	}
	if _, chained := f.Else.(*ast.IfStmt); chained {
		return
	}
	if len(f.Body.List) == 0 {
		return
	}
	if terminates(f.Body.List[len(f.Body.List)-1]) {
		s.add(RuleElseAfterExit, f.Else.Pos(), "the if arm already leaves, so the else only adds indentation")
	}
}

// terminates reports whether a statement unconditionally leaves the
// surrounding flow: return, break, continue, goto, panic, or a process exit.
func terminates(stmt ast.Stmt) bool {
	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return s.Tok == token.BREAK || s.Tok == token.CONTINUE || s.Tok == token.GOTO
	case *ast.ExprStmt:
		call, ok := s.X.(*ast.CallExpr)
		if !ok {
			return false
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			return fun.Name == "panic"
		case *ast.SelectorExpr:
			pkg, ok := fun.X.(*ast.Ident)
			if !ok {
				return false
			}
			return (pkg.Name == "os" && fun.Sel.Name == "Exit") ||
				(pkg.Name == "log" && strings.HasPrefix(fun.Sel.Name, "Fatal"))
		}
	}
	return false
}

// minMax recognises an if and an else that assign one variable the smaller or
// larger of the two compared values, which is the built-in min or max.
func (s *scanner) minMax(f *ast.IfStmt) {
	if f.Init != nil || f.Else == nil {
		return
	}
	elseBlock, ok := f.Else.(*ast.BlockStmt)
	if !ok {
		return
	}
	cond, ok := f.Cond.(*ast.BinaryExpr)
	if !ok {
		return
	}
	switch cond.Op {
	case token.LSS, token.GTR, token.LEQ, token.GEQ:
	default:
		return
	}
	thenName, thenVal, ok := singleAssign(f.Body)
	if !ok {
		return
	}
	elseName, elseVal, ok := singleAssign(elseBlock)
	if !ok || thenName != elseName {
		return
	}
	condX, condY := exprText(cond.X), exprText(cond.Y)
	straight := thenVal == condX && elseVal == condY
	crossed := thenVal == condY && elseVal == condX
	if straight || crossed {
		s.add(RuleMinMaxByHand, f.Pos(), thenName+" is the smaller or larger of two values, which is the built-in min or max")
	}
}

// singleAssign returns the target name and value text when a block is exactly
// one plain assignment to one identifier.
func singleAssign(b *ast.BlockStmt) (name, value string, ok bool) {
	if len(b.List) != 1 {
		return "", "", false
	}
	a, isAssign := b.List[0].(*ast.AssignStmt)
	if !isAssign || len(a.Lhs) != 1 || len(a.Rhs) != 1 {
		return "", "", false
	}
	if a.Tok != token.ASSIGN && a.Tok != token.DEFINE {
		return "", "", false
	}
	id, isIdent := a.Lhs[0].(*ast.Ident)
	if !isIdent {
		return "", "", false
	}
	return id.Name, exprText(a.Rhs[0]), true
}

// exprText renders an expression as comparable text, whitespace normalised.
func exprText(e ast.Expr) string {
	var b strings.Builder
	writeExpr(&b, e)
	return b.String()
}

// writeExpr prints the small expression grammar the min and max detector needs
// to compare. Anything it does not know renders as a unique placeholder, which
// can never equal another expression, so an unknown form fails safe.
func writeExpr(b *strings.Builder, e ast.Expr) {
	switch node := e.(type) {
	case *ast.Ident:
		b.WriteString(node.Name)
	case *ast.BasicLit:
		b.WriteString(node.Value)
	case *ast.SelectorExpr:
		writeExpr(b, node.X)
		b.WriteString(".")
		b.WriteString(node.Sel.Name)
	case *ast.BinaryExpr:
		b.WriteString("(")
		writeExpr(b, node.X)
		b.WriteString(node.Op.String())
		writeExpr(b, node.Y)
		b.WriteString(")")
	case *ast.ParenExpr:
		writeExpr(b, node.X)
	case *ast.CallExpr:
		writeExpr(b, node.Fun)
		b.WriteString("(")
		for i, arg := range node.Args {
			if i > 0 {
				b.WriteString(",")
			}
			writeExpr(b, arg)
		}
		b.WriteString(")")
	case *ast.IndexExpr:
		writeExpr(b, node.X)
		b.WriteString("[")
		writeExpr(b, node.Index)
		b.WriteString("]")
	case *ast.UnaryExpr:
		b.WriteString(node.Op.String())
		writeExpr(b, node.X)
	default:
		fmt.Fprintf(b, "<%p>", e)
	}
}

func (s *scanner) call(c *ast.CallExpr) {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	if pkg.Name == "sort" {
		switch sel.Sel.Name {
		case "Slice", "SliceStable", "Strings", "Ints", "Float64s":
			s.add(RuleLegacySort, c.Pos(), "sort."+sel.Sel.Name+" predates the slices package, which has owned this job since Go 1.21")
		}
	}
	if pkg.Name == "fmt" && sel.Sel.Name == "Sprintf" && len(c.Args) > 0 {
		if lit, ok := c.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING && verbOnlyFormat(lit.Value) {
			s.add(RuleSprintfConcat, c.Pos(), "Sprintf of "+lit.Value+" is concatenation wearing a costume")
		}
	}
}

// verbOnlyFormat reports whether a quoted format string consists only of %s
// and %v verbs, so the call adds nothing over the values themselves.
func verbOnlyFormat(quoted string) bool {
	body := strings.Trim(quoted, "`\"")
	if body == "" {
		return false
	}
	for len(body) > 0 {
		if !strings.HasPrefix(body, "%s") && !strings.HasPrefix(body, "%v") {
			return false
		}
		body = body[2:]
	}
	return true
}

func (s *scanner) index(ix *ast.IndexExpr) {
	call, ok := ix.Index.(*ast.CallExpr)
	if !ok {
		return
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "strconv" {
		return
	}
	if sel.Sel.Name == "Itoa" || sel.Sel.Name == "FormatInt" {
		s.add(RuleStringifiedKey, ix.Pos(), "a number stored as a string key; the map's key type should be the number")
	}
}

func (s *scanner) mapType(m *ast.MapType) {
	key, ok := m.Key.(*ast.Ident)
	if !ok || key.Name != "string" {
		return
	}
	switch v := m.Value.(type) {
	case *ast.Ident:
		if v.Name == "any" {
			s.add(RuleMapStringAny, m.Pos(), "map[string]any where the key set may be knowable")
		}
	case *ast.InterfaceType:
		if len(v.Methods.List) == 0 {
			s.add(RuleMapStringAny, m.Pos(), "map[string]interface{} where the key set may be knowable")
		}
	}
}

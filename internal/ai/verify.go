package ai

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/scanner"
	"go/token"
	"maps"
	"slices"
	"sort"
	"strings"
)

// VerifyGo reports whether src is syntactically valid Go, naming the first
// offending line when it is not. Every Go string a model produces passes
// through this before a caller is allowed to see it.
func VerifyGo(src string) error {
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, "candidate.go", src, parser.ParseComments)
	if err == nil {
		return nil
	}
	return parseErrorFrom(err)
}

// VerifyGoFragment reports whether src is a valid Go fragment: a sequence of
// statements as they would appear inside a function, or a set of declarations.
// It is for code sketches that are shown to a reader rather than compiled.
func VerifyGoFragment(src string) error {
	wrapped := "package p\n\nfunc _fragment() {\n" + src + "\n}\n"
	if VerifyGo(wrapped) == nil {
		return nil
	}
	declForm := "package p\n\n" + src + "\n"
	if err := VerifyGo(declForm); err != nil {
		return err
	}
	return nil
}

// parseErrorFrom turns a parser failure into a [ParseError] naming a line.
func parseErrorFrom(err error) error {
	var list scanner.ErrorList
	if ok := asErrorList(err, &list); ok && len(list) > 0 {
		first := list[0]
		return &ParseError{
			Line:   first.Pos.Line,
			Column: first.Pos.Column,
			Detail: first.Msg,
			Err:    err,
		}
	}
	return &ParseError{Detail: err.Error(), Err: err}
}

func asErrorList(err error, out *scanner.ErrorList) bool {
	if list, ok := err.(scanner.ErrorList); ok {
		*out = list
		return true
	}
	return false
}

// gofmt formats src, reporting a rejection when it cannot.
func gofmt(src string) (string, error) {
	out, err := format.Source([]byte(src))
	if err != nil {
		return "", &RejectedError{Gate: "format", Reason: "the result could not be formatted", Err: err}
	}
	return string(out), nil
}

// fileFacts is everything the structural checks compare between the
// deterministic baseline and a candidate.
type fileFacts struct {
	decls      []string
	exported   []string
	imports    map[string]bool // import path -> present
	importName map[string]bool // the identifier each import binds
	literals   []string
	// strings and numbers are the distinct literal values outside the import
	// declaration, split by what they carry: string content is what a program
	// prints, numeric content is how it steers. The idiom gate holds the two
	// to different rules, so they are collected apart.
	strings    map[string]bool
	numbers    map[string]bool
	qualifiers map[string]bool
	declared   map[string]bool
	banned     map[string]int
	errChecks  int
	directives []string
	// sortCalls, tiebreaks and printlns count the calls whose disappearance
	// or appearance changes what a program prints without touching one
	// literal: the sort family, cmp.Or in a comparator, and fmt.Println.
	sortCalls int
	tiebreaks int
	printlns  int
}

// factsOf parses src and collects the facts the invariant checks need.
func factsOf(src string) (*fileFacts, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "candidate.go", src, parser.ParseComments)
	if err != nil {
		return nil, parseErrorFrom(err)
	}
	f := &fileFacts{
		imports:    map[string]bool{},
		importName: map[string]bool{},
		strings:    map[string]bool{},
		numbers:    map[string]bool{},
		qualifiers: map[string]bool{},
		declared:   map[string]bool{},
		banned:     map[string]int{},
	}

	importPaths := map[token.Pos]bool{}
	for _, imp := range file.Imports {
		importPaths[imp.Path.Pos()] = true
		path := strings.Trim(imp.Path.Value, `"`)
		f.imports[path] = true
		name := path
		if i := strings.LastIndex(path, "/"); i >= 0 {
			name = path[i+1:]
		}
		if imp.Name != nil {
			name = imp.Name.Name
		}
		f.importName[name] = true
	}

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			f.decls = append(f.decls, funcSignature(fset, d))
			f.declared[d.Name.Name] = true
			if d.Name.IsExported() {
				f.exported = append(f.exported, funcSignature(fset, d))
			}
			if d.Name.Name == "init" && d.Recv == nil {
				f.banned["init"]++
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					sig := "type " + s.Name.Name + " " + render(fset, s.Type)
					f.decls = append(f.decls, sig)
					f.declared[s.Name.Name] = true
					if s.Name.IsExported() {
						f.exported = append(f.exported, sig)
					}
				case *ast.ValueSpec:
					for _, name := range s.Names {
						sig := d.Tok.String() + " " + name.Name
						if s.Type != nil {
							sig += " " + render(fset, s.Type)
						}
						f.decls = append(f.decls, sig)
						f.declared[name.Name] = true
						if name.IsExported() {
							f.exported = append(f.exported, sig)
						}
					}
				}
			}
		}
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.BasicLit:
			f.literals = append(f.literals, node.Kind.String()+" "+node.Value)
			if !importPaths[node.Pos()] {
				if node.Kind == token.STRING {
					f.strings[node.Value] = true
				} else {
					f.numbers[node.Value] = true
				}
			}
		case *ast.SelectorExpr:
			if id, ok := node.X.(*ast.Ident); ok {
				f.qualifiers[id.Name] = true
				if bannedSelector(id.Name, node.Sel.Name) {
					f.banned[id.Name+"."+node.Sel.Name]++
				}
				switch {
				case sortFamily(id.Name, node.Sel.Name):
					f.sortCalls++
				case id.Name == "cmp" && node.Sel.Name == "Or":
					f.tiebreaks++
				case id.Name == "fmt" && node.Sel.Name == "Println":
					f.printlns++
				}
			}
		case *ast.Ident:
			if node.Name == "panic" || node.Name == "recover" {
				f.banned[node.Name]++
			}
		case *ast.GoStmt:
			f.banned["go statement"]++
		case *ast.DeferStmt:
			f.banned["defer"]++
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				if id, ok := lhs.(*ast.Ident); ok {
					f.declared[id.Name] = true
					if id.Name == "_" {
						f.banned["blank assignment"]++
					}
				}
			}
		case *ast.Field:
			for _, name := range node.Names {
				f.declared[name.Name] = true
			}
		case *ast.IfStmt:
			if isErrCheck(node.Cond) {
				f.errChecks++
			}
		}
		return true
	})

	for _, group := range file.Comments {
		for _, c := range group.List {
			text := c.Text
			if strings.HasPrefix(text, "//go:") || strings.HasPrefix(text, "// +build") {
				f.directives = append(f.directives, text)
			}
		}
	}

	sort.Strings(f.decls)
	sort.Strings(f.exported)
	sort.Strings(f.literals)
	sort.Strings(f.directives)
	return f, nil
}

// sortFamily reports whether pkg.sel is one of the calls that put data in a
// deliberate order. The family is counted as a whole, so replacing
// sort.Slice with slices.SortFunc is fine and deleting either is not.
func sortFamily(pkg, sel string) bool {
	switch pkg {
	case "sort":
		switch sel {
		case "Slice", "SliceStable", "Strings", "Ints", "Float64s", "Sort", "Stable":
			return true
		}
	case "slices":
		switch sel {
		case "Sort", "SortFunc", "SortStableFunc", "Sorted", "SortedFunc", "SortedStableFunc":
			return true
		}
	}
	return false
}

// bannedSelector reports whether pkg.sel is one of the calls a mechanical
// rewrite may never introduce: they change how the program ends, or reach
// outside the type system.
func bannedSelector(pkg, sel string) bool {
	switch pkg {
	case "unsafe", "reflect":
		return true
	case "os":
		return sel == "Exit"
	case "log":
		return strings.HasPrefix(sel, "Fatal") || strings.HasPrefix(sel, "Panic")
	case "syscall":
		return sel == "Exit"
	}
	return false
}

// isErrCheck recognises the `if err != nil` shape, whatever the variable is
// called, so a rewrite cannot quietly drop an error check.
func isErrCheck(cond ast.Expr) bool {
	bin, ok := cond.(*ast.BinaryExpr)
	if !ok || bin.Op != token.NEQ {
		return false
	}
	lhs, ok := bin.X.(*ast.Ident)
	if !ok {
		return false
	}
	rhs, ok := bin.Y.(*ast.Ident)
	if !ok || rhs.Name != "nil" {
		return false
	}
	name := strings.ToLower(lhs.Name)
	return name == "err" || strings.HasSuffix(name, "err") || strings.HasSuffix(name, "error")
}

func funcSignature(fset *token.FileSet, d *ast.FuncDecl) string {
	var sb strings.Builder
	sb.WriteString("func ")
	if d.Recv != nil && len(d.Recv.List) > 0 {
		sb.WriteString("(" + render(fset, d.Recv.List[0].Type) + ") ")
	}
	sb.WriteString(d.Name.Name)
	sb.WriteString(render(fset, d.Type))
	return strings.TrimPrefix(sb.String(), "")
}

func render(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	cfg := printer.Config{Mode: printer.RawFormat}
	if err := cfg.Fprint(&buf, fset, node); err != nil {
		return fmt.Sprintf("<unprintable %T>", node)
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}

// checkIdiomRewrite proves that a candidate file differs from the baseline
// only in ways an idiom rewrite is allowed to differ.
//
// The declaration set, the exported surface, the compiler directives, the
// error checks and the set of ways the program can end must all survive
// unchanged. What an idiom rewrite may do that a rename may not is reshape
// expressions: replacing a hand-rolled loop with the standard library call
// that does the same thing necessarily adds an import and drops the loop's
// steering numbers, and forbidding either forbade the whole class of fixes
// this job exists for. So imports may grow, by exactly the declared standard
// library paths the new code reaches for; a numeric literal may disappear,
// but no numeric value absent from the baseline may appear, because an
// invented index or size is how a rewrite corrupts quietly; and strings are
// held hardest of all, because string content is what a program prints: no
// string value may be added or changed, and the only string that may
// disappear entirely is a format string of nothing but %s and %v verbs,
// which carries no text of its own.
func checkIdiomRewrite(baseline, candidate string, newImports []string) error {
	base, err := factsOf(baseline)
	if err != nil {
		return &RejectedError{Gate: "baseline", Reason: "the deterministic file does not parse", Err: err}
	}
	cand, err := factsOf(candidate)
	if err != nil {
		return &RejectedError{Gate: "parse", Reason: "the result does not parse", Err: err}
	}

	if !slices.Equal(base.decls, cand.decls) {
		return &RejectedError{Gate: "declarations", Reason: declDelta(base.decls, cand.decls)}
	}
	if !slices.Equal(base.exported, cand.exported) {
		return &RejectedError{Gate: "exported surface", Reason: "the set of exported declarations changed"}
	}
	for v := range cand.strings {
		if !base.strings[v] {
			return &RejectedError{Gate: "literals", Reason: fmt.Sprintf("the string %s is not in the deterministic file, and string content is what the program prints", short(v))}
		}
	}
	for v := range base.strings {
		if !cand.strings[v] && !verbOnlyFormat(v) {
			return &RejectedError{Gate: "literals", Reason: fmt.Sprintf("the string %s disappeared, and string content is what the program prints", short(v))}
		}
	}
	for v := range cand.numbers {
		if !base.numbers[v] {
			return &RejectedError{Gate: "literals", Reason: fmt.Sprintf("the number %s is not in the deterministic file, and an invented number is how an index goes wrong", short(v))}
		}
	}
	for path := range base.imports {
		if !cand.imports[path] {
			return &RejectedError{Gate: "imports", Reason: fmt.Sprintf("the import %q was dropped", path)}
		}
	}
	for path := range cand.imports {
		if base.imports[path] || containsString(newImports, path) {
			continue
		}
		return &RejectedError{Gate: "imports", Reason: fmt.Sprintf("the import %q is neither the converter's nor one the finding declared", path)}
	}
	for name := range cand.qualifiers {
		if base.qualifiers[name] || cand.importName[name] || cand.declared[name] || name == "_" {
			continue
		}
		return &RejectedError{Gate: "grounding", Reason: fmt.Sprintf("%s.… is neither imported nor declared in this file", name)}
	}
	for kind, count := range cand.banned {
		if count > base.banned[kind] {
			return &RejectedError{Gate: "control flow", Reason: fmt.Sprintf("the result introduces %s, which a rewrite of converted code may not add", kind)}
		}
	}
	if cand.errChecks < base.errChecks {
		return &RejectedError{Gate: "error handling", Reason: fmt.Sprintf("%d error checks in the deterministic file became %d", base.errChecks, cand.errChecks)}
	}
	if !slices.Equal(base.directives, cand.directives) {
		return &RejectedError{Gate: "directives", Reason: "the compiler directives or build tags changed"}
	}
	if err := checkBehaviourTells(base, cand); err != nil {
		return err
	}
	return nil
}

// checkBehaviourTells refuses the rewrite shapes that measurably corrupt
// programs while passing every structural check, taken straight from the
// corpus record of what the model actually did:
//
//   - deleting a sort or a tiebreak, which turns deterministic output into
//     map-order output while every literal survives;
//   - rewriting `for _, x := range xs` as `for x := range xs`, which quietly
//     swaps a list's values for its indices;
//   - turning fmt.Print into fmt.Println, whose extra newline and different
//     spacing rules change what the program prints.
func checkBehaviourTells(base, cand *fileFacts) error {
	if cand.sortCalls < base.sortCalls {
		return &RejectedError{Gate: "determinism", Reason: "the rewrite removes a sort, and iteration order is part of what the program prints"}
	}
	if cand.tiebreaks < base.tiebreaks {
		return &RejectedError{Gate: "determinism", Reason: "the rewrite removes a comparator's tiebreak, and tie order is part of what the program prints"}
	}
	if cand.printlns > base.printlns {
		return &RejectedError{Gate: "output", Reason: "the rewrite introduces fmt.Println, whose newline and spacing rules differ from the fmt.Print the converter wrote"}
	}
	return nil
}

// checkRangeMisread refuses a finding that rewrites `for _, x := range xs` as
// `for x := range xs`: the same identifier now holds indices instead of
// values, no literal or declaration notices, and the corpus caught the model
// doing exactly this four times in one run.
func checkRangeMisread(oldCode, newCode string) error {
	oldRanges := rangeForms(oldCode)
	if len(oldRanges) == 0 {
		return nil
	}
	for _, nr := range rangeForms(newCode) {
		if nr.keyOnly {
			for _, or := range oldRanges {
				if !or.keyOnly && or.name == nr.name && or.subject == nr.subject {
					return &RejectedError{Gate: "range", Reason: fmt.Sprintf(
						"the rewrite turns `for _, %s := range %s` into `for %s := range %s`, which swaps the values for their indices", or.name, or.subject, nr.name, nr.subject)}
				}
			}
		}
	}
	return nil
}

// rangeForm is one range statement in a code fragment: the loop variable that
// matters, whether it is the key alone, and the ranged expression's text.
type rangeForm struct {
	name    string
	keyOnly bool
	subject string
}

// rangeForms parses a code fragment and lists its range statements. A quoted
// fragment often opens a block it does not close, so missing closing braces
// are appended before parsing; a fragment that still does not parse
// contributes nothing, and the parse gates have their own say about it.
func rangeForms(fragment string) []rangeForm {
	depth := 0
	for _, r := range fragment {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
		}
	}
	for ; depth > 0; depth-- {
		fragment += "\n}"
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fragment.go", "package p\n\nfunc _fragment() {\n"+fragment+"\n}\n", 0)
	if err != nil {
		return nil
	}
	var out []rangeForm
	ast.Inspect(file, func(n ast.Node) bool {
		r, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}
		key, isIdent := r.Key.(*ast.Ident)
		if !isIdent {
			return true
		}
		f := rangeForm{subject: render(fset, r.X)}
		if r.Value == nil {
			f.name, f.keyOnly = key.Name, true
		} else if val, ok := r.Value.(*ast.Ident); ok && key.Name == "_" {
			f.name = val.Name
		} else {
			return true
		}
		if f.name != "" && f.name != "_" {
			out = append(out, f)
		}
		return true
	})
	return out
}

// verbOnlyFormat reports whether a quoted string literal is a format string
// consisting only of %s and %v verbs. Such a string carries no text of its
// own, so a rewrite that folds the formatting away may drop it.
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

func declDelta(base, cand []string) string {
	missing := missingFrom(base, cand)
	added := missingFrom(cand, base)
	switch {
	case len(missing) > 0 && len(added) > 0:
		return fmt.Sprintf("the declaration %s was replaced by %s", short(missing[0]), short(added[0]))
	case len(missing) > 0:
		return fmt.Sprintf("the declaration %s disappeared", short(missing[0]))
	case len(added) > 0:
		return fmt.Sprintf("the declaration %s was added", short(added[0]))
	}
	return "the set of declarations changed"
}

func literalDelta(base, cand []string) string {
	missing := missingFrom(base, cand)
	added := missingFrom(cand, base)
	switch {
	case len(missing) > 0 && len(added) > 0:
		return fmt.Sprintf("the literal %s became %s, and literals carry the program's output", short(litText(missing[0])), short(litText(added[0])))
	case len(missing) > 0:
		return fmt.Sprintf("the literal %s disappeared, and literals carry the program's output", short(litText(missing[0])))
	default:
		return fmt.Sprintf("the literal %s was added, and literals carry the program's output", short(litText(added[0])))
	}
}

func litText(s string) string {
	if i := strings.Index(s, " "); i >= 0 {
		return s[i+1:]
	}
	return s
}

// missingFrom returns the members of want that need are missing, counting
// duplicates.
func missingFrom(want, have []string) []string {
	counts := map[string]int{}
	for _, s := range have {
		counts[s]++
	}
	var out []string
	for _, s := range want {
		if counts[s] > 0 {
			counts[s]--
			continue
		}
		out = append(out, s)
	}
	return out
}

func short(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}

// commentsStripped returns src with every comment removed and the result
// formatted, so two files can be compared for "comments only" changes.
func commentsStripped(src string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "candidate.go", src, parser.ParseComments)
	if err != nil {
		return "", parseErrorFrom(err)
	}
	file.Comments = nil
	ast.Inspect(file, func(n ast.Node) bool {
		switch d := n.(type) {
		case *ast.FuncDecl:
			d.Doc = nil
		case *ast.GenDecl:
			d.Doc = nil
		case *ast.Field:
			d.Doc, d.Comment = nil, nil
		case *ast.TypeSpec:
			d.Doc, d.Comment = nil, nil
		case *ast.ValueSpec:
			d.Doc, d.Comment = nil, nil
		}
		return true
	})
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, file); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// stdPackages is the set of standard library package names, used to tell a
// package qualifier apart from a hallucination. It names packages only: proving
// that a symbol inside one exists needs a type checker and a build of the
// standard library, which this package deliberately does not require.
var stdPackages = func() map[string]bool {
	names := strings.Fields(`
adler32 aes ascii85 asn1 ast atomic base32 base64 big binary bits bufio build buildinfo bytes bzip2
cgi cgo cipher cmp cmplx color comment constant constraint context cookiejar coverage crc32 crc64
crypto csv debug des doc draw driver dsa dwarf ecdh ecdsa ed25519 elf elliptic embed encoding errors
exec expvar fcgi filepath fips140 flag flate fmt fnv format fs fstest gif gob gosym gzip hash heap
hex hkdf hmac hpke html http httptest httptrace httputil image importer io iotest ioutil iter jpeg
json jsonrpc list log lzw macho mail maphash maps math md5 metrics mime mlkem multipart net netip os
palette parse parser path pbkdf2 pe pem pkix plan9obj plugin png pprof printer quick quotedprintable
race rand rc4 reflect regexp ring rpc rsa runtime scanner sha1 sha256 sha3 sha512 signal slices slog
smtp sort sql strconv strings structs subtle suffixarray sync syntax syscall syslog tabwriter tar
template testing textproto time tls token trace types tzdata unicode unique unsafe url user utf16
utf8 version weak x509 xml zip zlib`)
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}()

// StdPackage reports whether name is the name of a standard library package.
func StdPackage(name string) bool { return stdPackages[name] }

// stdPackageNames returns the sorted set, for messages.
func stdPackageNames() []string {
	return slices.Sorted(maps.Keys(stdPackages))
}

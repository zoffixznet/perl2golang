package lower

import (
	"strconv"
	"strings"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file translates Perl's regular expressions into Go's.
//
// Go's regexp package implements RE2, which guarantees linear-time matching by
// giving up the features that need backtracking: no backreferences, no
// lookahead, no lookbehind. That is a real trade, and it is one of the few
// places where a Perl program genuinely cannot be carried across unchanged.
// Every such pattern is refused with the specific feature named, because a
// pattern that silently matches something else is the worst possible outcome.

// patternOf returns an expression naming the compiled pattern for a regex
// literal, hoisting it to package level when it is constant.
func (l *Lowerer) patternOf(e ast.Expr) (ir.Expr, bool) {
	var rx *ast.Regex
	switch n := e.(type) {
	case *ast.Regex:
		rx = n
	case *ast.Match:
		rx = n.Pattern
	case *ast.QrExpr:
		rx = n.Pattern
	case *ast.StrLit:
		rx = &ast.Regex{Raw: regexpQuoteMeta(n.Value)}
	case *ast.Var:
		// `my $re = qr/.../; $s =~ $re` holds the compiled pattern in a
		// variable, which is what a *regexp.Regexp is for.
		if n.Sigil == '$' {
			x := l.expr(n)
			if t := typeOrAny(x); t.Kind == ir.Named && t.Name == "*regexp.Regexp" {
				return x, true
			}
			// A scalar holding pattern text is a pattern too: Perl reads the
			// string as one wherever a pattern is wanted, so it is compiled
			// here rather than treated as text to match literally.
			out := l.runtimePattern(l.toStr(x, n), n)
			l.note(out, "The pattern is in a variable as text, so it has to be compiled "+
				"before it can be used. qr// in the original would have compiled it once, "+
				"and a *regexp.Regexp kept in a variable is the Go equivalent of that.",
				"mustcompile-pattern")
			return out, true
		}
		return nil, false
	default:
		return nil, false
	}
	if rx == nil {
		return nil, false
	}
	return l.compiledPattern(rx, e)
}

// compiledPattern turns a pattern into a Go *regexp.Regexp expression.
func (l *Lowerer) compiledPattern(rx *ast.Regex, at ast.Node) (ir.Expr, bool) {
	if interpolated(rx) {
		// The parts around the variables are still pattern text, and a
		// construct RE2 does not have is no more expressible for being next
		// to a variable. Checking them here is what stops `(?{ $n++ })` from
		// being read as an interpolation and pasted into a pattern string,
		// which produces something that is not a pattern at all.
		if feature, bad := unsupportedFeature(literalParts(rx)); bad {
			todo := l.refuse(at, feature.code, feature.name,
				feature.short, feature.message, feature.advice, "regexp-is-re2")
			l.patternTodo = &todo
			return nil, false
		}
		// The pattern is only known at run time, so it is compiled where it is
		// used rather than once at start-up.
		return l.runtimePattern(l.patternText(rx, at), at), true
	}

	goPattern, ok := l.translatePattern(rx, at)
	if !ok {
		return nil, false
	}
	return l.hoistedPattern(goPattern, rx.Raw, at), true
}

// hoistedPattern names one translated pattern at package level, reusing the
// declaration when the same pattern was hoisted before.
func (l *Lowerer) hoistedPattern(goPattern, perlRaw string, at ast.Node) ir.Expr {
	if p, seen := l.patterns[goPattern]; seen {
		return ir.NewIdent(p.Name, ir.NamedType("*regexp.Regexp", "regexp"))
	}
	name := l.names.take(patternName(perlRaw, len(l.patternOrd)))
	p := &patternVar{Name: name, GoRegex: goPattern, Perl: perlRaw, Line: posLine(at)}
	l.patterns[goPattern] = p
	l.patternOrd = append(l.patternOrd, goPattern)
	return ir.NewIdent(name, ir.NamedType("*regexp.Regexp", "regexp"))
}

// splitTrailingLook takes apart a pattern of the shape MAIN(?=LOOK) or
// MAIN(?!LOOK): a lookaround as the very last thing in the pattern, at the
// top nesting level, with something before it to match. That shape has a
// faithful two-pattern translation; a lookaround anywhere else does not.
func splitTrailingLook(raw string) (main, look string, negate, ok bool) {
	if len(raw) < 5 || raw[len(raw)-1] != ')' {
		return "", "", false, false
	}
	depth := 0
	inClass := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case c == '\\':
			i++
		case inClass:
			if c == ']' {
				inClass = false
			}
		case c == '[':
			inClass = true
		case c == '(':
			if depth == 0 && i > 0 &&
				(strings.HasPrefix(raw[i:], "(?=") || strings.HasPrefix(raw[i:], "(?!")) &&
				closingParen(raw, i) == len(raw)-1 {
				return raw[:i], raw[i+3 : len(raw)-1], raw[i+2] == '!', true
			}
			depth++
		case c == ')':
			depth--
		}
	}
	return "", "", false, false
}

// closingParen finds the index of the parenthesis closing the group that
// opens at index open, or -1 when the pattern is unbalanced.
func closingParen(raw string, open int) int {
	depth := 0
	inClass := false
	for i := open; i < len(raw); i++ {
		c := raw[i]
		switch {
		case c == '\\':
			i++
		case inClass:
			if c == ']' {
				inClass = false
			}
		case c == '[':
			inClass = true
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// lookAheadPair translates a pattern whose one backtracking feature is a
// trailing lookaround into its two halves, hoisted as ordinary patterns:
// the part that matches, and the assertion tested just after it, anchored
// so it can be run against the tail of the text.
func (l *Lowerer) lookAheadPair(rx *ast.Regex, at ast.Node) (mainPat, lookPat ir.Expr, negate, ok bool) {
	if rx == nil || interpolated(rx) {
		return nil, nil, false, false
	}
	mainRaw, lookRaw, neg, split := splitTrailingLook(rx.Raw)
	if !split {
		return nil, nil, false, false
	}
	// Either half needing a feature of its own that the engine lacks means
	// the whole pattern is refused in one piece, with that feature named.
	if _, bad := unsupportedFeature(mainRaw); bad {
		return nil, nil, false, false
	}
	if _, bad := unsupportedFeature(lookRaw); bad {
		return nil, nil, false, false
	}
	mainGo, okMain := l.translatePattern(&ast.Regex{Raw: mainRaw, Mods: rx.Mods}, at)
	if !okMain {
		return nil, nil, false, false
	}
	lookGo, okLook := l.translatePattern(&ast.Regex{Raw: lookRaw, Mods: strings.ReplaceAll(rx.Mods, "g", "")}, at)
	if !okLook {
		return nil, nil, false, false
	}
	mainPat = l.hoistedPattern(mainGo, mainRaw, at)
	lookPat = l.hoistedPattern(`\A(?:`+lookGo+`)`, "(?="+lookRaw+")", at)
	l.approximate(at, "P2G4004", "a trailing lookaround",
		"the assertion is tested after the match, by a scan",
		"A lookaround asserts what follows without consuming it, and Go's regexp "+
			"traded that feature for a guarantee that matching takes linear time. A "+
			"lookaround at the end of a pattern has a faithful two-pattern form: "+
			"match the first, test the second against what follows, and move on by "+
			"one when the test fails. One difference remains: a backtracking engine "+
			"can shrink the match itself until the assertion holds, and this scan "+
			"never retries a match shorter. The two agree whenever what the match "+
			"consumes and what the assertion expects do not overlap, which is the "+
			"common case and the case here.",
		"Where the overlap is real, restructure the pattern so the assertion's "+
			"text is matched and put back, or check it in code beside the match.",
		"regexp-is-re2")
	return mainPat, lookPat, neg, true
}

// runtimePattern compiles pattern text the converter cannot see, at the point
// where it is used.
func (l *Lowerer) runtimePattern(text ir.Expr, at ast.Node) ir.Expr {
	regexpT := ir.NamedType("*regexp.Regexp", "regexp")
	name := l.tmp("pattern")
	st := assign(":=", []ir.Expr{ir.NewIdent(name, regexpT)},
		[]ir.Expr{call("regexp", "regexp", "MustCompile", regexpT, text)})
	l.setProv(st, at)
	l.approximate(at, "P2G4080", "pattern built at run time",
		"the pattern is compiled every time this runs",
		"The pattern comes from a variable, so it cannot be compiled once at "+
			"start-up. MustCompile panics on a bad pattern, and a pattern built from "+
			"input can be bad.",
		"Compile it with regexp.Compile and check the error, and hoist the "+
			"compilation out of any loop it sits in.",
		"mustcompile-pattern", "errors-are-values")
	l.emit(st)
	return ir.NewIdent(name, regexpT)
}

// patternName invents a readable identifier for a hoisted pattern.
func patternName(raw string, n int) string {
	base := "pattern"
	word := firstWord(raw)
	if word != "" {
		base = goName(word) + "Pattern"
	}
	if n == 0 || word != "" {
		return base
	}
	return base + itoa(n+1)
}

// firstWord finds the longest run of letters in a pattern, which usually names
// what it is looking for.
func firstWord(raw string) string {
	best, cur := "", strings.Builder{}
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c == '\\' {
			i++
			if cur.Len() > len(best) {
				best = cur.String()
			}
			cur.Reset()
			continue
		}
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			cur.WriteByte(c)
			continue
		}
		if cur.Len() > len(best) {
			best = cur.String()
		}
		cur.Reset()
	}
	if cur.Len() > len(best) {
		best = cur.String()
	}
	if len(best) < 3 {
		return ""
	}
	return best
}

// interpolated reports whether a pattern embeds a variable.
func interpolated(rx *ast.Regex) bool {
	// A pattern that is nothing but an interpolated variable has one part,
	// and it is the part that makes the pattern unknown until the program
	// runs. Counting parts rather than looking at them read `/$re/` as the
	// two characters that spell the variable.
	if len(rx.Parts) == 0 {
		return false
	}
	for _, p := range rx.Parts {
		if _, ok := p.(*ast.StrLit); !ok {
			return true
		}
	}
	return false
}

// literalParts joins the parts of an interpolated pattern that are written
// out, with a placeholder standing in for each variable. The placeholder keeps
// the pieces from running together, so that a construct spanning the join is
// not invented out of two innocent halves.
func literalParts(rx *ast.Regex) string {
	var b strings.Builder
	for _, p := range rx.Parts {
		if s, ok := p.(*ast.StrLit); ok {
			b.WriteString(s.Value)
			continue
		}
		b.WriteByte('\x00')
	}
	return b.String()
}

// patternText builds the Go expression for an interpolated pattern.
func (l *Lowerer) patternText(rx *ast.Regex, at ast.Node) ir.Expr {
	var out ir.Expr
	add := func(x ir.Expr) {
		if out == nil {
			out = x
			return
		}
		out = ir.Bin("+", out, x, ir.TString)
	}
	for _, p := range rx.Parts {
		if s, ok := p.(*ast.StrLit); ok {
			if s.Value != "" {
				add(ir.Str(quote(s.Value)))
			}
			continue
		}
		add(l.toStr(l.expr(p), p))
	}
	if out == nil {
		return ir.Str(`""`)
	}
	if flags := goFlags(rx.Mods); flags != "" {
		out = ir.Bin("+", ir.Str(quote(flags)), out, ir.TString)
	}
	return out
}

// translatePattern converts a Perl pattern to RE2 syntax, or refuses with the
// specific feature named.
func (l *Lowerer) translatePattern(rx *ast.Regex, at ast.Node) (string, bool) {
	raw := rx.Raw
	if feature, ok := unsupportedFeature(raw); ok {
		todo := l.refuse(at, feature.code, feature.name,
			feature.short, feature.message, feature.advice, "regexp-is-re2")
		l.patternTodo = &todo
		return "", false
	}

	body := raw
	if strings.Contains(rx.Mods, "x") {
		body = stripExtended(body)
		l.inform(at, "P2G4030", "/x pattern",
			"The /x modifier lets a pattern carry whitespace and comments. Go has no "+
				"such flag, so the whitespace and comments were removed and the pattern "+
				"is written on one line.")
	}
	body = renameCaptures(body)
	body = translateAnchors(body, rx.Mods, l, at)
	body, ok := l.translateScanAnchor(body, rx, at)
	if !ok {
		return "", false
	}

	return goFlags(rx.Mods) + body, true
}

// translateScanAnchor deals with \G, which anchors a match at the position the
// last one left behind.
//
// Go's regexp has no such anchor and no notion of a position on a string, but
// the scan that carries the position hands the engine the text from that
// position onwards, and there `^` means exactly what \G meant. Anywhere else
// the anchor has nothing to refer to, so it is refused rather than quietly
// dropped: a tokeniser whose \G is missing matches the next token anywhere in
// the input instead of at the cursor, and produces a plausible wrong answer.
func (l *Lowerer) translateScanAnchor(body string, rx *ast.Regex, at ast.Node) (string, bool) {
	if !strings.Contains(body, `\G`) {
		return body, true
	}
	if strings.HasPrefix(body, `\G`) && strings.Contains(rx.Mods, "g") {
		rest := body[2:]
		if !strings.Contains(rest, `\G`) {
			l.inform(at, "P2G4090", `\G anchor`,
				"\\G anchors the match where the last one on this variable stopped. Go "+
					"keeps no such position, so the walk carries it in a variable and hands "+
					"the engine the text from there onwards. Against that text `^` is the "+
					"same anchor, which is why it reads as an ordinary start-of-string "+
					"anchor here.",
				"regexp-is-re2")
			return "^" + rest, true
		}
	}
	todo := l.refuse(at, "P2G4090", `\G`,
		"\\G is only available at the start of a global match",
		"\\G anchors a match at the position the previous match on the same variable "+
			"left behind. Go's regexp has no anchor for it and no position to anchor to; "+
			"the converter can express it only as the start of the text a scanning loop "+
			"hands the engine, which means at the start of a pattern used with /g.",
		"Give the walk an explicit index and match against the remaining text, "+
			"which is what the scanning form does and what makes \\G expressible at all.",
		"regexp-is-re2")
	l.patternTodo = &todo
	return "", false
}

// goFlags renders the Perl modifiers Go can express as an inline flag group.
func goFlags(mods string) string {
	var flags string
	for _, m := range mods {
		switch m {
		case 'i':
			flags += "i"
		case 'm':
			flags += "m"
		case 's':
			flags += "s"
		}
	}
	if flags == "" {
		return ""
	}
	return "(?" + flags + ")"
}

// renameCaptures rewrites (?<name>...) into Go's (?P<name>...) spelling, which
// every supported Go version accepts.
func renameCaptures(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			b.WriteByte(s[i])
			b.WriteByte(s[i+1])
			i++
			continue
		}
		if strings.HasPrefix(s[i:], "(?<") && i+3 < len(s) && s[i+3] != '=' && s[i+3] != '!' {
			b.WriteString("(?P<")
			i += 2
			continue
		}
		if strings.HasPrefix(s[i:], "(?'") {
			b.WriteString("(?P<")
			i += 2
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// translateAnchors reports the one anchor whose meaning differs.
func translateAnchors(s string, mods string, l *Lowerer, at ast.Node) string {
	if strings.Contains(mods, "m") || !strings.Contains(s, "$") {
		return s
	}
	// A trailing $ that is a real anchor rather than an escaped dollar.
	if strings.HasSuffix(s, "$") && !strings.HasSuffix(s, `\$`) {
		l.inform(at, "P2G4060", "$ anchor",
			"Without /m, Perl's $ matches at the end of the string or just before a "+
				"final newline. Go's $ matches only at the very end. For lines that have "+
				"already had their newline removed the two agree; for text read whole "+
				"they do not, and `\\n?$` restores the original meaning.")
	}
	return s
}

// unsupportedFeature scans a pattern for the constructs RE2 does not have.
type reFeature struct {
	code, name, short, message, advice string
}

func unsupportedFeature(raw string) (reFeature, bool) {
	type probe struct {
		needle string
		f      reFeature
	}
	probes := []probe{
		{"(?=", reFeature{"P2G4004", "lookahead",
			"lookahead is not available in Go's regexp package",
			"A lookahead asserts what follows without consuming it. Go's regexp is " +
				"RE2, which matches in time linear in the input and cannot backtrack, and " +
				"lookaround needs backtracking.",
			"Match the following text with a capture group and check it separately, " +
				"or test the surrounding condition with strings.HasPrefix and friends."}},
		{"(?!", reFeature{"P2G4004", "negative lookahead",
			"negative lookahead is not available in Go's regexp package",
			"A negative lookahead asserts that some text does not follow. RE2 has no " +
				"backtracking, so it cannot express the assertion.",
			"Match the candidates and reject the unwanted ones in Go code, which is " +
				"usually clearer than the pattern was."}},
		{"(?<=", reFeature{"P2G4005", "lookbehind",
			"lookbehind is not available in Go's regexp package",
			"A lookbehind asserts what precedes the match without consuming it, and " +
				"RE2 cannot do it.",
			"Capture the preceding text in a group and use the capture, or split the " +
				"work into two steps."}},
		{"(?<!", reFeature{"P2G4005", "negative lookbehind",
			"negative lookbehind is not available in Go's regexp package",
			"A negative lookbehind asserts that some text does not precede the match, " +
				"which RE2 cannot express.",
			"Capture the preceding text and reject it in Go code."}},
		{"(?>", reFeature{"P2G4012", "atomic group",
			"atomic grouping is not available in Go's regexp package",
			"An atomic group forbids backtracking into it. RE2 never backtracks, so " +
				"the construct has no meaning and no spelling.",
			"Remove the atomic group. RE2's matching guarantee is what the atomic " +
				"group was protecting against in the first place."}},
		{"(?{", reFeature{"P2G4011", "embedded code",
			"embedded code in a pattern has no Go equivalent",
			"(?{ ... }) runs Perl code during matching. No Go regexp engine does " +
				"anything like it.",
			"Match first, then run the code over the captures."}},
		{"(?R)", reFeature{"P2G4010", "recursive pattern",
			"a recursive pattern has no Go equivalent",
			"A recursive pattern matches nested structures, which needs a stack and " +
				"therefore backtracking.",
			"Nested structure needs a parser. A short hand-written scanner over the " +
				"input is both faster and easier to debug than the pattern was."}},
		{`\K`, reFeature{"P2G4014", `\K`,
			`\K is not available in Go's regexp package`,
			`\K discards everything matched so far, which is a backtracking control ` +
				"verb.",
			"Put the discarded part in a capture group and use the group's bounds, " +
				"or use FindStringSubmatchIndex to get the offsets."}},
	}
	for _, p := range probes {
		if strings.Contains(raw, p.needle) {
			return p.f, true
		}
	}
	if hasBackreference(raw) {
		return reFeature{"P2G4001", "backreference",
			"a backreference is not available in Go's regexp package",
			"A backreference makes the pattern match text it matched earlier. RE2 " +
				"cannot express it: matching in guaranteed linear time rules it out.",
			"Capture the first occurrence and compare it in Go code, which is " +
				"explicit about what is being compared.",
		}, true
	}
	return reFeature{}, false
}

// hasBackreference looks for \1 through \9 outside a character class.
func hasBackreference(raw string) bool {
	for i := 0; i+1 < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		if raw[i+1] >= '1' && raw[i+1] <= '9' {
			return true
		}
		i++
	}
	return false
}

// stripExtended removes the whitespace and comments the /x modifier allows.
func stripExtended(s string) string {
	var b strings.Builder
	inClass := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s):
			b.WriteByte(c)
			b.WriteByte(s[i+1])
			i++
			continue
		case c == '[':
			inClass = true
		case c == ']':
			inClass = false
		}
		if inClass {
			b.WriteByte(c)
			continue
		}
		if c == '#' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// regexpQuoteMeta escapes a literal string for use as a pattern.
func regexpQuoteMeta(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if strings.IndexByte(`\.+*?()|[]{}^$`, s[i]) >= 0 {
			b.WriteByte('\\')
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// countGroups counts the capture groups in a pattern.
func countGroups(raw string) int {
	n := 0
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '\\':
			i++
		case '[':
			for i < len(raw) && raw[i] != ']' {
				if raw[i] == '\\' {
					i++
				}
				i++
			}
		case '(':
			if i+1 < len(raw) && raw[i+1] == '?' {
				if i+2 < len(raw) && (raw[i+2] == 'P' || raw[i+2] == '<' || raw[i+2] == '\'') {
					n++
				}
				continue
			}
			n++
		}
	}
	return n
}

// namedGroups lists the named captures in a pattern, in order.
func namedGroups(raw string) map[string]int {
	out := map[string]int{}
	idx := 0
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '\\':
			i++
		case '(':
			if i+1 < len(raw) && raw[i+1] == '?' {
				rest := raw[i+2:]
				for _, prefix := range []string{"P<", "<", "'"} {
					if strings.HasPrefix(rest, prefix) {
						if prefix == "<" && len(rest) > 1 && (rest[1] == '=' || rest[1] == '!') {
							break
						}
						end := strings.IndexAny(rest[len(prefix):], ">'")
						if end >= 0 {
							idx++
							out[rest[len(prefix):len(prefix)+end]] = idx
						}
						break
					}
				}
				continue
			}
			idx++
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Matching

// matchExpr lowers `EXPR =~ /pattern/`.
func (l *Lowerer) matchExpr(n *ast.Match, forceBool bool) ir.Expr {
	normalizeMatch(n)
	if x, ok := l.scanMatch(n); ok {
		return x
	}
	subject := l.matchSubject(n.Bound)
	if g, _ := l.matchGroups(n); g == 0 && n.Pattern != nil {
		if mainPat, lookPat, neg, ok := l.lookAheadPair(n.Pattern, n); ok {
			out := l.helperCall(hMatchAhead, ir.TBool, mainPat, lookPat,
				ir.BoolLit(neg), l.toStr(subject, n))
			l.note(out, "The pattern ends in a lookaround, which Go's regexp does not "+
				"have, so it runs as two patterns: the helper matches the first and "+
				"tests the second against what follows the match, consuming nothing.",
				"regexp-is-re2")
			if n.Negate {
				return negated(out)
			}
			return out
		}
	}
	pattern, ok := l.patternOf(matchPattern(n))
	if !ok {
		return l.markRefusedPattern(ir.BoolLit(false))
	}

	groups, named := l.matchGroups(n)
	if groups == 0 {
		out := ir.CallOf(selector(pattern, "MatchString", nil), ir.TBool, subject)
		if n.Negate {
			out2 := negated(out)
			l.note(out2, "!~ is a match that succeeds when the pattern does not match, "+
				"which Go writes by negating the test.")
			return out2
		}
		l.note(out, "MatchString answers the same question as a bare match in boolean "+
			"context: did the pattern match anywhere in the string.",
			"regexp-is-re2")
		return out
	}

	name := l.tmp("m")
	st := assign(":=", []ir.Expr{ir.NewIdent(name, ir.SliceOf(ir.TString))},
		[]ir.Expr{ir.CallOf(selector(pattern, "FindStringSubmatch", nil), ir.SliceOf(ir.TString), subject)})
	l.setProv(st, n)
	l.note(st, "Perl leaves the captures in $1, $2 and so on, which are global and "+
		"survive until the next match. Go returns them as a slice: index 0 is the "+
		"whole match and index 1 onwards are the groups. A nil slice means no match, "+
		"so the test and the captures come from one call.",
		"submatch-and-named-groups", "nil-slices-vs-nil-maps")
	l.emit(st)

	frame := &captureFrame{Name: name, Count: groups, Named: named}
	l.captureStack = append(l.captureStack, frame)

	out := ir.Bin("!=", ir.NewIdent(name, ir.SliceOf(ir.TString)), ir.Nil(ir.SliceOf(ir.TString)), ir.TBool)
	if n.Negate {
		return ir.Bin("==", ir.NewIdent(name, ir.SliceOf(ir.TString)), ir.Nil(ir.SliceOf(ir.TString)), ir.TBool)
	}
	return out
}

// scanMatch lowers `$var =~ /pattern/g` used for its truth value, which is the
// idiom that walks a string one match at a time.
//
// Perl remembers where it got to on the variable itself, so the same expression
// in a while loop yields a different match each time round. Go has no such
// memory, so the position becomes a variable and the step becomes a call that
// takes it and moves it on.
func (l *Lowerer) scanMatch(n *ast.Match) (ir.Expr, bool) {
	if n.Pattern == nil || !strings.Contains(n.Pattern.Mods, "g") || n.Negate {
		return nil, false
	}
	v, ok := n.Bound.(*ast.Var)
	if !ok || v.Sigil != '$' {
		return nil, false
	}
	pattern, ok := l.patternOf(matchPattern(n))
	if !ok {
		return nil, false
	}
	b := l.lookup(v.Sigil, v.Name, v)
	p := l.matchPos(b, n)

	name := l.tmp("m")
	matchT := ir.SliceOf(ir.TString)
	step := assign(":=", []ir.Expr{ir.NewIdent(name, matchT)},
		[]ir.Expr{l.helperCall(hNextMatch, matchT, pattern,
			l.toStr(l.ident(b), v), ir.Un("&", ir.NewIdent(p.Go, ir.TInt), ir.PointerTo(ir.TInt)))})
	l.setProv(step, n)
	l.note(step, "A global match in scalar context is a cursor: each evaluation "+
		"starts where the last one stopped, and the position lives on the variable. "+
		"Go's regexp has no cursor, so the position is passed in by address and the "+
		"call moves it along.",
		"regexp-is-re2", "submatch-and-named-groups")
	l.emit(step)

	// Without /c a failed match forgets the position, so the next scan starts
	// at the beginning again.
	if !strings.Contains(n.Pattern.Mods, "c") {
		reset := &ir.If{
			Cond: ir.Bin("==", ir.NewIdent(name, matchT), ir.Nil(matchT), ir.TBool),
			Then: &ir.Block{Stmts: []ir.Stmt{
				assign("=", []ir.Expr{ir.NewIdent(p.Go, ir.TInt)}, []ir.Expr{ir.IntLit("0")}),
			}},
		}
		l.note(reset, "A failed global match resets the position to the start unless "+
			"the pattern carries /c. That is what makes a second loop over the same "+
			"string begin again rather than find nothing.")
		l.emit(reset)
	}

	frame := &captureFrame{Name: name, Count: countGroups(n.Pattern.Raw), Named: namedGroups(n.Pattern.Raw)}
	l.captureStack = append(l.captureStack, frame)

	l.approximate(n, "P2G4530", "//g in scalar context",
		"the match position is a variable rather than a property of the string",
		"Perl keeps the position on the scalar, so assigning to the variable forgets "+
			"it and every copy of the string has its own. The generated code keeps one "+
			"position variable per scanned variable, which matches the usual one-loop "+
			"case and not the case of two scans over the same variable at once.",
		"Where two scans overlap, give each one its own position variable.")
	return ir.Bin("!=", ir.NewIdent(name, matchT), ir.Nil(matchT), ir.TBool), true
}

// captureList lowers a match used in list context, where Perl yields the
// capture groups rather than a truth value.
//
// This is the shape behind `my ($name, $rest) = $line =~ /(\w+)\s+(.*)/`, one
// of the most common lines in any Perl script. Go returns the submatches as a
// slice, so the groups are read out of it by index; a failed match returns nil,
// and reading past the end of nil is the one place where the tolerant index
// helper earns its keep.
// matchYieldsList reports whether a match hands back a list when it is read in
// list context: /g collects every match, and capture groups come out one value
// per group. A match with neither is a truth value wherever it appears.
func (l *Lowerer) matchYieldsList(n *ast.Match) bool {
	if n == nil || n.Negate {
		return false
	}
	normalizeMatch(n)
	if n.Pattern != nil && strings.Contains(n.Pattern.Mods, "g") {
		return true
	}
	groups, _ := l.matchGroups(n)
	return groups > 0
}

func (l *Lowerer) captureList(n *ast.Match) ([]ir.Expr, bool) {
	normalizeMatch(n)
	if n.Negate {
		return nil, false
	}
	if n.Pattern != nil && strings.Contains(n.Pattern.Mods, "g") {
		return nil, false
	}
	groups, _ := l.matchGroups(n)
	if groups == 0 {
		return nil, false
	}
	depth := l.captureDepth()
	l.matchExpr(n, false)
	if l.captureDepth() <= depth {
		return nil, false
	}
	frame := l.captureStack[len(l.captureStack)-1]

	out := make([]ir.Expr, 0, groups)
	for i := 1; i <= groups; i++ {
		out = append(out, l.helperCall(hAt, ir.TString,
			ir.NewIdent(frame.Name, ir.SliceOf(ir.TString)), ir.IntLit(itoa(i))))
	}
	if groups == 1 {
		l.note(out[0], "The parentheses on the left of this assignment are what make the "+
			"match yield its capture group rather than a yes or no. Written without "+
			"them the same match is a truth value, and Go has to pick one of the two "+
			"at the point of use: FindStringSubmatch for the group, MatchString for "+
			"the question.",
			"submatch-and-named-groups", "context-is-gone")
	}
	l.approximate(n, "P2G4510", "a match read for its captures",
		"a failed match yields empty strings rather than nothing",
		"A match in list context yields its capture groups when it succeeds and the "+
			"empty list when it does not, so the variables on the left are left undef. "+
			"Go returns a nil slice for no match, and the groups read out of it are "+
			"empty strings.",
		"Test whether the match succeeded before using the groups. The submatch "+
			"slice being nil is that test.",
		"submatch-and-named-groups", "nil-vs-undef")
	return out, true
}

// notePattern records what a qr// assigned to a variable captures, so that a
// later match against that variable knows how many groups it has and what they
// are called. Perl carries that inside the compiled pattern; Go's
// *regexp.Regexp knows it too, but only while the program runs.
func notePattern(b *Binding, rhs ast.Expr) {
	q, ok := rhs.(*ast.QrExpr)
	if !ok || q.Pattern == nil || b == nil {
		return
	}
	b.Groups = countGroups(q.Pattern.Raw)
	b.NamedGroups = namedGroups(q.Pattern.Raw)
}

// globalMatch lowers `EXPR =~ /pattern/g` in list context, where Perl yields
// every match rather than a truth value.
func (l *Lowerer) globalMatch(n *ast.Match) (ir.Expr, bool) {
	if n.Pattern == nil || !strings.Contains(n.Pattern.Mods, "g") || n.Negate {
		return nil, false
	}
	subject := l.matchSubject(n.Bound)
	pattern, ok := l.patternOf(matchPattern(n))
	if !ok {
		return nil, false
	}
	groups := countGroups(n.Pattern.Raw)
	if groups == 0 {
		out := ir.CallOf(selector(pattern, "FindAllString", nil), ir.SliceOf(ir.TString),
			subject, ir.IntLit("-1"))
		l.note(out, "A global match in list context collects every match. "+
			"FindAllString does the same; the -1 asks for all of them rather than a "+
			"limited number, and it returns nil when there are none, which ranges "+
			"zero times.",
			"regexp-is-re2", "nil-slices-vs-nil-maps")
		return out, true
	}

	// With capture groups Perl returns the groups themselves, flattened. Go
	// returns one slice per match, so the flattening is written out.
	name := l.tmp("found")
	decl := &ir.DeclStmt{Names: []string{name}, Type: ir.SliceOf(ir.TString)}
	matchName := l.tmp("m")
	loop := &ir.Range{
		Key:   ir.NewIdent("_", ir.TInt),
		Value: ir.NewIdent(matchName, ir.SliceOf(ir.TString)),
		X: ir.CallOf(selector(pattern, "FindAllStringSubmatch", nil),
			ir.SliceOf(ir.SliceOf(ir.TString)), subject, ir.IntLit("-1")),
		Define: true,
		Body: &ir.Block{Stmts: []ir.Stmt{
			assign("=", []ir.Expr{ir.NewIdent(name, ir.SliceOf(ir.TString))},
				[]ir.Expr{func() ir.Expr {
					c := appendTo(ir.NewIdent(name, ir.SliceOf(ir.TString)),
						slicing(ir.NewIdent(matchName, ir.SliceOf(ir.TString)), ir.IntLit("1"), nil, ir.SliceOf(ir.TString)))
					c.Ellipsis = true
					return c
				}()}),
		}},
	}
	l.setProv(decl, n)
	l.note(decl, "A global match with capture groups yields the groups of every "+
		"match, flattened into one list. Go keeps the matches separate, one slice "+
		"per match with the whole match at index 0, so the flattening is written "+
		"out here.",
		"submatch-and-named-groups")
	l.emit(decl)
	l.emit(loop)
	return ir.NewIdent(name, ir.SliceOf(ir.TString)), true
}

// matchPattern picks the pattern node out of a match.
func matchPattern(n *ast.Match) ast.Expr {
	if n.Pattern != nil {
		return n.Pattern
	}
	return n.PatternExpr
}

// matchSubject lowers what a match is applied to, defaulting to the topic.
func (l *Lowerer) matchSubject(bound ast.Expr) ir.Expr {
	if bound == nil {
		if len(l.topicStack) > 0 {
			return l.toStr(l.topicStack[len(l.topicStack)-1], nil)
		}
		return l.toStr(l.varExpr(&ast.Var{Sigil: '$', Name: "_"}), nil)
	}
	return l.toStr(l.expr(bound), bound)
}

// captureVar resolves $1, $2 and so on against the innermost active match.
func (l *Lowerer) captureVar(n int) (ir.Expr, bool) {
	if len(l.captureStack) == 0 {
		return nil, false
	}
	f := l.captureStack[len(l.captureStack)-1]
	if n < 1 || n > f.Count {
		return nil, false
	}
	return index(ir.NewIdent(f.Name, ir.SliceOf(ir.TString)), ir.IntLit(itoa(n)), ir.TString), true
}

// wholeMatch resolves $&, which is group 0 of the innermost match in scope.
func (l *Lowerer) wholeMatch() (ir.Expr, bool) {
	if len(l.captureStack) == 0 {
		return nil, false
	}
	f := l.captureStack[len(l.captureStack)-1]
	return index(ir.NewIdent(f.Name, ir.SliceOf(ir.TString)), ir.IntLit("0"), ir.TString), true
}

// namedCapture resolves $+{name}.
func (l *Lowerer) namedCapture(name string) (ir.Expr, bool) {
	if len(l.captureStack) == 0 {
		return nil, false
	}
	f := l.captureStack[len(l.captureStack)-1]
	idx, ok := f.Named[name]
	if !ok {
		return nil, false
	}
	return index(ir.NewIdent(f.Name, ir.SliceOf(ir.TString)), ir.IntLit(itoa(idx)), ir.TString), true
}

// qrExpr lowers qr//.
func (l *Lowerer) qrExpr(n *ast.QrExpr) ir.Expr {
	p, ok := l.patternOf(n)
	if !ok {
		// A bare nil has no type, so `re := nil` does not compile. The
		// conversion is a stand-in either way; it may as well be one the
		// compiler accepts, so the rest of the file can still be built and
		// read.
		return l.markRefusedPattern(
			ir.Raw("(*regexp.Regexp)(nil)", ir.NamedType("*regexp.Regexp", "regexp")))
	}
	l.note(p, "qr// precompiles a pattern into a value that can be stored and reused. "+
		"A *regexp.Regexp is exactly that, and Go programs normally keep one in a "+
		"package-level variable.",
		"mustcompile-pattern")
	return p
}

// ---------------------------------------------------------------------------
// Substitution

// substStmt lowers s/// in statement position.
func (l *Lowerer) substStmt(n *ast.Subst) []ir.Stmt {
	if n.EvalRepl || !templateRepl(n.Repl) {
		return l.substEval(n)
	}

	target := l.substTarget(n)
	if target == nil {
		return l.refusedStmtMarker(n)
	}
	if mainPat, lookPat, neg, ok := l.lookAheadPair(n.Pattern, n); ok {
		repl, replOK := l.replacement(n)
		if !replOK {
			return l.refusedStmtMarker(n)
		}
		out := l.helperCall(hReplaceAhead, ir.TString, mainPat, lookPat,
			ir.BoolLit(neg), ir.BoolLit(strings.Contains(n.Pattern.Mods, "g")),
			l.toStr(target, nil), repl)
		st := assign("=", []ir.Expr{target}, []ir.Expr{out})
		l.setProv(st, n)
		l.note(st, "The pattern ends in a lookaround, which Go's regexp does not "+
			"have, so the substitution runs as two patterns: the helper replaces "+
			"each match of the first whose tail satisfies the second, and what the "+
			"assertion looked at stays in the text for the next match to use.",
			"regexp-is-re2", "replace-and-expansion")
		return []ir.Stmt{st}
	}
	pattern, ok := l.patternOf(n.Pattern)
	if !ok {
		return l.refusedStmtMarker(n)
	}
	repl, replOK := l.replacement(n)
	if !replOK {
		return l.refusedStmtMarker(n)
	}

	global := strings.Contains(n.Pattern.Mods, "g")
	method := "ReplaceAllString"
	if !global {
		out := l.helperCall(hReplaceFirst, ir.TString, pattern, l.toStr(target, nil), repl)
		st := assign("=", []ir.Expr{target}, []ir.Expr{out})
		l.setProv(st, n)
		l.note(st, "Without /g a substitution replaces only the first match. Go's "+
			"ReplaceAllString always replaces every match, so a small helper does the "+
			"first-only form.",
			"replace-and-expansion")
		return []ir.Stmt{st}
	}
	out := ir.CallOf(selector(pattern, method, nil), ir.TString, l.toStr(target, nil), repl)
	st := assign("=", []ir.Expr{target}, []ir.Expr{out})
	l.setProv(st, n)
	l.note(st, "s///g becomes ReplaceAllString. The replacement is a template where "+
		"${1} names the first capture group. Perl writes that as $1; the braces are "+
		"required in Go so that ${1}0 is not read as group 10.",
		"replace-and-expansion", "submatch-and-named-groups")
	return []ir.Stmt{st}
}

// substEval lowers `s/pattern/CODE/e`, where the replacement is an expression
// evaluated once per match rather than a template.
//
// Go has the same shape in ReplaceAllStringFunc, with one wrinkle: the function
// is handed the whole match and not the groups, so a replacement that reads $1
// has to look them up again inside it.
func (l *Lowerer) substEval(n *ast.Subst) []ir.Stmt {
	target := l.substTarget(n)
	if target == nil {
		return nil
	}
	out, ok := l.substByFunc(n, target)
	if !ok {
		return nil
	}
	st := assign("=", []ir.Expr{target}, []ir.Expr{out})
	l.setProv(st, n)
	return []ir.Stmt{st}
}

// substByFunc builds the ReplaceAllStringFunc form of a substitution, which is
// what a replacement that is not a template has to become.
//
// Two kinds of replacement need it. /e makes the replacement code rather than
// text, and a replacement that folds case or otherwise transforms a capture
// group is code too, however it was spelled: Go's replacement template can
// name a group and can do nothing else with it.
func (l *Lowerer) substByFunc(n *ast.Subst, target ir.Expr) (ir.Expr, bool) {
	pattern, ok := l.patternOf(n.Pattern)
	if !ok {
		return nil, false
	}

	whole := l.tmp("match")
	matches := l.tmp("groups")
	matchT := ir.SliceOf(ir.TString)

	find := assign(":=", []ir.Expr{ir.NewIdent(matches, matchT)},
		[]ir.Expr{ir.CallOf(selector(pattern, "FindStringSubmatch", nil), matchT,
			ir.NewIdent(whole, ir.TString))})

	frame := &captureFrame{Name: matches, Count: countGroups(n.Pattern.Raw)}
	if n.Pattern != nil {
		frame.Named = namedGroups(n.Pattern.Raw)
	}
	l.captureStack = append(l.captureStack, frame)
	depth := len(l.captureStack)

	savedPre := l.pre
	l.pre = nil
	value := l.toStr(l.expr(n.Repl), n.Repl)
	body := append(l.takePre(), &ir.Return{Results: []ir.Expr{value}})
	l.pre = savedPre
	l.restoreCaptures(depth - 1)

	// The groups are only looked up when the replacement reads one. A
	// replacement that only uses the whole match, or none of it, would
	// otherwise declare a variable Go refuses to leave unread.
	stmts := body
	reads := map[string]int{}
	for _, st := range body {
		countReads(st, reads)
	}
	if reads[matches] > 0 {
		stmts = append([]ir.Stmt{find}, body...)
	}
	fn := funcLit([]ir.Param{{Name: whole, Type: ir.TString}}, []*ir.Type{ir.TString},
		&ir.Block{Stmts: stmts})

	global := strings.Contains(n.Pattern.Mods, "g")
	var out ir.Expr
	if global {
		out = ir.CallOf(selector(pattern, "ReplaceAllStringFunc", nil), ir.TString,
			l.toStr(target, nil), fn)
	} else {
		out = l.helperCall(hReplaceFirstFunc, ir.TString, pattern, l.toStr(target, nil), fn)
	}
	if n.EvalRepl {
		l.note(out, "The /e modifier makes the replacement code rather than a template, "+
			"and ReplaceAllStringFunc is the same idea: it calls a function for every "+
			"match and uses what comes back. The function is handed the matched text "+
			"only, so the capture groups are found again inside it.",
			"replace-and-expansion", "submatch-and-named-groups")
	} else {
		l.note(out, "Go's replacement template can name a capture group and can do "+
			"nothing else with it: there is no case folding and no arithmetic in it. "+
			"A replacement that transforms what it captured is code, so it becomes a "+
			"function called once per match, which is also where the groups have to "+
			"be found again.",
			"replace-and-expansion", "submatch-and-named-groups")
	}

	if !global {
		l.note(out, "Without /g only the first match is replaced. The regexp package "+
			"has no first-only form of either replacement call, so a small helper "+
			"cuts the string at the match and puts the pieces back together.",
			"replace-and-expansion")
	}
	return out, true
}

// templateRepl reports whether a replacement is something Go's replacement
// template can say.
//
// The template understands literal text and ${n}, and nothing else. A part
// that mentions a capture group without being one is a computation over the
// match, which has to run per match rather than once before the replacement.
func templateRepl(repl ast.Expr) bool {
	switch r := repl.(type) {
	case nil, *ast.StrLit:
		return true
	case *ast.InterpLit:
		for _, p := range r.Parts {
			if v, ok := p.(*ast.Var); ok && v.Sigil == '$' && isDigits(v.Name) {
				continue
			}
			if _, ok := p.(*ast.StrLit); ok {
				continue
			}
			if mentionsCapture(p) {
				return false
			}
		}
		return true
	}
	return !mentionsCapture(repl)
}

// mentionsCapture reports whether an expression reads a capture group, which
// is what makes it belong inside the per-match function rather than in front
// of the replacement.
func mentionsCapture(e ast.Expr) bool {
	found := false
	walkExprs([]ast.Stmt{&ast.ExprStmt{X: e}}, func(x ast.Expr) {
		if v, ok := x.(*ast.Var); ok && v.Sigil == '$' && (isDigits(v.Name) || v.Name == "&") {
			found = true
		}
	})
	return found
}

// substExpr lowers s/// used for its value, which is the number of
// substitutions made.
func (l *Lowerer) substExpr(n *ast.Subst) ir.Expr {
	// /r is the one form that answers with the new text: it leaves the
	// original alone, which is what makes it usable inside a map block.
	if n.Pattern != nil && strings.Contains(n.Pattern.Mods, "r") {
		if x, ok := l.substCopy(n); ok {
			return x
		}
	}
	// The count has to be taken before the substitution runs, because
	// afterwards there is nothing left to count. Reading the target first
	// also means substStmt's own resolution of it does not run twice.
	count := l.substCount(n)
	for _, st := range l.substStmt(n) {
		l.emit(st)
	}
	if count != nil {
		return count
	}
	if t := l.substTarget(n); t != nil {
		return t
	}
	return ir.Str(`""`)
}

// substCopy lowers `s///r`, which yields the edited text and leaves the
// variable it was applied to untouched.
func (l *Lowerer) substCopy(n *ast.Subst) (ir.Expr, bool) {
	target := l.peekSubstTarget(n)
	if target == nil {
		return nil, false
	}
	if !n.EvalRepl && templateRepl(n.Repl) {
		if mainPat, lookPat, neg, ok := l.lookAheadPair(n.Pattern, n); ok {
			repl, replOK := l.replacement(n)
			if !replOK {
				return nil, false
			}
			out := l.helperCall(hReplaceAhead, ir.TString, mainPat, lookPat,
				ir.BoolLit(neg), ir.BoolLit(strings.Contains(n.Pattern.Mods, "g")),
				l.toStr(target, n), repl)
			l.note(out, "The pattern ends in a lookaround, so the substitution runs "+
				"as two patterns; /r hands back the edited copy, which is the only "+
				"form Go has anyway.",
				"regexp-is-re2", "replace-and-expansion")
			return out, true
		}
	}
	pattern, ok := l.patternOf(n.Pattern)
	if !ok {
		return nil, false
	}
	if n.EvalRepl || !templateRepl(n.Repl) {
		out, ok := l.substByFunc(n, target)
		if !ok {
			return nil, false
		}
		return out, true
	}
	repl, ok := l.replacement(n)
	if !ok {
		return nil, false
	}
	var out ir.Expr
	if strings.Contains(n.Pattern.Mods, "g") {
		out = ir.CallOf(selector(pattern, "ReplaceAllString", nil), ir.TString,
			l.toStr(target, n), repl)
	} else {
		out = l.helperCall(hReplaceFirst, ir.TString, pattern, l.toStr(target, n), repl)
	}
	l.note(out, "/r edits a copy and hands it back, leaving the variable alone. Go "+
		"has only that form: a string cannot be changed in place at all, so every "+
		"replacement returns a new one and the /r question never comes up.",
		"replace-and-expansion", "strings-are-bytes")
	return out, true
}

// substCount names how many matches a substitution is about to replace, which
// is the value `s///` yields.
func (l *Lowerer) substCount(n *ast.Subst) ir.Expr {
	if n.Pattern == nil {
		return nil
	}
	if mainPat, lookPat, neg, ok := l.lookAheadPair(n.Pattern, n); ok {
		target := l.peekSubstTarget(n)
		if target == nil {
			return nil
		}
		name := l.tmp("replaced")
		st := assign(":=", []ir.Expr{ir.NewIdent(name, ir.TInt)},
			[]ir.Expr{l.helperCall(hCountAhead, ir.TInt, mainPat, lookPat,
				ir.BoolLit(neg), ir.BoolLit(strings.Contains(n.Pattern.Mods, "g")),
				l.toStr(target, n))})
		l.setProv(st, n)
		l.note(st, "A substitution answers with how many matches it replaced, so "+
			"they are counted before the replacement removes them, by the same "+
			"two-pattern scan the replacement itself uses.",
			"replace-and-expansion", "regexp-is-re2")
		l.emit(st)
		return ir.NewIdent(name, ir.TInt)
	}
	pattern, ok := l.patternOf(n.Pattern)
	if !ok {
		return nil
	}
	target := l.peekSubstTarget(n)
	if target == nil {
		return nil
	}
	name := l.tmp("replaced")
	var value ir.Expr
	if strings.Contains(n.Pattern.Mods, "g") {
		value = lenOf(ir.CallOf(selector(pattern, "FindAllString", nil), ir.SliceOf(ir.TString),
			l.toStr(target, n), ir.IntLit("-1")))
	} else {
		value = ir.NewIdent(name, ir.TInt)
		// A pattern with groups leaves them in $1, $2 and so on, and
		// `if ($line =~ s/^(\S+)\s+//) { $owner = $1 }` is how a parser strips
		// a field and keeps it in one line. The call that decides whether
		// anything matched is also the call that has the groups, so asking for
		// the submatches rather than a yes or no costs nothing and is the only
		// way the branch below can read them.
		cond := ir.Expr(ir.CallOf(selector(pattern, "MatchString", nil), ir.TBool, l.toStr(target, n)))
		if groups, named := l.substGroups(n); groups > 0 {
			matchT := ir.SliceOf(ir.TString)
			mName := l.tmp("m")
			find := assign(":=", []ir.Expr{ir.NewIdent(mName, matchT)},
				[]ir.Expr{ir.CallOf(selector(pattern, "FindStringSubmatch", nil), matchT,
					l.toStr(target, n))})
			l.setProv(find, n)
			l.note(find, "The groups this pattern captures are gone once the "+
				"replacement has run, so they are taken here, from the same call that "+
				"says whether anything matched. Perl left them in $1 and $2, which are "+
				"globals that outlive the match; Go returns them, and a nil slice means "+
				"the pattern did not match.",
				"submatch-and-named-groups", "replace-and-expansion")
			l.emit(find)
			l.captureStack = append(l.captureStack,
				&captureFrame{Name: mName, Count: groups, Named: named})
			cond = ir.Bin("!=", ir.NewIdent(mName, matchT), ir.Nil(matchT), ir.TBool)
		}
		decl := &ir.DeclStmt{Names: []string{name}, Type: ir.TInt}
		set := &ir.If{
			Cond: cond,
			Then: &ir.Block{Stmts: []ir.Stmt{
				assign("=", []ir.Expr{ir.NewIdent(name, ir.TInt)}, []ir.Expr{ir.IntLit("1")}),
			}},
		}
		l.setProv(decl, n)
		l.note(decl, "A substitution answers with how many matches it replaced, not "+
			"with the new text, so the count is taken before the replacement runs. "+
			"Without /g there is at most one.",
			"replace-and-expansion")
		l.emit(decl)
		l.emit(set)
		return value
	}
	st := assign(":=", []ir.Expr{ir.NewIdent(name, ir.TInt)}, []ir.Expr{value})
	l.setProv(st, n)
	l.note(st, "A substitution answers with how many matches it replaced, not with "+
		"the new text, so they are counted before the replacement removes them.",
		"replace-and-expansion")
	l.emit(st)
	return ir.NewIdent(name, ir.TInt)
}

// peekSubstTarget resolves what a substitution modifies without emitting
// anything, so that the count can read it before the replacement runs.
func (l *Lowerer) peekSubstTarget(n *ast.Subst) ir.Expr {
	if n.Bound == nil {
		if len(l.topicStack) > 0 {
			return l.topicStack[len(l.topicStack)-1]
		}
		return nil
	}
	if _, ok := boundAssign(n.Bound); ok {
		// The assignment has not run yet, so there is nothing to read.
		return nil
	}
	return l.assignTarget(n.Bound)
}

// substTarget resolves what a substitution modifies.
func (l *Lowerer) substTarget(n *ast.Subst) ir.Expr {
	if n.Bound == nil {
		if len(l.topicStack) > 0 {
			return l.topicStack[len(l.topicStack)-1]
		}
		return l.assignTarget(&ast.Var{Sigil: '$', Name: "_"})
	}
	// `(my $copy = $original) =~ s/.../.../` is how Perl edits a copy without
	// touching the original: the parenthesised assignment yields the new
	// variable, and the substitution runs on that. The assignment is a
	// statement of its own here and the substitution then names the copy.
	if a, ok := boundAssign(n.Bound); ok {
		for _, st := range l.assignStmts(a) {
			l.emit(st)
		}
		if v, single := soleDeclaredVar(a); single {
			return l.assignTarget(v)
		}
		return l.assignTarget(a.LHS)
	}
	return l.assignTarget(n.Bound)
}

// boundAssign reads the assignment out of the parentheses a copy-then-edit is
// always written with.
func boundAssign(e ast.Expr) (*ast.Assign, bool) {
	for {
		switch n := e.(type) {
		case *ast.Assign:
			if n.Op != "=" {
				return nil, false
			}
			return n, true
		case *ast.List:
			if len(n.Elems) != 1 {
				return nil, false
			}
			e = n.Elems[0]
		default:
			return nil, false
		}
	}
}

// soleDeclaredVar returns the one variable a `my` on the left of an assignment
// introduces, when there is exactly one.
func soleDeclaredVar(a *ast.Assign) (*ast.Var, bool) {
	my, ok := a.LHS.(*ast.My)
	if !ok {
		return nil, false
	}
	vars := declaredVars(my)
	if len(vars) != 1 {
		return nil, false
	}
	return vars[0], true
}

// replacement builds the Go replacement template.
func (l *Lowerer) replacement(n *ast.Subst) (ir.Expr, bool) {
	if n.Repl == nil {
		return ir.Str(`""`), true
	}
	switch r := n.Repl.(type) {
	case *ast.StrLit:
		return ir.Str(quote(escapeTemplate(r.Value))), true
	case *ast.InterpLit:
		var out ir.Expr
		add := func(x ir.Expr) {
			if out == nil {
				out = x
				return
			}
			out = ir.Bin("+", out, x, ir.TString)
		}
		for _, p := range r.Parts {
			if s, ok := p.(*ast.StrLit); ok {
				if s.Value != "" {
					add(ir.Str(quote(escapeTemplate(s.Value))))
				}
				continue
			}
			if v, ok := p.(*ast.Var); ok && v.Sigil == '$' && isDigits(v.Name) {
				add(ir.Str(quote("${" + v.Name + "}")))
				continue
			}
			// An outer variable in the replacement has to be escaped at run
			// time, because a $ in it would be read as a group reference.
			add(call("strings", "strings", "ReplaceAll", ir.TString,
				l.toStr(l.expr(p), p), ir.Str(`"$"`), ir.Str(`"$$"`)))
		}
		if out == nil {
			return ir.Str(`""`), true
		}
		return out, true
	}
	return l.toStr(l.expr(n.Repl), n.Repl), true
}

// escapeTemplate protects a literal dollar sign from being read as a group
// reference by the replacement template.
func escapeTemplate(s string) string { return strings.ReplaceAll(s, "$", "$$") }

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Transliteration

// transStmt lowers tr///.
func (l *Lowerer) transStmt(n *ast.Trans) []ir.Stmt {
	target := l.transTarget(n)
	if target == nil {
		return nil
	}
	// Every form below works on text, so a target whose type did not resolve
	// is read as text on the way in. Storing a string back into a dynamic
	// variable is fine, which is why only the read side needs the conversion.
	text := l.toStr(target, nil)
	if mods := n.Mods; mods != "" {
		if sts, ok := l.transModified(n, target, text); ok {
			return sts
		}
		return []ir.Stmt{l.todoStmt(n, "P2G4520", "tr/// with modifiers",
			"this combination of tr modifiers is not implemented",
			"The d, s, c and r modifiers change what tr does: delete unreplaced "+
				"characters, squash repeats, complement the search list, or return the "+
				"result instead of modifying in place.",
			"strings.Map returning -1 deletes a character, and strings.NewReplacer "+
				"handles multi-character replacements. Pick the one that matches the "+
				"modifier that was used.")}
	}

	search := expandTrList(n.SearchList)
	repl := expandTrList(n.ReplList)
	if len(search) == 0 {
		return nil
	}

	if s, r := string(search), string(repl); s == lowerAlphabet && r == upperAlphabet {
		st := assign("=", []ir.Expr{target},
			[]ir.Expr{call("strings", "strings", "ToUpper", ir.TString, text)})
		l.setProv(st, n)
		l.note(st, "tr/a-z/A-Z/ is upper-casing spelled character by character. "+
			"strings.ToUpper says the same thing and handles every alphabet, not just "+
			"the ASCII one.")
		return []ir.Stmt{st}
	}
	if s, r := string(search), string(repl); s == upperAlphabet && r == lowerAlphabet {
		st := assign("=", []ir.Expr{target},
			[]ir.Expr{call("strings", "strings", "ToLower", ir.TString, text)})
		l.setProv(st, n)
		return []ir.Stmt{st}
	}

	if len(repl) == 0 {
		// With no replacement list tr leaves the string alone and answers how
		// many of its characters were in the search list, which is how Perl
		// counts occurrences of a character class.
		l.inform(n, "P2G4521", "tr with an empty replacement",
			"tr with no replacement list does not change the string: it counts how "+
				"many of its characters are in the search list. The count is the value "+
				"of the expression, and the string is left as it was.")
		return nil
	}

	var pairs []ir.Expr
	for i, c := range search {
		to := repl[len(repl)-1]
		if i < len(repl) {
			to = repl[i]
		}
		pairs = append(pairs, ir.Str(quote(string(c))), ir.Str(quote(string(to))))
	}
	name := l.tmp("replacer")
	decl := assign(":=", []ir.Expr{ir.NewIdent(name, ir.NamedType("*strings.Replacer", "strings"))},
		[]ir.Expr{call("strings", "strings", "NewReplacer", ir.NamedType("*strings.Replacer", "strings"), pairs...)})
	l.setProv(decl, n)
	l.note(decl, "tr/// replaces characters one for one. strings.NewReplacer takes "+
		"the pairs and builds a replacer that can be reused, which is worth hoisting "+
		"out of a loop.")
	l.emit(decl)
	st := assign("=", []ir.Expr{target},
		[]ir.Expr{ir.CallOf(selector(ir.NewIdent(name, nil), "Replace", nil), ir.TString, text)})
	return []ir.Stmt{st}
}

// transModified lowers tr/// with the c, d, s or r modifiers, which change
// what a plain character-for-character replacement means.
func (l *Lowerer) transModified(n *ast.Trans, target, text ir.Expr) ([]ir.Stmt, bool) {
	complement := strings.Contains(n.Mods, "c")
	del := strings.Contains(n.Mods, "d")
	squeeze := strings.Contains(n.Mods, "s")
	ret := strings.Contains(n.Mods, "r")
	for _, m := range n.Mods {
		if !strings.ContainsRune("cdsr", m) {
			return nil, false
		}
	}
	search := string(expandTrList(n.SearchList))
	repl := string(expandTrList(n.ReplList))
	if search == "" {
		return nil, false
	}
	out := l.helperCall(hMapChars, ir.TString, text,
		ir.Str(quote(search)), ir.Str(quote(repl)),
		ir.BoolLit(complement), ir.BoolLit(del), ir.BoolLit(squeeze))
	l.note(out, "The modifiers on a transliteration change what it does rather than "+
		"how it is spelled, and the strings package has no call that covers them: "+
		"complementing the search list, deleting what has no replacement and "+
		"collapsing repeats are three separate decisions.",
		"strings-package", "strings-are-bytes")
	l.approximate(n, "P2G4520", "tr/// with modifiers",
		"the modifiers become arguments rather than syntax",
		"`"+n.Mods+"` says what to do with characters the search list matches. Go "+
			"has no transliteration operator at all, so the rules are passed to a "+
			"helper as the flags they are.",
		"Where only one modifier is in play, the direct Go is usually shorter: "+
			"strings.Map returning -1 deletes, and strings.NewReplacer replaces.",
		"strings-package")
	if ret {
		// The r modifier leaves the original alone and yields the result,
		// which is the one form that is an expression rather than a change.
		l.transValue = out
		return nil, true
	}
	st := assign("=", []ir.Expr{target}, []ir.Expr{l.assignable(out, typeOrAny(target), n)})
	l.setProv(st, n)
	return []ir.Stmt{st}, true
}

const (
	lowerAlphabet = "abcdefghijklmnopqrstuvwxyz"
	upperAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// transExpr lowers tr/// used for its value.
func (l *Lowerer) transExpr(n *ast.Trans) ir.Expr {
	// With an empty replacement list the value is the count, and the string
	// is untouched. That is the whole of the `my $n = ($s =~ tr/aeiou//)`
	// idiom, which is Perl's way of counting characters.
	if (n.Mods == "" || n.Mods == "c") && len(expandTrList(n.ReplList)) == 0 {
		search := expandTrList(n.SearchList)
		if len(search) > 0 {
			if target := l.transTarget(n); target != nil {
				helper, what := hCountChars, "in"
				if n.Mods == "c" {
					helper, what = hCountOther, "outside"
				}
				out := l.helperCall(helper, ir.TInt, l.toStr(target, nil), ir.Str(quote(string(search))))
				l.note(out, "tr with no replacement list counts rather than translates: "+
					"it reports how many characters of the string are "+what+" the search "+
					"list and leaves the string alone.")
				return out
			}
		}
	}
	saved := l.transValue
	l.transValue = nil
	for _, st := range l.transStmt(n) {
		l.emit(st)
	}
	if l.transValue != nil {
		out := l.transValue
		l.transValue = saved
		return out
	}
	l.transValue = saved
	if t := l.transTarget(n); t != nil {
		return t
	}
	return ir.Str(`""`)
}

func (l *Lowerer) transTarget(n *ast.Trans) ir.Expr {
	if n.Bound == nil {
		if len(l.topicStack) > 0 {
			return l.topicStack[len(l.topicStack)-1]
		}
		return l.assignTarget(&ast.Var{Sigil: '$', Name: "_"})
	}
	// `(my $copy = $original) =~ tr/.../.../` edits a copy and leaves the
	// original alone, exactly as the substitution form does: the assignment
	// becomes a statement of its own and the replacement names the copy.
	if a, ok := boundAssign(n.Bound); ok {
		for _, st := range l.assignStmts(a) {
			l.emit(st)
		}
		if v, single := soleDeclaredVar(a); single {
			return l.assignTarget(v)
		}
		return l.assignTarget(a.LHS)
	}
	return l.assignTarget(n.Bound)
}

// expandTrList turns a tr/// character list, which may contain ranges, into
// the characters it stands for.
func expandTrList(s string) []rune {
	var out []rune
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '\\' && i+1 < len(rs) {
			i++
			// A backslash escape names a character rather than quoting the
			// one after it: \t is a tab and not the letter t.
			switch rs[i] {
			case 't':
				out = append(out, '\t')
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 'f':
				out = append(out, '\f')
			case '0':
				out = append(out, 0)
			case 'e':
				out = append(out, 27)
			case 'a':
				out = append(out, 7)
			default:
				out = append(out, rs[i])
			}
			continue
		}
		if i+2 < len(rs) && rs[i+1] == '-' && rs[i+2] >= rs[i] {
			for c := rs[i]; c <= rs[i+2]; c++ {
				out = append(out, c)
			}
			i += 2
			continue
		}
		out = append(out, rs[i])
	}
	return out
}

var _ = strconv.Itoa

// markRefusedPattern attaches the refusal from a pattern that could not be
// translated to whatever stands in for it.
//
// The report always names the pattern, but a reader is looking at the code, and
// `if false {` says nothing at all about why. The marker puts the code and the
// reason in the same place, which is what the TODO in every other refusal does.
func (l *Lowerer) markRefusedPattern(x ir.Expr) ir.Expr {
	if l.patternTodo == nil {
		return x
	}
	ir.MetaOf(x).Todo = l.patternTodo
	l.patternTodo = nil
	return x
}

// refusedStmtMarker is markRefusedPattern for a whole statement that has
// nothing left to emit. Dropping the statement silently would leave the code
// reading as though the step never existed; the marker keeps the original
// text at the point of refusal and names the gap on stderr if the line is
// reached, which is what every other refused statement leaves behind.
func (l *Lowerer) refusedStmtMarker(n ast.Node) []ir.Stmt {
	if l.patternTodo == nil {
		return nil
	}
	todo := *l.patternTodo
	l.patternTodo = nil
	st := l.todoMarker(todo, n)
	st.Stub = l.helperCall(hNotImplementedHere, ir.TVoid,
		ir.Str(strconv.Quote(todo.Code)), ir.Str(strconv.Quote(todoWording(todo))))
	st.Info.Spelled = true
	return []ir.Stmt{st}
}

// matchGroups reports how many capture groups a match has, and what the named
// ones are called.
//
// A pattern written out has them in its own text. A pattern held in a variable
// recorded them when the qr// was assigned, because that is the only moment the
// converter can see what is inside it.
func (l *Lowerer) matchGroups(n *ast.Match) (int, map[string]int) {
	if n.Pattern != nil {
		return countGroups(n.Pattern.Raw), namedGroups(n.Pattern.Raw)
	}
	v, ok := n.PatternExpr.(*ast.Var)
	if !ok || v.Sigil != '$' {
		return 0, nil
	}
	if b, found := l.scope.lookup(varKey('$', v.Name)); found {
		return b.Groups, b.NamedGroups
	}
	if b, found := l.globalSeen[varKey('$', v.Name)]; found {
		return b.Groups, b.NamedGroups
	}
	return 0, nil
}

// normalizeMatch rewrites `/$re/` into the same shape as `$x =~ $re`.
//
// A pattern whose whole body is one interpolated scalar is that scalar's
// pattern, not a pattern to compile from the text `$re`. Compiling the text
// would produce a regular expression that matches an end of line followed by
// the letters r and e, which is the sort of wrong that never announces itself.
func normalizeMatch(n *ast.Match) {
	if n == nil || n.Pattern == nil || n.PatternExpr != nil {
		return
	}
	// A modifier applies to the pattern as written, and there is nowhere to
	// put it on an already-compiled one.
	if n.Pattern.Mods != "" {
		return
	}
	if len(n.Pattern.Parts) != 1 {
		return
	}
	v, ok := n.Pattern.Parts[0].(*ast.Var)
	if !ok || v.Sigil != '$' {
		return
	}
	n.PatternExpr = v
	n.Pattern = nil
}

// substGroups reports how many capture groups a substitution's pattern has,
// and what the named ones are called.
func (l *Lowerer) substGroups(n *ast.Subst) (int, map[string]int) {
	if n.Pattern == nil {
		return 0, nil
	}
	return countGroups(n.Pattern.Raw), namedGroups(n.Pattern.Raw)
}

// returnCaptures lowers `return $text =~ /a(b)c/`, where the sub is handing
// its caller the capture groups rather than a yes or no.
//
// This is the one place where Perl's context reaches across a function
// boundary. The parentheses that say "list" are at the call site, so the same
// sub yields two strings to `my ($h, $p) = pair($rec)` and a truth value to
// `if (pair($rec))`, and the match inside it never learns which. A Go
// function has one shape, and the shape that keeps both answers is the
// submatch slice: nil when the pattern did not match, which is false in the
// test and empty in the list, and the groups when it did.
func (l *Lowerer) returnCaptures(n *ast.Return, s *Sub) ([]ir.Stmt, bool) {
	if s == nil || len(n.Exprs) != 1 {
		return nil, false
	}
	m, ok := n.Exprs[0].(*ast.Match)
	if !ok || m.Negate {
		return nil, false
	}
	if m.Pattern != nil && strings.Contains(m.Pattern.Mods, "g") {
		// A global match already yields every match rather than the groups of
		// one, and the ordinary list path handles it.
		return nil, false
	}
	groups, _ := l.matchGroups(m)
	if groups == 0 {
		return nil, false
	}
	matchT := ir.SliceOf(ir.TString)

	depth := l.captureDepth()
	l.matchExpr(m, false)
	if l.captureDepth() <= depth {
		return nil, false
	}
	name := l.captureStack[len(l.captureStack)-1].Name
	l.restoreCaptures(depth)

	if l.pass == 1 {
		s.ResultEvidence = append(s.ResultEvidence, []*ir.Type{matchT})
	}
	found := ir.NewIdent(name, matchT)
	guard := &ir.If{
		Cond: ir.Bin("==", found, ir.Nil(matchT), ir.TBool),
		Then: &ir.Block{Stmts: []ir.Stmt{&ir.Return{Results: []ir.Expr{ir.Nil(matchT)}}}},
	}
	l.setProv(guard, n)
	l.note(guard, "A match that found nothing yields the empty list in Perl, so the "+
		"caller's variables are left undef and the same call read as a test is "+
		"false. A nil slice is both of those at once: len is 0, ranging over it "+
		"does nothing, and it is the zero value the caller would get anyway.",
		"nil-slices-vs-nil-maps", "context-is-gone")

	out := &ir.Return{Results: []ir.Expr{
		slicing(found, ir.IntLit("1"), nil, matchT),
	}}
	l.setProv(out, n)
	l.note(out, "Index 0 of the submatch slice is the whole match and the groups "+
		"start at 1, so the groups alone are the slice from 1 onwards. Perl put "+
		"them in $1 and $2, which are globals the caller could read afterwards; Go "+
		"has no such thing, so they are the return value.",
		"submatch-and-named-groups", "multiple-return-values")

	l.approximate(n, "P2G4512", "returning a match's captures",
		"the sub returns the groups, and a caller that wanted a truth value gets a list",
		"A Perl match yields its capture groups in list context and a truth value in "+
			"scalar context, and a sub returning one passes that choice on to its own "+
			"caller. A Go function has one shape. This one returns the groups, which is "+
			"the answer that keeps the information: a nil slice is still false and still "+
			"empty, so a caller testing it reads the same way.",
		"Where the caller only wanted to know whether it matched, `len(...) > 0` says "+
			"so; where it wanted the groups, they are already there.",
		"context-is-gone", "submatch-and-named-groups")
	return []ir.Stmt{guard, out}, true
}

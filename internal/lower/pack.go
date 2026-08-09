package lower

import (
	"strings"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file lowers pack and unpack.
//
// A pack template is a little language of its own, and the converter does not
// compile it: the template is nearly always written out at the call, so the
// generated program carries an interpreter for the common codes and hands it
// the template unchanged. What the converter does do is read a written-out
// template before emitting, so a template the interpreter cannot handle is
// refused here, at conversion time, rather than discovered when the program
// runs.

// packSupported is the set of template codes the emitted interpreter
// understands. It must match what packItems in the runtime sources accepts;
// TestPackTemplateSupportAgrees holds the two together.
const packSupported = "aAZxcCsSlLqQnNvVhH"

// packCodeNames names the template codes worth explaining when they are
// refused. Anything not listed is described generically.
var packCodeNames = map[byte]string{
	'b': "a bit string, low bit first",
	'B': "a bit string, high bit first",
	'f': "a native single-precision float",
	'd': "a native double-precision float",
	'w': "a BER compressed integer",
	'U': "a Unicode character number",
	'X': "a step backwards over the data",
	'@': "an absolute position in the data",
	'(': "a group",
	'<': "a byte-order modifier",
	'>': "a byte-order modifier",
	'!': "a native-size modifier",
	'/': "a length-prefixed field",
	'%': "a checksum",
}

// packTemplateIssue scans a written-out template for the first code the
// emitted interpreter has no rule for, returning that code and what it means.
// A template of supported codes returns ok.
func packTemplateIssue(template string) (code byte, meaning string, ok bool) {
	for i := 0; i < len(template); i++ {
		c := template[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n':
		case strings.IndexByte(packSupported, c) >= 0:
			for i+1 < len(template) &&
				(template[i+1] == '*' || (template[i+1] >= '0' && template[i+1] <= '9')) {
				i++
			}
			if i+1 < len(template) && strings.IndexByte("<>!", template[i+1]) >= 0 {
				return describePackCode(template, i+1)
			}
		default:
			return describePackCode(template, i)
		}
	}
	return 0, "", true
}

func describePackCode(template string, i int) (byte, string, bool) {
	c := template[i]
	meaning, known := packCodeNames[c]
	if !known {
		meaning = "a code the emitted interpreter has no rule for"
	}
	return c, meaning, false
}

// unpackKinds reads the value kinds a written-out unpack template produces,
// in order: one string per text code, one int per integer code repetition.
// It reports ok as false when a code is outside the supported set or a
// repeated integer code has no fixed count, since the positions after such
// a code cannot be known here.
func unpackKinds(template string) (kinds []*ir.Type, ok bool) {
	for i := 0; i < len(template); i++ {
		c := template[i]
		count, star := 0, false
		read := func() {
			for i+1 < len(template) {
				next := template[i+1]
				if next == '*' {
					star = true
					i++
					continue
				}
				if next >= '0' && next <= '9' {
					count = count*10 + int(next-'0')
					i++
					continue
				}
				break
			}
		}
		switch {
		case c == ' ' || c == '\t' || c == '\n':
		case strings.IndexByte("aAZhH", c) >= 0:
			read()
			kinds = append(kinds, ir.TString)
		case c == 'x':
			read()
		case strings.IndexByte("cCsSlLnNvV", c) >= 0:
			read()
			if star {
				return nil, false
			}
			if count == 0 {
				count = 1
			}
			for range count {
				kinds = append(kinds, ir.TInt)
			}
		default:
			// q and Q included: Q comes back a uint64, which is not a kind
			// the caller can fold into the others.
			return nil, false
		}
	}
	return kinds, true
}

// allStrings reports whether every kind in the list is text.
func allStrings(kinds []*ir.Type) bool {
	if len(kinds) == 0 {
		return false
	}
	for _, k := range kinds {
		if k.Kind != ir.String {
			return false
		}
	}
	return true
}

// unpackCall lowers `unpack TEMPLATE, DATA` to a call of the emitted template
// interpreter. The result is a list of values whose kinds only the template
// knows, so it comes back as a slice of any.
func (l *Lowerer) unpackCall(n *ast.Call) ir.Expr {
	return l.packLike(n, "unpack")
}

// packCall lowers `pack TEMPLATE, LIST` the same way, in the other direction.
func (l *Lowerer) packCall(n *ast.Call) ir.Expr {
	return l.packLike(n, "pack")
}

func (l *Lowerer) packLike(n *ast.Call, name string) ir.Expr {
	ret := ir.SliceOf(ir.TAny)
	helper := hUnpackTemplate
	if name == "pack" {
		ret = ir.TString
		helper = hPackTemplate
	}
	args := flatten(argList(n))
	if len(args) == 0 {
		if name == "pack" {
			return ir.Str(`""`)
		}
		return composite(ret, nil, nil)
	}

	if template, isStatic := staticString(args[0]); isStatic {
		if code, meaning, ok := packTemplateIssue(template); !ok {
			return l.todoExpr(n, "P2G5061", name+" template",
				"the `"+string(code)+"` template code is not in the emitted interpreter",
				"This template asks for `"+string(code)+"`, "+meaning+", and the "+
					"interpreter this program carries stops at the codes fixed-width "+
					"records actually use: text, hex, and whole-width integers.",
				"Translate this field by hand: encoding/binary reads and writes every "+
					"integer shape, math.Float64bits crosses to floats, and a bit string "+
					"is a loop over a byte's bits.",
				"encoding-binary", "strings-are-bytes")
		}
	} else {
		l.approximate(n, "P2G5062", name+" with a computed template",
			"the template is only known while the program runs",
			"The template string is built at run time, so nothing can check it "+
				"before then. A code outside the emitted interpreter's set stops the "+
				"program at this call, naming the template.",
			"Write the template out at the call, or check it against the codes "+
				"the interpreter documents before handing it over.",
			"encoding-binary")
	}

	template := l.argStr(n, 0)
	if name == "unpack" {
		// A written-out template says what each field is. When every code
		// produces text, the whole result is text, and saying so is the
		// difference between a []string and a slice of any.
		if tpl, isStatic := staticString(args[0]); isStatic {
			if kinds, ok := unpackKinds(tpl); ok && allStrings(kinds) {
				helper = hUnpackText
				ret = ir.SliceOf(ir.TString)
			}
		}
		out := l.helperCall(helper, ret, template, l.argStr(n, 1))
		l.explainPackLike(n, name, out)
		return out
	}

	parts, _ := l.listParts(args[1:])
	hasSlice := false
	for i, p := range parts {
		if t := typeOrAny(p); t.Kind == ir.Slice {
			hasSlice = true
			if t.Elem != nil && t.Elem.Kind != ir.Any {
				parts[i] = l.helperCall(hAnyList, ir.SliceOf(ir.TAny), p)
			}
		}
	}
	var out *ir.Call
	if hasSlice {
		out = l.helperCall(helper, ret, template, l.listValue(parts, ir.TAny))
		out.Ellipsis = true
	} else {
		for i, p := range parts {
			parts[i] = l.assignable(p, ir.TAny, n)
		}
		out = l.helperCall(helper, ret, append([]ir.Expr{template}, parts...)...)
	}
	l.explainPackLike(n, name, out)
	return out
}

// explainPackLike records what the template call became, once as a note on
// the expression and once in the report.
func (l *Lowerer) explainPackLike(n *ast.Call, name string, out ir.Expr) {
	l.note(out, "The template is a little language, and Go has nothing that reads "+
		"one: encoding/binary works an integer at a time with the byte order named "+
		"in the call, and a fixed-width text field is a slice expression. The "+
		"template interpreter emitted with this program keeps the record layout "+
		"in the one string the records were built around.",
		"encoding-binary", "strings-are-bytes")
	l.inform(n, "P2G5060", name,
		"`"+name+"` reads a template language Go does not have, so the program "+
			"carries an interpreter for the codes fixed-width records use and "+
			"hands it this template unchanged. Where the layout is stable, "+
			"encoding/binary and slice expressions say the same thing without "+
			"the interpreter, one field at a time.",
		"encoding-binary", "strings-are-bytes")
}

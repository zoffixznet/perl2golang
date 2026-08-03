package lower

import (
	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file covers the modules a script uses to turn data into bytes and back:
// JSON, checksums and base64.
//
// All three are in Go's standard library, and all three are functions rather
// than objects there. Where the Perl configured an encoder once and used it in
// several places, the generated code keeps a small value with the settings on
// it, so that the chain of option calls still reads the same way.

// jsonType and digestType are the generated types the two configured objects
// become.
func (l *Lowerer) jsonType() *ir.Type   { return ir.NamedType("*"+l.use(hJSONCodec), "") }
func (l *Lowerer) digestType() *ir.Type { return ir.NamedType("*"+l.use(hDigest), "") }

// formatConstructor lowers `JSON::PP->new` and `Digest::MD5->new`.
func (l *Lowerer) formatConstructor(n *ast.MethodCall, module, method string) (ir.Expr, bool) {
	if method != "new" {
		return nil, false
	}
	switch module {
	case "JSON::PP", "JSON", "JSON::XS":
		out := l.helperCall(hNewJSONCodec, l.jsonType())
		l.note(out, "encoding/json needs no object for the common case: Marshal and "+
			"Unmarshal are functions. This one exists because the settings were "+
			"chosen once here and used in several places.",
			"encoding-json")
		return out, true
	case "Digest::MD5", "Digest::SHA", "Digest":
		out := l.helperCall(hNewDigest, l.digestType())
		l.note(out, "A hash in Go is an io.Writer, so feeding it is Write and copying "+
			"a whole file into it is io.Copy. crypto/md5 is in the standard library, "+
			"and MD5 is not suitable for anything security-related: crypto/sha256 has "+
			"the same shape.")
		return out, true
	}
	return nil, false
}

// formatMethod lowers a method call on one of the configured objects, which
// are named types rather than classes the file declared.
func (l *Lowerer) formatMethod(n *ast.MethodCall, recv ir.Expr, method string) (ir.Expr, bool) {
	t := typeOrAny(recv)
	switch t.Name {
	case "*" + hJSONCodec:
		return l.jsonMethod(n, recv, method)
	case "*" + hDigest:
		return l.digestMethod(n, recv, method)
	}
	return nil, false
}

// jsonMethod maps one of the encoder's option or transfer calls.
func (l *Lowerer) jsonMethod(n *ast.MethodCall, recv ir.Expr, method string) (ir.Expr, bool) {
	args, _ := l.listParts(n.Args)
	on := ir.Expr(ir.BoolLit(true))
	if len(args) > 0 {
		on = l.toBool(args[0], n)
	}
	switch method {
	case "canonical", "sort_by":
		out := ir.CallOf(selector(recv, "Canonical", nil), l.jsonType(), on)
		l.approximate(n, "P2G7511", "canonical",
			"map keys are already written in sorted order",
			"json.Marshal walks a map's keys in sorted order, so output is stable "+
				"between runs without asking for it. The call is kept so the chain still "+
				"reads the same, and it does nothing.",
			"Drop the call. A struct is written in field order, which is stable too "+
				"and is usually what a document with a fixed shape wants.",
			"encoding-json", "map-iteration-order")
		return out, true
	case "pretty", "indent":
		out := ir.CallOf(selector(recv, "Pretty", nil), l.jsonType(), on)
		l.note(out, "json.MarshalIndent is the indented form, and the indent is an "+
			"argument rather than a setting on an encoder.",
			"encoding-json")
		return out, true
	case "ascii", "latin1", "utf8":
		out := ir.CallOf(selector(recv, "Ascii", nil), l.jsonType(), on)
		return out, true
	case "allow_nonref", "allow_blessed", "convert_blessed", "relaxed", "space_before", "space_after":
		l.approximate(n, "P2G7511", method,
			"this setting has no counterpart",
			"encoding/json has no switches for it: the encoder writes one shape and "+
				"the decoder accepts one grammar.",
			"Where the difference matters, write the value out yourself or post-process "+
				"the text.",
			"encoding-json")
		return recv, true
	case "encode":
		value := ir.Expr(ir.Nil(ir.TAny))
		if len(args) > 0 {
			value = l.assignable(args[0], ir.TAny, n)
		}
		out := ir.CallOf(selector(recv, "Encode", nil), ir.TString, value)
		l.approximate(n, "P2G7512", "encode",
			"the rendering differs in spacing and in what is escaped",
			"encoding/json escapes <, > and & by default, writes no space before a "+
				"colon, and indents with what you pass rather than with three spaces. "+
				"The values are the same; the bytes are not.",
			"Where two programs have to agree on the exact text, encode both sides "+
				"with the same library.",
			"encoding-json")
		return out, true
	case "decode":
		text := ir.Expr(ir.Str(`""`))
		if len(args) > 0 {
			text = l.toStr(args[0], n)
		}
		out := ir.CallOf(selector(recv, "Decode", nil), ir.TAny, text)
		l.approximate(n, "P2G7510", "decode",
			"every number comes back as a floating-point value",
			"json.Unmarshal into an `any` gives map[string]any, []any, string, bool "+
				"and float64. There is no integer: 3 and 3.0 are the same value coming "+
				"out, which shows up the first time one is printed.",
			"Decode into a declared struct instead. The fields say what the types "+
				"are, an int stays an int, and a missing field is a compile-time "+
				"question rather than a run-time one.",
			"encoding-json", "struct-tags")
		return out, true
	}
	return nil, false
}

// digestMethod maps one of the checksum's calls.
func (l *Lowerer) digestMethod(n *ast.MethodCall, recv ir.Expr, method string) (ir.Expr, bool) {
	args, _ := l.listParts(n.Args)
	switch method {
	case "add":
		parts := make([]ir.Expr, len(args))
		for i, a := range args {
			parts[i] = l.toStr(a, n)
		}
		return ir.CallOf(selector(recv, "Add", nil), l.digestType(), parts...), true
	case "addfile":
		handle := ir.Expr(ir.Nil(ir.TAny))
		if len(args) > 0 {
			handle = l.assertTo(args[0], ir.NamedType("*os.File", "os"))
		}
		out := ir.CallOf(selector(recv, "AddFile", nil), l.digestType(), handle)
		l.note(out, "A hash is an io.Writer, so hashing a file is io.Copy from the "+
			"file into the hash. There is no separate call for it, and the same code "+
			"works for anything else that can be read.",
			"io-reader-writer")
		return out, true
	case "hexdigest":
		return ir.CallOf(selector(recv, "HexDigest", nil), ir.TString), true
	case "b64digest":
		return ir.CallOf(selector(recv, "B64Digest", nil), ir.TString), true
	case "reset":
		return recv, true
	}
	return nil, false
}

// formatFunction lowers the plain functions the same modules export.
func (l *Lowerer) formatFunction(n *ast.Call) (ir.Expr, bool) {
	switch n.Name {
	case "md5_hex", "Digest::MD5::md5_hex":
		return l.helperCall(hHashHex, ir.TString, l.joinedArgs(n)), true
	case "md5_base64", "Digest::MD5::md5_base64":
		return l.helperCall(hHashBase64, ir.TString, l.joinedArgs(n)), true
	case "encode_base64", "MIME::Base64::encode_base64":
		eol := ir.Expr(ir.Str(`"\n"`))
		if argCount(n) > 1 {
			eol = l.argStr(n, 1)
		}
		out := l.helperCall(hToBase64, ir.TString, l.argStr(n, 0), eol)
		l.approximate(n, "P2G7570", "encode_base64",
			"the encoder writes one unbroken line",
			"encoding/base64 emits the whole encoding on one line. A mail encoder "+
				"wraps at 76 characters and ends with a line break, which is what the "+
				"helper adds.",
			"Where the wrapping does not matter, base64.StdEncoding.EncodeToString on "+
				"its own is the whole call.")
		return out, true
	case "decode_base64", "MIME::Base64::decode_base64":
		return l.helperCall(hFromBase64, ir.TString, l.argStr(n, 0)), true
	case "encode_json", "to_json", "JSON::PP::encode_json":
		out := ir.CallOf(selector(l.helperCall(hNewJSONCodec, l.jsonType()), "Encode", nil),
			ir.TString, l.assignable(l.argExpr(n, 0), ir.TAny, n))
		l.note(out, "json.Marshal returns the bytes and an error, and the bytes are "+
			"turned into text with a conversion. Ignoring the error is only safe "+
			"because a value built in this program cannot fail to encode.",
			"encoding-json", "errors-are-values")
		return out, true
	case "decode_json", "from_json", "JSON::PP::decode_json":
		out := ir.CallOf(selector(l.helperCall(hNewJSONCodec, l.jsonType()), "Decode", nil),
			ir.TAny, l.argStr(n, 0))
		l.approximate(n, "P2G7510", "decode_json",
			"every number comes back as a floating-point value",
			"Decoding into an `any` gives map[string]any, []any, string, bool and "+
				"float64, and no integer at all.",
			"Decode into a declared struct, where the fields say what the types are.",
			"encoding-json", "struct-tags")
		return out, true
	case "JSON::PP::true", "JSON::true":
		return ir.BoolLit(true), true
	case "JSON::PP::false", "JSON::false":
		return ir.BoolLit(false), true
	case "JSON::PP::null", "JSON::null":
		return ir.Nil(ir.TAny), true
	}
	return nil, false
}

// joinedArgs renders a call's arguments as the one string a checksum covers,
// which is what Perl's list-taking digest functions do.
func (l *Lowerer) joinedArgs(n *ast.Call) ir.Expr {
	args := flatten(argList(n))
	if len(args) == 0 {
		return ir.Str(`""`)
	}
	out := l.argStr(n, 0)
	for i := 1; i < len(args); i++ {
		out = ir.Bin("+", out, l.argStr(n, i), ir.TString)
	}
	return out
}

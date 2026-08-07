package src

import (
	"encoding/json"
	"sort"
	"strings"
)

// jsonCodec renders and parses JSON with the settings a program asked for.
//
// encoding/json needs no object at all for the common case: json.Marshal and
// json.Unmarshal are functions. This exists because the settings were chosen
// once and used in several places, and because two of them are already true
// here: map keys always come out sorted, and every number arrives as a
// float64 whatever it looked like in the text.
type jsonCodec struct {
	indent bool
	ascii  bool
}

// newJSONCodec returns a codec with the default settings.
func newJSONCodec() *jsonCodec { return &jsonCodec{} }

// Canonical is a no-op: json.Marshal already writes map keys in sorted order,
// so output is stable without asking.
func (c *jsonCodec) Canonical(bool) *jsonCodec { return c }

// Pretty turns on indented output.
func (c *jsonCodec) Pretty(on bool) *jsonCodec { c.indent = on; return c }

// Ascii escapes every character above U+007F.
func (c *jsonCodec) Ascii(on bool) *jsonCodec { c.ascii = on; return c }

// Encode renders a value as JSON text, ending in a newline when indented, as
// a document written for a person to read does.
//
// The indented form is written by hand rather than by MarshalIndent, because
// the readers of this output may be diffing it against documents written by
// other tools, and the traditional pretty style differs from Go's in one
// visible detail: a space on both sides of the colon, `"key" : value`.
func (c *jsonCodec) Encode(v any) string {
	if c.indent {
		var b strings.Builder
		c.write(&b, c.generic(v), "")
		b.WriteString("\n")
		return b.String()
	}
	out, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return c.unescape(string(out))
}

// unescape undoes the HTML-safety escapes json.Marshal applies, unless the
// codec was asked for ASCII-only output.
//
// json.Marshal escapes <, > and & so that the output is safe to drop into an
// HTML page. Most callers are not writing HTML and the escaping shows up as
// a difference nobody asked for; an Encoder with SetEscapeHTML(false) is the
// other way to turn it off.
func (c *jsonCodec) unescape(text string) string {
	if c.ascii {
		return text
	}
	return strings.NewReplacer(`\u003c`, "<", `\u003e`, ">", `\u0026`, "&").Replace(text)
}

// generic reduces a value to maps, slices and scalars, so the writer below
// has only those to walk: a struct in the data takes the same road it would
// have taken through Marshal, field tags included.
func (c *jsonCodec) generic(v any) any {
	switch v.(type) {
	case nil, bool, string, float64, int:
		return v
	}
	out, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var plain any
	if json.Unmarshal(out, &plain) != nil {
		return v
	}
	return plain
}

// write renders one value at the given indentation, three spaces per level.
func (c *jsonCodec) write(b *strings.Builder, v any, indent string) {
	inner := indent + "   "
	switch x := v.(type) {
	case map[string]any:
		if len(x) == 0 {
			b.WriteString("{}")
			return
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("{\n")
		for i, k := range keys {
			b.WriteString(inner)
			b.WriteString(c.scalar(k))
			b.WriteString(" : ")
			c.write(b, x[k], inner)
			if i < len(keys)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(indent)
		b.WriteString("}")
	case []any:
		if len(x) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[\n")
		for i, el := range x {
			b.WriteString(inner)
			c.write(b, el, inner)
			if i < len(x)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(indent)
		b.WriteString("]")
	default:
		b.WriteString(c.scalar(v))
	}
}

// scalar renders one leaf value the way the compact form would.
func (c *jsonCodec) scalar(v any) string {
	out, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return c.unescape(string(out))
}

// Decode parses JSON text into maps, slices, strings, float64s and bools.
func (c *jsonCodec) Decode(text string) any {
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return nil
	}
	return v
}

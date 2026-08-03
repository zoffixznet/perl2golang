package src

import (
	"encoding/json"
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
func (c *jsonCodec) Encode(v any) string {
	var out []byte
	var err error
	if c.indent {
		out, err = json.MarshalIndent(v, "", "   ")
	} else {
		out, err = json.Marshal(v)
	}
	if err != nil {
		return ""
	}
	text := string(out)
	if !c.ascii {
		// json.Marshal escapes these three so that the output is safe to
		// drop into an HTML page. Most callers are not writing HTML and the
		// escaping shows up as a difference nobody asked for; an Encoder
		// with SetEscapeHTML(false) is the other way to turn it off.
		text = strings.NewReplacer(`\u003c`, "<", `\u003e`, ">", `\u0026`, "&").Replace(text)
	}
	if c.indent {
		text += "\n"
	}
	return text
}

// Decode parses JSON text into maps, slices, strings, float64s and bools.
func (c *jsonCodec) Decode(text string) any {
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return nil
	}
	return v
}

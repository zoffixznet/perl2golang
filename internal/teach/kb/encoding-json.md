---
id: encoding-json
title: JSON maps onto types, not onto hashes
tags: [idiom, json, serialisation, stdlib]
perl_triggers: [encode-json, decode-json, json-pp, json-maybexs, json-xs, to-json, json-canonical, json-pretty, json-boolean]
severity: info
prerequisites: [struct-tags, type-assertions-and-switches]
---

`decode_json` hands you a hashref and you start digging; `json.Unmarshal` wants to know, in advance, what shape the data has. Declaring a struct for the document is the idiomatic path and it pays immediately: field names, types, and optionality become compile-checked, and misspelling a key is a build error instead of an `undef` three functions later. The escape hatch — decoding into `map[string]any` — exists for genuinely unknown shapes, and it is deliberately unpleasant to use, because every access needs a type assertion (`type-assertions-and-switches`).

## The Perl you know

```perl
use JSON::MaybeXS;
my $data = decode_json($body);
my $port = $data->{server}{port};        # a typo here is silent undef
my $json = encode_json({ name => "Jane Doe", tags => [] });
```

Everything is a hashref, arrayref, or scalar; numbers and strings blur (`1` may encode as `"1"` depending on how the scalar was last used), and key order is whatever the hash gives you.

## The Go you write

```go
package main

import (
	"encoding/json"
	"fmt"
)

type Config struct {
	Name   string   `json:"name"`
	Server Server   `json:"server"`
	Tags   []string `json:"tags"`
	Debug  *bool    `json:"debug,omitempty"` // pointer: absent and false differ
}

type Server struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

func main() {
	input := []byte(`{"name":"payroll","server":{"host":"db1","port":5432},
	                  "tags":["prod","eu"],"extra":"ignored"}`)

	var cfg Config
	if err := json.Unmarshal(input, &cfg); err != nil { // note the &: it must be a pointer
		fmt.Println("bad config:", err)
		return
	}
	fmt.Printf("%+v\n", cfg)
	fmt.Println(cfg.Server.Port, cfg.Debug == nil)

	out, err := json.Marshal(cfg)
	fmt.Println(string(out), err)

	// Unknown shape: decode into any, then assert your way down.
	var loose map[string]any
	if err := json.Unmarshal(input, &loose); err != nil {
		fmt.Println(err)
		return
	}
	server, ok := loose["server"].(map[string]any)
	fmt.Println(ok)
	port, ok := server["port"].(float64) // every JSON number is a float64 here
	fmt.Println(port, ok)

	// A type error is reported, not tolerated.
	var wrong struct {
		Name int `json:"name"`
	}
	fmt.Println(json.Unmarshal(input, &wrong))
}
```

```
{Name:payroll Server:{Host:db1 Port:5432} Tags:[prod eu] Debug:<nil>}
5432 true
{"name":"payroll","server":{"host":"db1","port":5432},"tags":["prod","eu"]} <nil>
true
5432 true
json: cannot unmarshal string into Go struct field .name of type int
```

Unknown input fields (`extra`) are ignored by default, missing input fields leave the zero value in place, and the round trip dropped `debug` entirely thanks to `omitempty` on a nil pointer.

## The mismatch

The rules that catch Perl programmers, in the order they catch them. Only *exported* fields are marshalled — a lower-case field is invisible to the encoder and produces `{}` with no diagnostic (`packages-and-exported-names`, `struct-tags`). `Unmarshal` needs the address of the destination (`&cfg`); passing the value gives you the `json: Unmarshal(non-pointer ...)` error, which is a runtime, not compile-time, failure. Numbers decoded into `any` are always `float64` — a 64-bit database ID will lose precision, so decode into a struct with an `int64` field, or use `json.NewDecoder(r).UseNumber()` to get `json.Number` (a string you parse deliberately). A nil slice encodes as `null` while an empty slice encodes as `[]`, which is a real API contract difference JavaScript clients notice (`nil-slices-vs-nil-maps`); initialise with `[]T{}` when the wire form matters. Map keys are emitted in sorted order, so Go's map randomisation (`map-iteration-order`) never leaks into JSON output — one place you get determinism for free. Beyond the basics: `json.MarshalIndent` is the pretty-printer, `json.NewDecoder(r)`/`NewEncoder(w)` stream from any reader or writer instead of buffering whole documents (`io-reader-writer`), `DisallowUnknownFields()` turns the tolerant default into strict validation, and a type that implements `MarshalJSON`/`UnmarshalJSON` controls its own representation — the `TO_JSON` equivalent, but type-directed and never optional at the call site. Times are handled: `time.Time` marshals as RFC 3339, no `DateTime::Format::*` decision required (`time-layouts`).

## Two differences in the bytes, not in the values

Once the data survives the round trip, two things about the *text* still catch people out, and both are silent.

`json.Marshal` escapes `<`, `>` and `&` as their `\u00xx` forms. It does that so its output can be dropped into an HTML page without becoming a script injection, and it does it whether or not you are writing HTML. A name like `widget <&> gadget` comes out unreadable and no longer byte-compares with what another encoder produced. Turning it off needs the streaming form, because the function form has no switch:

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func plain(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.Encode(v) // note: Encode appends a newline, Marshal does not
	return b.String()
}

func main() {
	out, _ := json.Marshal(map[string]string{"name": "widget <&> gadget"})
	fmt.Print(string(out), "\n")
	fmt.Print(plain(map[string]string{"name": "widget <&> gadget"}))
}
```

```
{"name":"widget \u003c\u0026\u003e gadget"}
{"name":"widget <&> gadget"}
```

And `MarshalIndent` is not the same pretty-printer other languages ship. Go writes `"a": 1`; Perl's `JSON::PP` in pretty mode writes `"a" : 1`, with a space before the colon and three spaces of indent. Neither is wrong and neither is configurable in Go. If two programs have to produce identical text, encode both sides with the same library; if only the *values* have to match, compare after decoding rather than comparing the text.

A third thing is worth knowing before it bites: `Encoder.Encode` writes a trailing newline and `json.Marshal` does not. That is deliberate, because the streaming form is designed for one document per line, and it is the usual reason a golden-file test fails by exactly one byte.

Further reading: https://pkg.go.dev/encoding/json and https://go.dev/blog/json

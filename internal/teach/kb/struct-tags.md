---
id: struct-tags
title: Struct tags - metadata strings that drive serialisation
tags: [idiom, types, structs, json, tags]
perl_triggers: [json-encode-object, to-json, dbix-class-column, moosex-attribute]
severity: info
prerequisites: [structs-and-embedding, packages-and-exported-names]
---

The backtick string after a struct field — `` `json:"name,omitempty"` `` — is a *struct tag*: machine-readable metadata that libraries read via reflection to decide how to serialise, validate, or map that field. There is no Perl analogue because Perl serialisers just walk your hashref's keys; Go serialisers see a typed struct and need you to say what the wire names are. Two facts here generate real bugs: unexported (lower-case) fields are invisible to every serialiser no matter what you tag them, and a typo inside a tag is just a string — the compiler cannot check it (though `go vet` catches common malformations).

## The Perl you know

```perl
use JSON::PP;
my $user = { name => "Jane Doe", email => "", api_token => "secret" };
print encode_json($user);
# {"name":"Jane Doe","email":"","api_token":"secret"}  — every key, always
```

The hash key *is* the wire name; hiding or renaming means restructuring the data or writing `TO_JSON` by hand.

## The Go you write

Compiled and run as shown:

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type User struct {
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	APIToken string `json:"-"`
	internal int    // unexported: encoding/json cannot see it at all
}

func main() {
	u := User{Name: "Jane Doe", APIToken: "secret", internal: 7}
	out, err := json.Marshal(u)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}
```

```
{"name":"Jane Doe"}
```

One line of output, four lessons: `Name` was renamed to lower-case `name` on the wire; `Email` vanished because it was empty and tagged `omitempty`; `APIToken` was deliberately excluded with `json:"-"`; and `internal` never had a chance, because reflection cannot read unexported fields.

## The mismatch

The struct-tags mechanism generalises far beyond JSON — `xml:`, `yaml:`, `db:`, `validate:` tags all share the same syntax (space-separated `key:"value"` pairs in one backtick string) and the same reflection API — so tags are the Go answer to a whole family of Perl attribute systems, from `MooseX` traits to DBIx::Class column info. The traps to respect: forgetting a tag does not fail, it silently uses the Go field name (`Name` marshals as `"Name"`, and your API consumers get PascalCase keys); tagging an unexported field does nothing, and the compiler will not tell you (the field is skipped, which reads as data loss at the far end — see `packages-and-exported-names`); and `omitempty` means "zero value", so an account balance of `0` or an explicit `false` also disappears — when zero-but-present matters, use a pointer field (`*int`, `*bool`) and revisit `nil-vs-undef`. Full details of tag-to-JSON behaviour live in the `encoding/json` docs (see also `encoding-json`).

Further reading: https://pkg.go.dev/encoding/json#Marshal and https://go.dev/ref/spec#Struct_types

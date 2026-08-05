---
id: encoding-binary
title: Binary and fixed-width data, without the template
tags: [binary, encoding, stdlib, idiom]
perl_triggers: [pack, unpack, vec, sysread-binary]
severity: info
prerequisites: [strings-are-bytes, structs-and-embedding]
---

`pack` and `unpack` are one function each, driven by a template string that is a little language: `a3 N n4` says three raw bytes, a big-endian 32, four big-endian 16s. Go has no counterpart to the template. It splits the job into parts you already know: a fixed-width text field is a slice expression, an integer field is one call into `encoding/binary` with the byte order written at the call, and a whole record can be read in one go by declaring a struct — the struct *is* the template, and unlike the string it is checked by the compiler.

## The Perl you know

```perl
# fixed-width text
my ($tag, $seq, $type, $desc, $cents) = unpack 'a3 A6 A4 A20 A10', $line;

# binary, big-endian on the wire
my $rec = pack 'a3 n N', 'TXN', $seq, $cents;
my ($t, $s, $c) = unpack 'a3 n N', $rec;
```

The template carries the whole layout in one string, `A` quietly strips the padding, and the native-width codes (`s`, `l`, `q`) mean whatever the machine that wrote the file meant — which is exactly the property that makes files written on one box unreadable on another.

## The Go you write

```go
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

// One fixed-width text record: tag, sequence, type, description, cents.
//   TXN 000017 DEP payroll deposit      0000012500
func parseTxn(line string) (tag, typ, desc string, seq, cents int) {
	tag = line[0:3]
	seq, _ = strconv.Atoi(line[3:9]) // zero-padded, still base ten
	typ = strings.TrimRight(line[9:13], " ")
	desc = strings.TrimRight(line[13:33], " ")
	cents, _ = strconv.Atoi(line[33:43])
	return
}

// The same record as binary: a 3-byte tag, then big-endian integers. The
// struct is the template, and the compiler checks every use of it.
type txnHeader struct {
	Tag   [3]byte
	Seq   uint16
	Cents uint32
}

func main() {
	tag, typ, desc, seq, cents := parseTxn("TXN000017DEP payroll deposit     0000012500")
	fmt.Printf("%s #%d %s %q %d.%02d\n", tag, seq, typ, desc, cents/100, cents%100)

	// Writing binary: the byte order is named at every call.
	var rec []byte
	rec = append(rec, "TXN"...)
	rec = binary.BigEndian.AppendUint16(rec, 17)
	rec = binary.BigEndian.AppendUint32(rec, 12500)
	fmt.Printf("% x\n", rec)

	// Reading it back, one field at a time.
	seq2 := binary.BigEndian.Uint16(rec[3:5])
	cents2 := binary.BigEndian.Uint32(rec[5:9])
	fmt.Println(seq2, cents2)

	// Or in one call, struct-directed.
	var h txnHeader
	if err := binary.Read(bytes.NewReader(rec), binary.BigEndian, &h); err != nil {
		fmt.Println("short record:", err)
		return
	}
	fmt.Printf("%s %d %d\n", h.Tag, h.Seq, h.Cents)
}
```

```
TXN #17 DEP "payroll deposit" 125.00
54 58 4e 00 11 00 00 30 d4
17 12500
TXN 17 12500
```

The slice expressions in `parseTxn` are byte offsets, which for a fixed-width file is precisely what you want (`strings-are-bytes`), and `strings.TrimRight(field, " ")` is the `A` code's padding rule said out loud. On the binary side, every call names its byte order; nothing is ever "native" unless you write `binary.NativeEndian`, and writing it is a signal to the reader that the format is tied to one kind of machine.

## The mismatch

The byte order is the big one. Perl's native codes made the machine's endianness an invisible default, and a program could work for years before meeting a file from the other kind of machine. Go makes the order part of every call, so the decision is in the code and in the diff. Second, a slice expression panics where `unpack` shrugged: `line[33:43]` on a 40-byte line stops the program, so length-check lines that come from outside (`len(line) >= 43`) before slicing them — the crash is loud where Perl's quiet empty fields let short records slip through as zeros. Third, `binary.Read` fills exported fields in declaration order with no padding or alignment, which is almost always what a wire format wants but is *not* what a C compiler laid out in a struct with mixed field sizes; for reading C structs, add explicit padding fields. Fourth, reading an exact number of bytes from a stream is `io.ReadFull`, not `Read` — `Read` may return fewer bytes than the buffer holds and no error, which works on your test file and fails on a pipe. And where the record layout genuinely lives in data — templates read from a config, say — there is no shame in an interpreter: that is what the `unpackTemplate` helper emitted with converted programs is, a documented loop over the codes, and reading it is a fair tour of everything above.

Further reading: https://pkg.go.dev/encoding/binary and https://pkg.go.dev/io#ReadFull

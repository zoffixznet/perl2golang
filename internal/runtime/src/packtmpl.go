package src

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// packItem is one code of a pack or unpack template: a letter, and either a
// repeat count or a star meaning "however much is left".
type packItem struct {
	code  byte
	count int
	star  bool
}

// packItems reads a template like "a3 N n4 Z* H8" into its codes. Whitespace
// separates codes and means nothing. A code with no count means one.
//
// Only the codes the two template functions below understand come back;
// anything else stops the program with a message naming the code. The codes
// with no counterpart here are the bit strings (b, B), the floats (f, d),
// the BER integers (w), and the positioning codes (X, @), along with every
// grouping and modifier form.
func packItems(template string) []packItem {
	var items []packItem
	for i := 0; i < len(template); i++ {
		c := template[i]
		if c == ' ' || c == '\t' || c == '\n' {
			continue
		}
		if strings.IndexByte("aAZxcCsSlLqQnNvVhH", c) < 0 {
			panic(fmt.Sprintf("pack template %q: the %q code is not supported", template, string(c)))
		}
		item := packItem{code: c, count: 1}
		if i+1 < len(template) {
			switch next := template[i+1]; {
			case next == '*':
				item.star = true
				i++
			case next >= '0' && next <= '9':
				n := 0
				for i+1 < len(template) && template[i+1] >= '0' && template[i+1] <= '9' {
					n = n*10 + int(template[i+1]-'0')
					i++
				}
				item.count = n
			case next == '<' || next == '>' || next == '!':
				panic(fmt.Sprintf("pack template %q: the %q modifier is not supported", template, string(next)))
			}
		}
		items = append(items, item)
	}
	return items
}

// packSize is how many bytes each fixed-width numeric code occupies.
var packSize = map[byte]int{
	'c': 1, 'C': 1,
	's': 2, 'S': 2, 'n': 2, 'v': 2,
	'l': 4, 'L': 4, 'N': 4, 'V': 4,
	'q': 8, 'Q': 8,
}

// unpackText reads fields out of data with a template whose every code
// produces text, which is what a fixed-width record of columns uses. It is
// unpackTemplate with the result kept as the strings it already is, so the
// fields need no conversion on the way out.
func unpackText(template, data string) []string {
	fields := unpackTemplate(template, data)
	out := make([]string, len(fields))
	for i, f := range fields {
		s, _ := f.(string)
		out[i] = s
	}
	return out
}

// unpackTemplate reads fields out of data: the template says what each
// stretch of bytes is, and the result holds one value per field, in order.
//
// The text codes take a length: "a" keeps the bytes as they are, "A" also
// drops trailing spaces and NULs (the padding a fixed-width record carries),
// and "Z" stops at the first NUL. "H" and "h" read hex digits, high or low
// nibble first, and their count is digits rather than bytes. "x" skips a
// byte. The integer codes each take a fixed number of bytes: "c", "s", "l"
// and "q" are signed 8, 16, 32 and 64 bits in this machine's byte order, "C",
// "S", "L" and "Q" their unsigned forms, "n" and "N" unsigned 16 and 32 bits
// big-endian, "v" and "V" the same little-endian. A count on an integer code
// repeats it, one value per repetition. A star means the rest of the data.
//
// Data that runs out early is not an error: text shorter than its count is
// taken as it is, and an integer field with too few bytes left produces
// nothing. Integers come back as int, except "Q", whose whole range only
// fits a uint64.
//
// New Go code reads binary data with encoding/binary, and a fixed-width text
// record is plain slicing: line[0:3], line[3:11]. This function earns its
// place only while the template strings are still the ones the original
// records were written with.
func unpackTemplate(template, data string) []any {
	var out []any
	p := 0
	for _, item := range packItems(template) {
		switch item.code {
		case 'a', 'A', 'Z':
			n := item.count
			if item.star || n > len(data)-p {
				n = len(data) - p
			}
			if item.code == 'Z' && item.star {
				// A star-sized Z field ends at its NUL, not at the end of
				// the data, so the codes after it read on from there.
				if i := strings.IndexByte(data[p:], 0); i >= 0 {
					n = i + 1
				}
			}
			field := data[p : p+n]
			p += n
			switch item.code {
			case 'A':
				field = strings.TrimRight(field, " \x00")
			case 'Z':
				if i := strings.IndexByte(field, 0); i >= 0 {
					field = field[:i]
				}
			}
			out = append(out, field)
		case 'x':
			n := item.count
			if item.star {
				n = len(data) - p
			}
			if n > len(data)-p {
				panic(fmt.Sprintf("unpack %q: x outside of data", template))
			}
			p += n
		case 'h', 'H':
			n := item.count
			if item.star || n > 2*(len(data)-p) {
				n = 2 * (len(data) - p)
			}
			hex := make([]byte, n)
			for i := range hex {
				b := data[p+i/2]
				if (i%2 == 0) == (item.code == 'H') {
					b >>= 4
				}
				hex[i] = "0123456789abcdef"[b&0x0f]
			}
			p += (n + 1) / 2
			out = append(out, string(hex))
		default:
			size := packSize[item.code]
			reps := item.count
			if item.star {
				reps = (len(data) - p) / size
			}
			for ; reps > 0 && size <= len(data)-p; reps-- {
				out = append(out, unpackInt(item.code, data[p:p+size]))
				p += size
			}
		}
	}
	return out
}

// unpackInt decodes one integer field. The unsigned codes widen as they are,
// and the signed ones sign-extend from their own width first.
func unpackInt(code byte, b string) any {
	native := binary.NativeEndian
	switch code {
	case 'c':
		return int(int8(b[0]))
	case 'C':
		return int(b[0])
	case 's':
		return int(int16(native.Uint16([]byte(b))))
	case 'S':
		return int(native.Uint16([]byte(b)))
	case 'n':
		return int(binary.BigEndian.Uint16([]byte(b)))
	case 'v':
		return int(binary.LittleEndian.Uint16([]byte(b)))
	case 'l':
		return int(int32(native.Uint32([]byte(b))))
	case 'L':
		return int(native.Uint32([]byte(b)))
	case 'N':
		return int(binary.BigEndian.Uint32([]byte(b)))
	case 'V':
		return int(binary.LittleEndian.Uint32([]byte(b)))
	case 'q':
		return int(native.Uint64([]byte(b)))
	}
	return native.Uint64([]byte(b)) // Q
}

// packTemplate builds a string of bytes: each template code takes values
// from args and lays them down in order.
//
// The text codes pad or truncate one value to their count: "A" pads with
// spaces, "a" with NULs, and "Z" with NULs while always keeping one NUL at
// the end, so "Z3" holds two bytes of text at most. A star takes the value's
// own length. "H" and "h" write a string of hex digits into bytes, high or
// low nibble first. "x" writes a NUL byte and takes no value. The integer
// codes are the same widths and byte orders unpackTemplate reads, a count
// repeats a code, taking one value per repetition, and a star takes every
// value that is left. Values past the end of args are the empty string and
// zero, and an integer too wide for its field keeps its low bits and drops
// the rest, the way the records these templates describe were written.
func packTemplate(template string, args ...any) string {
	var out []byte
	next := 0
	take := func() any {
		if next >= len(args) {
			return nil
		}
		next++
		return args[next-1]
	}
	for _, item := range packItems(template) {
		switch item.code {
		case 'a', 'A', 'Z':
			field := toText(take())
			n := item.count
			if item.code == 'Z' && !item.star {
				// The terminating NUL lives inside the count.
				n--
			}
			if item.star {
				n = len(field)
			}
			if len(field) > n {
				field = field[:n]
			}
			pad := byte(0)
			if item.code == 'A' {
				pad = ' '
			}
			out = append(out, field...)
			for i := len(field); i < n; i++ {
				out = append(out, pad)
			}
			if item.code == 'Z' {
				out = append(out, 0)
			}
		case 'x':
			for i := 0; i < item.count; i++ {
				out = append(out, 0)
			}
		case 'h', 'H':
			field := toText(take())
			n := item.count
			if item.star {
				n = len(field)
			}
			if n > len(field) {
				n = len(field)
			}
			for i := 0; i < n; i += 2 {
				hi := hexNibble(field[i])
				lo := byte(0)
				if i+1 < n {
					lo = hexNibble(field[i+1])
				}
				if item.code == 'h' {
					hi, lo = lo, hi
				}
				out = append(out, hi<<4|lo)
			}
		default:
			reps := item.count
			if item.star {
				reps = len(args) - next
			}
			for ; reps > 0; reps-- {
				out = packInt(out, item.code, take())
			}
		}
	}
	return string(out)
}

// hexNibble is the value of one hex digit. A stray character counts as
// zero rather than stopping the packing.
func hexNibble(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

// packInt lays down one integer field. The value arrives as whatever the
// program had; a genuine integer keeps every bit, and anything else goes
// through the usual numeric reading first.
func packInt(out []byte, code byte, v any) []byte {
	var u uint64
	switch n := v.(type) {
	case int:
		u = uint64(n)
	case int64:
		u = uint64(n)
	case uint64:
		u = n
	default:
		u = uint64(int64(toNum(v)))
	}
	native := binary.NativeEndian
	switch code {
	case 'c', 'C':
		return append(out, byte(u))
	case 's', 'S':
		return native.AppendUint16(out, uint16(u))
	case 'n':
		return binary.BigEndian.AppendUint16(out, uint16(u))
	case 'v':
		return binary.LittleEndian.AppendUint16(out, uint16(u))
	case 'l', 'L':
		return native.AppendUint32(out, uint32(u))
	case 'N':
		return binary.BigEndian.AppendUint32(out, uint32(u))
	case 'V':
		return binary.LittleEndian.AppendUint32(out, uint32(u))
	}
	return native.AppendUint64(out, u) // q, Q
}

package src

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"hash"
	"io"
	"os"
	"strings"
)

// digest accumulates bytes and reports a checksum over all of them.
//
// The standard library's hash.Hash is the same idea with better names: Write
// takes the bytes, because a hash is an io.Writer, and Sum returns them.
type digest struct{ h hash.Hash }

// newDigest starts a checksum with nothing in it yet.
func newDigest() *digest { return &digest{h: md5.New()} }

// Add appends text to what the checksum covers.
func (d *digest) Add(parts ...string) *digest {
	for _, p := range parts {
		io.WriteString(d.h, p)
	}
	return d
}

// AddFile appends everything an open file still holds.
func (d *digest) AddFile(f *os.File) *digest {
	if f != nil {
		io.Copy(d.h, f)
	}
	return d
}

// HexDigest is the checksum as lower-case hexadecimal, and it empties the
// accumulator as a side effect, which is what the caller expects.
func (d *digest) HexDigest() string {
	sum := hex.EncodeToString(d.h.Sum(nil))
	d.h.Reset()
	return sum
}

// B64Digest is the checksum in base64 with the padding removed.
func (d *digest) B64Digest() string {
	sum := base64.StdEncoding.EncodeToString(d.h.Sum(nil))
	d.h.Reset()
	return strings.TrimRight(sum, "=")
}

// hashHex is the checksum of one string, as hexadecimal.
func hashHex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// hashBase64 is the checksum of one string, in base64 without padding.
func hashBase64(s string) string {
	sum := md5.Sum([]byte(s))
	return strings.TrimRight(base64.StdEncoding.EncodeToString(sum[:]), "=")
}

// toBase64 encodes text as base64, wrapped at 76 characters with a trailing
// newline, which is what a mail encoder produces.
func toBase64(s, eol string) string {
	if eol == "\n" || eol == "" {
		eol = "\n"
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(s))
	var b strings.Builder
	for len(encoded) > 76 {
		b.WriteString(encoded[:76])
		b.WriteString(eol)
		encoded = encoded[76:]
	}
	if encoded != "" {
		b.WriteString(encoded)
		b.WriteString(eol)
	}
	return b.String()
}

// fromBase64 decodes base64 text, ignoring the line breaks an encoder added.
func fromBase64(s string) string {
	clean := strings.NewReplacer("\n", "", "\r", "", " ", "", "\t", "").Replace(s)
	out, err := base64.StdEncoding.WithPadding(base64.NoPadding).DecodeString(strings.TrimRight(clean, "="))
	if err != nil {
		return ""
	}
	return string(out)
}

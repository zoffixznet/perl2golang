// The expectations here were taken from perl 5.42.2: each case was run
// through the equivalent pack or unpack one-liner and the answers copied
// byte for byte.
package src

import (
	"reflect"
	"testing"
)

func TestUnpackTemplate(t *testing.T) {
	tests := []struct {
		template string
		data     string
		want     []any
	}{
		{"a3 A8 A10", "TXN000123DEP ", []any{"TXN", "000123DE", "P"}},
		{"A4 A2 A2", "20240705", []any{"2024", "07", "05"}},
		{"a3 A6 A4 A20 A10", "TXN000001DEP payroll deposit     0000012500",
			[]any{"TXN", "000001", "DEP", "payroll deposit", "0000012500"}},
		{"n N v V", "\x01\x02\x01\x02\x03\x04\x01\x02\x01\x02\x03\x04",
			[]any{258, 16909060, 513, 67305985}},
		{"c C", "\xff\xff", []any{-1, 255}},
		{"C*", "abc", []any{97, 98, 99}},
		{"N2", "\x00\x00\x00\x01", []any{1}}, // data runs out: one value, not two
		{"a2 x2 a2", "abXXcd", []any{"ab", "cd"}},
		{"Z* a2", "ab\x00cd", []any{"ab", "cd"}}, // Z* ends at its NUL
		{"Z8", "ab\x00cd   ", []any{"ab"}},
		{"A8", "ab", []any{"ab"}}, // short text is taken as it is
		{"A8", "ab  \x00\x00  ", []any{"ab"}},
		{"a8", "ab  \x00\x00  ", []any{"ab  \x00\x00  "}},
		{"H6 h4", "ABCde", []any{"414243", "4656"}},
		{"H*", "ABC", []any{"414243"}},
		{"H3", "AB", []any{"414"}},
		{"x* a2", "abcd", []any{""}},
		{"", "abc", nil},
	}
	for _, tt := range tests {
		got := unpackTemplate(tt.template, tt.data)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("unpackTemplate(%q, %q) = %#v, want %#v", tt.template, tt.data, got, tt.want)
		}
	}
}

func TestUnpackText(t *testing.T) {
	got := unpackText("a3 A8 A10", "HDR20240601Main St   ")
	want := []string{"HDR", "20240601", "Main St"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unpackText = %#v, want %#v", got, want)
	}
	if got := unpackText("", "abc"); len(got) != 0 {
		t.Errorf("unpackText on an empty template = %#v, want none", got)
	}
}

func TestUnpackTemplateRoundTrip(t *testing.T) {
	// The native-order codes have no fixed byte expectations, so they are
	// checked the way they are used: whatever pack lays down, unpack reads
	// back.
	packed := packTemplate("s l q", -2, -70000, -5000000000)
	if got, want := unpackTemplate("s l q", packed), []any{-2, -70000, -5000000000}; !reflect.DeepEqual(got, want) {
		t.Errorf("signed round trip = %#v, want %#v", got, want)
	}
	packed = packTemplate("S L Q", 65534, 4294967294, uint64(18446744073709551615))
	if got, want := unpackTemplate("S L Q", packed), []any{65534, 4294967294, uint64(18446744073709551615)}; !reflect.DeepEqual(got, want) {
		t.Errorf("unsigned round trip = %#v, want %#v", got, want)
	}
}

func TestPackTemplate(t *testing.T) {
	tests := []struct {
		template string
		args     []any
		want     string
	}{
		{"A6", []any{"abc"}, "abc   "},
		{"a6", []any{"abc"}, "abc\x00\x00\x00"},
		{"A3", []any{"abcdef"}, "abc"},
		{"Z5", []any{"abc"}, "abc\x00\x00"},
		{"Z3", []any{"abcdef"}, "ab\x00"}, // the NUL lives inside the count
		{"Z*", []any{"abc"}, "abc\x00"},
		{"A*", []any{"hello"}, "hello"},
		{"n N v V", []any{258, 16909060, 513, 67305985},
			"\x01\x02\x01\x02\x03\x04\x01\x02\x01\x02\x03\x04"},
		{"C4", []any{97, 98, 300, -1}, "ab\x2c\xff"}, // too wide wraps, as in perl
		{"C*", []any{1, 2, 3}, "\x01\x02\x03"},
		{"H4", []any{"4142"}, "AB"},
		{"H3", []any{"abc"}, "\xab\xc0"},
		{"A2 x A2", []any{"ab", "cd"}, "ab\x00cd"},
		{"N2", []any{5}, "\x00\x00\x00\x05\x00\x00\x00\x00"}, // missing values are zero
		{"a3 A8 A10", []any{"HDR", "20240705", "EASTSIDE"}, "HDR20240705EASTSIDE  "},
		{"A4", []any{7.5}, "7.5 "}, // values are texted the way perl texts them
	}
	for _, tt := range tests {
		got := packTemplate(tt.template, tt.args...)
		if got != tt.want {
			t.Errorf("packTemplate(%q, %v) = %q, want %q", tt.template, tt.args, got, tt.want)
		}
	}
}

func TestPackTemplateRejectsUnknownCodes(t *testing.T) {
	for _, template := range []string{"w2", "b8", "f", "d", "N<", "l!", "@4", "X2"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("packItems(%q) did not panic", template)
				}
			}()
			packItems(template)
		}()
	}
	// x past the end of the data dies in perl and panics here.
	defer func() {
		if recover() == nil {
			t.Error("unpackTemplate(x5) past the end did not panic")
		}
	}()
	unpackTemplate("x5", "abc")
}

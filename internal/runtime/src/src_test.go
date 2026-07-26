// The expectations in this file were taken from perl 5.42.2 rather than from
// memory: each table was run through the equivalent Perl one-liner and the two
// outputs compared byte for byte. Where a helper deliberately parts company
// with Perl, the case says so and the helper's doc comment explains why.
package src

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

func TestNum(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"42", 42},
		{" 42 ", 42},
		{"-17", -17},
		{"+3.5", 3.5},
		{"3.14abc", 3.14},
		{"abc", 0},
		{"", 0},
		{".5", 0.5},
		{"-.5", -0.5},
		{"5.", 5},
		{".", 0},
		{"1e3", 1000},
		{"1E3", 1000},
		{"1e", 1},  // the exponent is only taken when it has digits
		{"1e+", 1}, // and a sign alone is not digits
		{"1.5e-3", 0.0015},
		{"0x1f", 0},  // no hex: the numeric prefix of "0x1f" is "0"
		{"0b101", 0}, // and no binary either
		{"017", 17},  // a leading zero is not octal
		{"1_000", 1}, // underscores are a literal-only convenience
		{"00012", 12},
		{"9999999999999999999999", 1e22},
		{" -  3", 0}, // the sign must touch the digits
		{"--3", 0},
		{"3-4", 3},
		{"\t\n 6", 6}, // every kind of leading space is skipped
		{"1e310", math.Inf(1)},
		{"-1e310", math.Inf(-1)},
		{"1e-400", 0},
		{"inf", math.Inf(1)},
		{"-inf", math.Inf(-1)},
		{"Infinity", math.Inf(1)},
		{"-0", 0},
	}
	for _, tt := range tests {
		got := parseNum(tt.in)
		if got != tt.want {
			t.Errorf("parseNum(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
	for _, in := range []string{"nan", "NaN", "-nan", "nanny"} {
		if got := parseNum(in); !math.IsNaN(got) {
			t.Errorf("parseNum(%q) = %v, want NaN", in, got)
		}
	}
}

func TestStr(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{math.Copysign(0, -1), "0"},
		{1, "1"},
		{1.5, "1.5"},
		{4.8, "4.8"},
		{-2.5, "-2.5"},
		{1.0 / 3.0, "0.333333333333333"},
		{0.1 + 0.2, "0.3"},
		{255, "255"},
		{3.14159265358979, "3.14159265358979"},
		{1e15, "1e+15"},
		{1e16, "1e+16"},
		{1e20, "1e+20"},
		{1e21, "1e+21"},
		{1e-5, "1e-05"},
		{1e100, "1e+100"},
		{1e-100, "1e-100"},
		{9007199254740992, "9.00719925474099e+15"},
		{math.Inf(1), "Inf"},
		{math.Inf(-1), "-Inf"},
		{math.NaN(), "NaN"},
	}
	for _, tt := range tests {
		if got := formatNum(tt.in); got != tt.want {
			t.Errorf("formatNum(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNumOf(t *testing.T) {
	tests := []struct {
		in   any
		want float64
	}{
		{nil, 0},
		{true, 1},
		{false, 0},
		{float64(2.5), 2.5},
		{float32(0.5), 0.5},
		{int(-7), -7},
		{int64(1 << 40), 1 << 40},
		{"12abc", 12},
		{"", 0},
		{" 3.5 ", 3.5},
		{uint8(200), 200}, // not a listed type, so it goes through its text
		{[]byte("42"), 42},
	}
	for _, tt := range tests {
		if got := toNum(tt.in); got != tt.want {
			t.Errorf("toNum(%#v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// stringer stands in for a value that knows how to render itself.
type stringer struct{ text string }

func (s stringer) String() string { return s.text }

func TestText(t *testing.T) {
	tests := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"abc", "abc"},
		{true, "1"},
		{false, ""},
		{float64(0.1 + 0.2), "0.3"},
		{float32(1.5), "1.5"},
		{[]byte("bytes"), "bytes"},
		{stringer{"stringer"}, "stringer"},
		{42, "42"},
		{int64(-1), "-1"},
	}
	for _, tt := range tests {
		if got := toText(tt.in); got != tt.want {
			t.Errorf("toText(%#v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTrue(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"0", false},
		{"0.0", true}, // reads as zero, but it is not the string "0"
		{"00", true},
		{"0E0", true},
		{" ", true},
		{"0 ", true},
		{" 0", true},
		{"0\n", true},
		{"false", true},
	}
	for _, tt := range tests {
		if got := truthy(tt.in); got != tt.want {
			t.Errorf("truthy(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestMod(t *testing.T) {
	tests := []struct{ a, b, want int }{
		{7, 3, 1},
		{-7, 3, 2},
		{7, -3, -2},
		{-7, -3, -1},
		{0, 5, 0},
		{5, 5, 0},
		{10, 3, 1},
		{-10, 3, 2},
		{10, -3, -2},
		{-1, 5, 4},
		{1, 1, 0},
		{-3, 7, 4},
	}
	for _, tt := range tests {
		if got := mod(tt.a, tt.b); got != tt.want {
			t.Errorf("mod(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestPowInt(t *testing.T) {
	tests := []struct{ base, exponent, want int }{
		{2, 10, 1024},
		{2, 0, 1},
		{0, 0, 1},
		{-2, 3, -8},
		{-1, 3, -1},
		{-1, 4, 1},
		{3, 5, 243},
		{10, 3, 1000},
		{7, 2, 49},
		{1, 100, 1},
		{5, 1, 5},
		{3, 39, 4052555153018976267}, // exact, where math.Pow would round
		// A negative exponent truncates the true fraction towards zero.
		{2, -1, 0},
		{1, -5, 1},
		{-1, -3, -1},
		{-1, -4, 1},
	}
	for _, tt := range tests {
		if got := powInt(tt.base, tt.exponent); got != tt.want {
			t.Errorf("powInt(%d, %d) = %d, want %d", tt.base, tt.exponent, got, tt.want)
		}
	}
}

func TestStrInc(t *testing.T) {
	tests := []struct{ in, want string }{
		// Letters and digits carry within their own range.
		{"aa", "ab"},
		{"Az", "Ba"},
		{"zz", "aaa"},
		{"a9", "b0"},
		{"Zz", "AAa"},
		{"a", "b"},
		{"z", "aa"},
		{"zZ", "aaA"},
		{"ZZ", "AAA"},
		{"zz9", "aaa0"},
		{"Zz99", "AAa00"},
		{"Az9", "Ba0"},
		{"foo123", "foo124"},
		{"abc", "abd"},
		{"007", "008"},
		{"09", "10"},
		{"99", "100"},
		{"9", "10"},
		{"0", "1"},
		// Anything else is incremented as a number.
		{"", "1"},
		{"a9z", "1"}, // digits may not be followed by letters
		{" a", "1"},  // nor may anything be preceded by a space
		{"1.5", "2.5"},
		{"3.9", "4.9"},
		{"a1b", "1"},
		{"-1", "0"},
	}
	for _, tt := range tests {
		if got := strInc(tt.in); got != tt.want {
			t.Errorf("strInc(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSprintf(t *testing.T) {
	// Arguments are passed as text on purpose: that is how a value that has
	// been read from a file or a command line arrives, and every conversion
	// has to cope with it.
	tests := []struct {
		format string
		args   []any
		want   string
	}{
		{"%s", []any{"abc"}, "abc"},
		{"%s", []any{"3.0"}, "3.0"},
		{"%d", []any{"12abc"}, "12"},
		{"%d", []any{"abc"}, "0"},
		{"%d", []any{"3.7"}, "3"},
		{"%d", []any{"-3.7"}, "-3"},
		{"%d", []any{" 42 "}, "42"},
		{"%d", []any{"0x1f"}, "0"},
		{"%5.2f", []any{"3.14159"}, " 3.14"},
		{"%-8s|", []any{"hi"}, "hi      |"},
		{"%08.3f", []any{"3.14159"}, "0003.142"},
		{"%x", []any{"255"}, "ff"},
		{"%X", []any{"255"}, "FF"},
		{"%#x", []any{"255"}, "0xff"},
		{"%o", []any{"8"}, "10"},
		{"%b", []any{"5"}, "101"},
		{"%B", []any{"255"}, "11111111"},
		{"%#b", []any{"5"}, "0b101"},
		{"%e", []any{"1234.5678"}, "1.234568e+03"},
		{"%E", []any{"1234.5678"}, "1.234568E+03"},
		{"%g", []any{"0.0001234"}, "0.0001234"},
		{"%g", []any{"123456789"}, "1.23457e+08"},
		{"%g", []any{"100000"}, "100000"},
		{"%g", []any{"1000000"}, "1e+06"},
		{"%.3g", []any{"3.14159"}, "3.14"},
		{"%G", []any{"1e-10"}, "1E-10"},
		{"%.10g", []any{"3.14159265358979"}, "3.141592654"},
		{"%c", []any{"65"}, "A"},
		{"%c", []any{"9731"}, "☃"},
		{"%%", nil, "%"},
		{"%s %s", []any{"a", "b"}, "a b"},
		{"%2$s %1$s", []any{"a", "b"}, "b a"},
		{"%3$s", []any{"a", "b", "c"}, "c"},
		{"%*d", []any{"5", "42"}, "   42"},
		{"%-*d|", []any{"5", "42"}, "42   |"},
		{"%.*f", []any{"2", "3.14159"}, "3.14"},
		{"%5s|", []any{"abcdefg"}, "abcdefg|"},
		{"%.3s|", []any{"abcdefg"}, "abc|"},
		{"%10.4s|", []any{"abcdefg"}, "      abcd|"},
		{"%+d", []any{"42"}, "+42"},
		{"% d", []any{"42"}, " 42"},
		{"%u", []any{"-1"}, "18446744073709551615"},
		{"%x", []any{"-1"}, "ffffffffffffffff"},
		{"%vd", []any{"1.22.333"}, "49.46.50.50.46.51.51.51"},
		{"%ld", []any{"42"}, "42"}, // length modifiers are accepted and ignored
		{"%.5d", []any{"42"}, "00042"},
		{"%.0d", []any{"0"}, ""},
		{"%.0f", []any{"0.5"}, "0"}, // ties go to the even digit
		{"%.0f", []any{"1.5"}, "2"},
		{"%.0f", []any{"2.5"}, "2"},
		{"%s", nil, ""},  // a missing argument is empty text
		{"%d", nil, "0"}, // and the number zero
		{"literal", nil, "literal"},
	}
	for _, tt := range tests {
		if got := sprintf(tt.format, tt.args...); got != tt.want {
			t.Errorf("sprintf(%q, %v) = %q, want %q", tt.format, tt.args, got, tt.want)
		}
	}
}

func TestSprintfTakesValuesOfAnyType(t *testing.T) {
	tests := []struct {
		format string
		args   []any
		want   string
	}{
		{"%s", []any{0.1 + 0.2}, "0.3"},
		{"%s", []any{true}, "1"},
		{"%s", []any{nil}, ""},
		{"%d", []any{3.7}, "3"},
		{"%d", []any{42}, "42"},
		{"%.2f", []any{float32(1.5)}, "1.50"},
	}
	for _, tt := range tests {
		if got := sprintf(tt.format, tt.args...); got != tt.want {
			t.Errorf("sprintf(%q, %v) = %q, want %q", tt.format, tt.args, got, tt.want)
		}
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		pattern string
		s       string
		limit   int
		want    []string
	}{
		{",", "a,b,c", 0, []string{"a", "b", "c"}},
		{",", "a,b,c,,,", 0, []string{"a", "b", "c"}},              // a limit of zero drops
		{",", "a,b,c,,,", -1, []string{"a", "b", "c", "", "", ""}}, // a negative one keeps
		{",", "a,b,c,,,", 4, []string{"a", "b", "c", ",,"}},
		{",", ",a,b", 0, []string{"", "a", "b"}}, // a leading empty field is kept
		{",", ",,a", 0, []string{"", "", "a"}},
		{",", "a,b,c", 2, []string{"a", "b,c"}},
		{",", "a,b,c", 10, []string{"a", "b", "c"}},
		{",", "a,b,c", 1, []string{"a,b,c"}},
		{",", "", 0, nil},
		{",", "", -1, nil},
		{",", ",", 0, nil},
		{",", ",", -1, []string{"", ""}},
		{",", ",,,", -1, []string{"", "", "", ""}},
		{"a", "aaa", -1, []string{"", "", "", ""}},
		{"z", "abc", 0, []string{"abc"}},
		{"", "abc", 0, []string{"a", "b", "c"}}, // a zero-width match splits characters
		{"", "abc", 2, []string{"a", "bc"}},
		{"x*", "abc", 0, []string{"a", "b", "c"}},
		{"x*", "axb", 0, []string{"a", "b"}},
		{`\s+`, "  a b  c", 0, []string{"", "a", "b", "c"}},
		{`\b`, "ab cd", 0, []string{"ab", " ", "cd"}},
		{"a*", "baaac", -1, []string{"b", "c", ""}},
		{"$", "abc", 0, []string{"abc"}},
		{`\W+`, "Hello, World! Bye", 0, []string{"Hello", "World", "Bye"}},
		// Captured text is returned too, and does not count towards the limit.
		{"(,)", "a,b,c", 0, []string{"a", ",", "b", ",", "c"}},
		{"(,)", "a,b,c", 2, []string{"a", ",", "b,c"}},
		{"(,)", "a,b,c,d", 3, []string{"a", ",", "b", ",", "c,d"}},
		{"(,)(;)", "a,;b,;c", 2, []string{"a", ",", ";", "b,;c"}},
		{"(x*)", "abc", 0, []string{"a", "", "b", "", "c"}},
		{`(\d)(\w)?`, "a1bc3", 0, []string{"a", "1", "b", "c", "3"}},
		// A group that did not take part contributes an empty field.
		{"(a)|(b)", "xayb", -1, []string{"x", "a", "", "y", "", "b", ""}},
		// "^" only means the start of a line when the pattern says so.
		{"(?m)^", "a\nb", 0, []string{"a\n", "b"}},
	}
	for _, tt := range tests {
		got := splitPattern(regexp.MustCompile(tt.pattern), tt.s, tt.limit)
		if !slices.Equal(got, tt.want) {
			t.Errorf("splitPattern(%q, %q, %d) = %q, want %q", tt.pattern, tt.s, tt.limit, got, tt.want)
		}
	}
}

func TestSubstr(t *testing.T) {
	const s = "Hello, World"
	tests := []struct {
		offset, length int
		want           string
	}{
		{0, 5, "Hello"},
		{7, 5, "World"},
		{-5, 5, "World"},
		{-5, -1, "Worl"},
		{2, -2, "llo, Wor"},
		{0, -1, "Hello, Worl"},
		{0, 100, "Hello, World"},
		{3, 0, ""},
		{5, -10, ""},
		{-3, 10, "rld"},
		{12, 1, ""},
		{0, 0, ""},
		{-20, 15, "Hello, "}, // clipped to the part that overlaps
		{-100, 100, "Hello, World"},
		{11, 5, "d"},
		{6, -6, ""},
		// Perl returns undef for a window that misses the string entirely.
		// Go has no undef, so these give the empty string instead.
		{100, 1, ""},
		{-100, 3, ""},
	}
	for _, tt := range tests {
		if got := substr(s, tt.offset, tt.length); got != tt.want {
			t.Errorf("substr(%q, %d, %d) = %q, want %q", s, tt.offset, tt.length, got, tt.want)
		}
	}
}

func TestSubstrFrom(t *testing.T) {
	const s = "Hello, World"
	tests := []struct {
		offset int
		want   string
	}{
		{0, "Hello, World"},
		{5, ", World"},
		{-5, "World"},
		{12, ""},
		{-100, "Hello, World"}, // clipped to the start
		{13, ""},               // undef in Perl, empty here
	}
	for _, tt := range tests {
		if got := substrFrom(s, tt.offset); got != tt.want {
			t.Errorf("substrFrom(%q, %d) = %q, want %q", s, tt.offset, got, tt.want)
		}
	}
}

func TestSubstrReplace(t *testing.T) {
	const s = "Hello, World"
	tests := []struct {
		offset, length int
		want           string
	}{
		{0, 5, "<R>, World"},
		{7, 5, "Hello, <R>"},
		{-5, 5, "Hello, <R>"},
		{-5, -1, "Hello, <R>d"},
		{2, -2, "He<R>ld"},
		{0, 0, "<R>Hello, World"},
		{12, 0, "Hello, World<R>"},
		{5, 100, "Hello<R>"},
		{-20, 15, "<R>World"},
		{3, -100, "Hel<R>lo, World"},
	}
	for _, tt := range tests {
		if got := substrReplace(s, tt.offset, tt.length, "<R>"); got != tt.want {
			t.Errorf("substrReplace(%q, %d, %d, %q) = %q, want %q",
				s, tt.offset, tt.length, "<R>", got, tt.want)
		}
	}
}

func TestIndex(t *testing.T) {
	const s = "hello world hello"
	tests := []struct {
		substr   string
		position int
		want     int
	}{
		{"hello", 0, 0},
		{"hello", 1, 12},
		{"hello", -5, 0}, // a position before the start means the start
		{"hello", 17, -1},
		{"o", 4, 4},
		{"o", 100, -1},
		{"world", 0, 6},
		{"z", 0, -1},
		{"", 0, 0},
		{"", 5, 5},
		{"", 100, 17}, // a position past the end means the end
		{"", -3, 0},
	}
	for _, tt := range tests {
		if got := indexOf(s, tt.substr, tt.position); got != tt.want {
			t.Errorf("indexOf(%q, %q, %d) = %d, want %d", s, tt.substr, tt.position, got, tt.want)
		}
	}
}

func TestRindex(t *testing.T) {
	const s = "hello world hello"
	tests := []struct {
		substr   string
		position int
		want     int
	}{
		{"hello", 17, 12},
		{"hello", 100, 12},
		{"hello", 12, 12},
		{"hello", 11, 0},
		{"hello", 1, 0},
		{"hello", 0, 0},
		// The position is where a match may start, so a position too far
		// left for the needle to fit finds nothing at all.
		{"hello", -1, -1},
		{"hello", -5, -1},
		{"hello", -100, -1},
		{"h", -1, -1},
		{"o", 100, 16},
		{"o", 4, 4},
		{"world", 0, -1},
		{"z", 0, -1},
		{"", 0, 0},
		{"", 5, 5},
		{"", 100, 17},
		{"", -3, 0},
	}
	for _, tt := range tests {
		if got := lastIndexOf(s, tt.substr, tt.position); got != tt.want {
			t.Errorf("lastIndexOf(%q, %q, %d) = %d, want %d", s, tt.substr, tt.position, got, tt.want)
		}
	}
}

func TestChop(t *testing.T) {
	tests := []struct{ in, rest, removed string }{
		{"abc", "ab", "c"},
		{"a", "", "a"},
		{"", "", ""},
		{"ab\n", "ab", "\n"},
		{"hello", "hell", "o"},
		{"0", "", "0"},
		// A whole character comes off, not a byte. Perl without "use utf8"
		// would leave half a character behind here.
		{"héllo", "héll", "o"},
		{"caté", "cat", "é"},
	}
	for _, tt := range tests {
		rest, removed := chop(tt.in)
		if rest != tt.rest || removed != tt.removed {
			t.Errorf("chop(%q) = %q, %q, want %q, %q", tt.in, rest, removed, tt.rest, tt.removed)
		}
	}
}

func TestOrd(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"A", 65},
		{"abc", 97},
		{"", 0},
		{"0", 48},
		{" ", 32},
		{"~", 126},
		{"é", 233},  // the code point, where a byte-oriented Perl says 195
		{"☃", 9731}, // and a snowman is one character, not three
	}
	for _, tt := range tests {
		if got := ord(tt.in); got != tt.want {
			t.Errorf("ord(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestUcfirstAndLcfirst(t *testing.T) {
	tests := []struct{ in, upper, lower string }{
		{"abc", "Abc", "abc"},
		{"Abc", "Abc", "abc"},
		{"", "", ""},
		{"1a", "1a", "1a"},
		{"a", "A", "a"},
		{"ABC", "ABC", "aBC"},
		{"aBC", "ABC", "aBC"},
		{"élan", "Élan", "élan"}, // one character in, one character out
		{"Élan", "Élan", "élan"},
	}
	for _, tt := range tests {
		if got := ucFirst(tt.in); got != tt.upper {
			t.Errorf("ucFirst(%q) = %q, want %q", tt.in, got, tt.upper)
		}
		if got := lcFirst(tt.in); got != tt.lower {
			t.Errorf("lcFirst(%q) = %q, want %q", tt.in, got, tt.lower)
		}
	}
}

func TestReverseStr(t *testing.T) {
	tests := []struct{ in, want string }{
		{"abc", "cba"},
		{"", ""},
		{"a", "a"},
		{"ab", "ba"},
		{"racecar", "racecar"},
		{"héllo", "olléh"}, // characters, not bytes
		{"☃x", "x☃"},
	}
	for _, tt := range tests {
		if got := reverseStr(tt.in); got != tt.want {
			t.Errorf("reverseStr(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRepeat(t *testing.T) {
	tests := []struct {
		s     string
		count int
		want  string
	}{
		{"ab", 3, "ababab"},
		{"ab", 1, "ab"},
		{"ab", 0, ""},
		{"ab", -1, ""}, // strings.Repeat would panic
		{"", 5, ""},
		{"x", 4, "xxxx"},
	}
	for _, tt := range tests {
		if got := repeatStr(tt.s, tt.count); got != tt.want {
			t.Errorf("repeatStr(%q, %d) = %q, want %q", tt.s, tt.count, got, tt.want)
		}
	}
}

func TestRepeatList(t *testing.T) {
	tests := []struct {
		xs    []int
		count int
		want  []int
	}{
		{[]int{1, 2}, 3, []int{1, 2, 1, 2, 1, 2}},
		{[]int{1, 2}, 1, []int{1, 2}},
		{[]int{1, 2}, 0, nil},
		{[]int{1, 2}, -1, nil}, // slices.Repeat would panic
		{nil, 3, []int{}},
		{[]int{7}, 4, []int{7, 7, 7, 7}},
	}
	for _, tt := range tests {
		got := repeatList(tt.xs, tt.count)
		if !slices.Equal(got, tt.want) {
			t.Errorf("repeatList(%v, %d) = %v, want %v", tt.xs, tt.count, got, tt.want)
		}
	}

	// The result never shares storage with the original.
	xs := []int{1, 2}
	out := repeatList(xs, 2)
	out[0] = 99
	if xs[0] != 1 {
		t.Errorf("repeatList wrote through to its argument: %v", xs)
	}

	if got := repeatList([]string{"a"}, 2); !slices.Equal(got, []string{"a", "a"}) {
		t.Errorf("repeatList on strings = %v", got)
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	present := filepath.Join(dir, "present")
	if err := os.WriteFile(present, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(present, link); err != nil {
		t.Skipf("symlinks are unavailable here: %v", err)
	}
	dangling := filepath.Join(dir, "dangling")
	if err := os.Symlink(filepath.Join(dir, "gone"), dangling); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path string
		want bool
	}{
		{present, true},
		{empty, true}, // an empty file still exists
		{dir, true},   // and so does a directory
		{link, true},
		{dangling, false}, // links are followed, so a dead one does not exist
		{filepath.Join(dir, "missing"), false},
		{filepath.Join(present, "under-a-file"), false},
		{"", false},
	}
	for _, tt := range tests {
		if got := fileExists(tt.path); got != tt.want {
			t.Errorf("fileExists(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestMagicStr(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"aa", true},
		{"Az9", true},
		{"007", true},
		{"a", true},
		{"9", true},
		{"aZ09", true},
		{"", false},
		{"a9z", false}, // digits may not be followed by letters
		{" a", false},
		{"1.5", false},
		{"a-b", false},
		{"héllo", false},
	}
	for _, tt := range tests {
		if got := magicStr(tt.in); got != tt.want {
			t.Errorf("magicStr(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestIsTrue(t *testing.T) {
	var nilMap map[string]int
	var nilSlice []int
	var nilPointer *int
	one := 1

	tests := []struct {
		in   any
		want bool
	}{
		{nil, false},
		{true, true},
		{false, false},
		{"", false},
		{"0", false},
		{"0.0", true}, // the string rule, not the number rule
		{"00", true},
		{" ", true},
		{0, false},
		{-1, true},
		{0.0, false},
		{math.Copysign(0, -1), false},
		{math.NaN(), true}, // not a number, but not zero either
		{int8(0), false},
		{uint64(0), false},
		{uint64(3), true},
		{float32(0), false},
		{[]int{}, false},
		{[]int{0}, true}, // a list of one false thing is still a list
		{nilSlice, false},
		{map[string]int{}, false},
		{map[string]int{"a": 0}, true},
		{nilMap, false},
		{nilPointer, false},
		{&one, true},
		{struct{}{}, true},
		{[0]int{}, false},
		{[1]int{0}, true},
	}
	for _, tt := range tests {
		if got := isTrue(tt.in); got != tt.want {
			t.Errorf("isTrue(%#v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestSeq(t *testing.T) {
	tests := []struct {
		from, to int
		want     []int
	}{
		{1, 5, []int{1, 2, 3, 4, 5}},
		{0, 0, []int{0}},
		{-2, 2, []int{-2, -1, 0, 1, 2}},
		{5, 1, nil}, // never counts backwards
		{3, 2, nil},
		{-5, -3, []int{-5, -4, -3}},
		{7, 8, []int{7, 8}},
	}
	for _, tt := range tests {
		if got := seq(tt.from, tt.to); !slices.Equal(got, tt.want) {
			t.Errorf("seq(%d, %d) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestStrRange(t *testing.T) {
	tests := []struct {
		from, to string
		want     []string
	}{
		{"aa", "ad", []string{"aa", "ab", "ac", "ad"}},
		{"az", "bc", []string{"az", "ba", "bb", "bc"}},
		{"a", "e", []string{"a", "b", "c", "d", "e"}},
		{"a", "a", []string{"a"}},
		{"A", "E", []string{"A", "B", "C", "D", "E"}},
		{"Aa", "Ac", []string{"Aa", "Ab", "Ac"}},
		{"zz", "aaa", []string{"zz", "aaa"}},
		{"09", "11", []string{"09", "10", "11"}},
		{"1", "5", []string{"1", "2", "3", "4", "5"}},
		{"a1", "a9", []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7", "a8", "a9"}},
		// The end is shorter than the start, so it can never come up.
		{"aa", "a", nil},
		{"a1b", "z", nil},
		{" a", "c", nil},
		// The start does not step character by character, so it stands alone.
		{"1a", "9z", []string{"1a"}},
		{"", "c", []string{""}},
	}
	for _, tt := range tests {
		if got := strRange(tt.from, tt.to); !slices.Equal(got, tt.want) {
			t.Errorf("strRange(%q, %q) = %q, want %q", tt.from, tt.to, got, tt.want)
		}
	}

	// Ends that are not in one sequence stop at the length of the end
	// rather than running away: "b" never reaches "a", so the walk ends
	// with the last one-character value.
	runaway := strRange("b", "a")
	if want := 25; len(runaway) != want || runaway[0] != "b" || runaway[len(runaway)-1] != "z" {
		t.Errorf("strRange(%q, %q) = %q, want %d values from b to z", "b", "a", runaway, want)
	}
	if got := strRange("a", "aa"); len(got) != 27 || got[25] != "z" || got[26] != "aa" {
		t.Errorf("strRange(%q, %q) gave %d values ending %q", "a", "aa", len(got), got[len(got)-1])
	}
	if got := strRange("a", "ZZ"); len(got) != 702 || got[len(got)-1] != "zz" {
		t.Errorf("strRange(%q, %q) gave %d values ending %q", "a", "ZZ", len(got), got[len(got)-1])
	}
}

func TestJoinList(t *testing.T) {
	if got := joinList([]int{1, 2, 3}, ","); got != "1,2,3" {
		t.Errorf("joinList of ints = %q", got)
	}
	if got := joinList([]string{"a", "b"}, ""); got != "ab" {
		t.Errorf("joinList with no separator = %q", got)
	}
	if got := joinList([]string{"only"}, ", "); got != "only" {
		t.Errorf("joinList of one = %q", got)
	}
	if got := joinList([]string{}, ","); got != "" {
		t.Errorf("joinList of none = %q", got)
	}
	if got := joinList[int](nil, ","); got != "" {
		t.Errorf("joinList of nil = %q", got)
	}
	// Elements go through toText, so these are its rules, not fmt's.
	if got := joinList([]any{1, "a", 2.5, nil, true}, ","); got != "1,a,2.5,,1" {
		t.Errorf("joinList of mixed values = %q, want %q", got, "1,a,2.5,,1")
	}
	if got := joinList([]float64{0.1 + 0.2, 1e21}, " "); got != "0.3 1e+21" {
		t.Errorf("joinList of floats = %q", got)
	}
	if got := joinList([]int{1, 2}, ", "); got != "1, 2" {
		t.Errorf("joinList with a two-character separator = %q", got)
	}
}

func TestAt(t *testing.T) {
	xs := []string{"a", "b", "c"}
	tests := []struct {
		i    int
		want string
	}{
		{0, "a"},
		{2, "c"},
		{-1, "c"}, // counts back from the end
		{-3, "a"},
		{3, ""}, // past the end is a missing value, not a panic
		{99, ""},
		{-4, ""},
		{-99, ""},
	}
	for _, tt := range tests {
		if got := at(xs, tt.i); got != tt.want {
			t.Errorf("at(%q, %d) = %q, want %q", xs, tt.i, got, tt.want)
		}
	}

	if got := at([]int{}, 0); got != 0 {
		t.Errorf("at of an empty list = %v, want the zero value", got)
	}
	if got := at([]int(nil), -1); got != 0 {
		t.Errorf("at of no list = %v, want the zero value", got)
	}
	if got := at([]float64{1.5}, 0); got != 1.5 {
		t.Errorf("at of a float list = %v", got)
	}
}

// The expectations here were taken from perl 5.42.2, one hex() or oct()
// call per case.
package src

import "testing"

func TestHexNum(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"0x10", 16},
		{"10", 16},
		{"ff", 255},
		{"FF", 255},
		{"0xff", 255},
		{"1g", 1}, // junk ends the read
		{"_5", 5}, // underscores are skipped
		{"", 0},
		{"g", 0},
	}
	for _, tt := range tests {
		if got := hexNum(tt.in); got != tt.want {
			t.Errorf("hexNum(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestOctNum(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"10", 8},
		{"010", 8},
		{"19", 1}, // 9 is not an octal digit
		{"0x10", 16},
		{"0b101", 5},
		{"0b12", 1},
		{"0o17", 15},
		{"1_0", 8},
		{" 0x10", 16},
		{"", 0},
	}
	for _, tt := range tests {
		if got := octNum(tt.in); got != tt.want {
			t.Errorf("octNum(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

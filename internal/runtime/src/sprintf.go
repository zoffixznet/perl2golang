package src

import (
	"fmt"
	"strconv"
	"strings"
)

// sprintf formats args according to format and returns the result.
//
// A conversion is a percent sign, an optional argument index such as
// "2$", any of the flags "-+ 0#", an optional "v", a width, a precision
// and a conversion letter. Width and precision may be written as "*",
// which takes the number from the next argument. The letters are s, c, d,
// i, u, o, x, X, b, B, e, E, f, F, g and G, plus "%%" for a literal
// percent sign. Length modifiers such as "l" are accepted and ignored.
//
// Arguments do not have to match their conversion: each one is read as
// text or as a number on demand, so "%d" on "12abc" gives 12 and "%s" on
// a number gives the number. A missing argument reads as empty text and
// as the number 0. The default precision of g and G is six significant
// digits, and the integer conversions other than d and i treat their
// argument as an unsigned 64 bit value.
//
// With the "v" flag the argument is treated as a string and every one of
// its characters is formatted separately, the results joined with dots.
func sprintf(format string, args ...any) string {
	var out strings.Builder
	next := 0
	arg := func(index int) any {
		i := index - 1
		if index == 0 {
			i = next
			next++
		}
		if i < 0 || i >= len(args) {
			return nil
		}
		return args[i]
	}

	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			out.WriteByte(format[i])
			continue
		}
		verbStart := i
		j := i + 1

		index := 0
		if k := j; k < len(format) && format[k] >= '1' && format[k] <= '9' {
			n := 0
			for k < len(format) && format[k] >= '0' && format[k] <= '9' {
				n = n*10 + int(format[k]-'0')
				k++
			}
			if k < len(format) && format[k] == '$' {
				index = n
				j = k + 1
			}
		}

		flags := ""
		vector := false
		for j < len(format) {
			c := format[j]
			if c == 'v' {
				vector = true
				j++
				continue
			}
			if strings.IndexByte("-+ 0#", c) < 0 {
				break
			}
			if !strings.ContainsRune(flags, rune(c)) {
				flags += string(c)
			}
			j++
		}

		width, hasWidth := 0, false
		if j < len(format) && format[j] == '*' {
			width, hasWidth = int(toNum(arg(0))), true
			j++
		} else {
			for j < len(format) && format[j] >= '0' && format[j] <= '9' {
				width, hasWidth = width*10+int(format[j]-'0'), true
				j++
			}
		}
		if width < 0 {
			if !strings.Contains(flags, "-") {
				flags += "-"
			}
			width = -width
		}

		precision, hasPrecision := 0, false
		if j < len(format) && format[j] == '.' {
			j++
			hasPrecision = true
			if j < len(format) && format[j] == '*' {
				precision = int(toNum(arg(0)))
				j++
			} else {
				for j < len(format) && format[j] >= '0' && format[j] <= '9' {
					precision = precision*10 + int(format[j]-'0')
					j++
				}
			}
			if precision < 0 {
				precision, hasPrecision = 0, false
			}
		}

		for j < len(format) && strings.IndexByte("hlLqV", format[j]) >= 0 {
			j++
		}
		if j >= len(format) {
			out.WriteString(format[verbStart:])
			break
		}

		conv := format[j]
		i = j
		if conv == '%' {
			out.WriteByte('%')
			continue
		}

		spec := "%" + flags
		if hasWidth {
			spec += strconv.Itoa(width)
		}
		switch {
		case hasPrecision:
			spec += "." + strconv.Itoa(precision)
		case conv == 'g' || conv == 'G':
			spec += ".6"
		}

		if vector {
			letter := conv
			switch letter {
			case 'i', 'u':
				letter = 'd'
			case 'B':
				letter = 'b'
			}
			text := toText(arg(index))
			parts := make([]string, 0, len(text))
			for _, r := range text {
				parts = append(parts, fmt.Sprintf(spec+string(letter), int64(r)))
			}
			out.WriteString(strings.Join(parts, "."))
			continue
		}

		switch conv {
		case 's':
			out.WriteString(fmt.Sprintf(spec+"s", toText(arg(index))))
		case 'c':
			out.WriteString(fmt.Sprintf(spec+"c", rune(int64(toNum(arg(index))))))
		case 'd', 'i':
			out.WriteString(fmt.Sprintf(spec+"d", int64(toNum(arg(index)))))
		case 'u':
			out.WriteString(fmt.Sprintf(spec+"d", uint64(int64(toNum(arg(index))))))
		case 'o', 'x', 'X', 'b':
			out.WriteString(fmt.Sprintf(spec+string(conv), uint64(int64(toNum(arg(index))))))
		case 'B':
			out.WriteString(fmt.Sprintf(spec+"b", uint64(int64(toNum(arg(index))))))
		case 'e', 'E', 'f', 'F', 'g', 'G':
			out.WriteString(fmt.Sprintf(spec+string(conv), toNum(arg(index))))
		default:
			out.WriteString(format[verbStart : j+1])
		}
	}
	return out.String()
}

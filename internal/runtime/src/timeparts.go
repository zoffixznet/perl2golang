package src

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// timeParts breaks a Unix timestamp into the nine numbers a time split
// reports, in that order: seconds, minutes, hours, day of the month, month
// counted from zero, year counted from 1900, weekday, day of the year counted
// from zero, and whether daylight saving is in force.
//
// The two offsets are the ones that catch everyone: the month is one less
// than its name and the year is 1900 less than itself.
func timeParts(epoch int, local bool) []int {
	t := time.Unix(int64(epoch), 0).UTC()
	if local {
		t = time.Unix(int64(epoch), 0).Local()
	}
	dst := 0
	if local {
		if _, offset := t.Zone(); offset != stdOffset(t) {
			dst = 1
		}
	}
	return []int{
		t.Second(), t.Minute(), t.Hour(), t.Day(), int(t.Month()) - 1,
		t.Year() - 1900, int(t.Weekday()), t.YearDay() - 1, dst,
	}
}

// stdOffset is the zone offset this location uses outside daylight saving,
// taken from the middle of January and the middle of July, whichever is
// further from UTC in the negative direction.
func stdOffset(t time.Time) int {
	_, jan := time.Date(t.Year(), time.January, 15, 12, 0, 0, 0, t.Location()).Zone()
	_, jul := time.Date(t.Year(), time.July, 15, 12, 0, 0, 0, t.Location()).Zone()
	return min(jan, jul)
}

// timeFrom rebuilds a time from the nine numbers timeParts produced, reading
// only the six that name a moment.
func timeFrom(parts []int) time.Time {
	at := func(i int) int {
		if i < len(parts) {
			return parts[i]
		}
		return 0
	}
	year := at(5)
	// A year is written either in full or as the years since 1900, and both
	// spellings are in use. Anything that looks like a real year is taken as
	// one, which is the rule the older calls follow too.
	if year < 1000 {
		year += 1900
	}
	return time.Date(year, time.Month(at(4)+1), at(3), at(2), at(1), at(0), 0, time.UTC)
}

// timeText renders a moment the way a bare time-to-string conversion does:
// "Thu Jan  1 00:00:00 1970", with the day of the month space-padded.
func timeText(t time.Time) string {
	return t.Format("Mon Jan _2 15:04:05 2006")
}

// formatStamp renders a moment through a format written as percent codes.
//
// Go writes a layout as an example timestamp instead, which covers most of
// these directly; this exists for the codes that have no layout at all, such
// as the day of the year and the week number, and for a format worked out
// while the program runs.
func formatStamp(format string, t time.Time) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 >= len(format) {
			b.WriteByte(format[i])
			continue
		}
		i++
		switch format[i] {
		case '%':
			b.WriteByte('%')
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'j':
			fmt.Fprintf(&b, "%03d", t.YearDay())
		case 'w':
			b.WriteString(strconv.Itoa(int(t.Weekday())))
		case 'u':
			day := int(t.Weekday())
			if day == 0 {
				day = 7
			}
			b.WriteString(strconv.Itoa(day))
		case 'U':
			fmt.Fprintf(&b, "%02d", (t.YearDay()+6-int(t.Weekday()))/7)
		case 'W':
			day := (int(t.Weekday()) + 6) % 7
			fmt.Fprintf(&b, "%02d", (t.YearDay()+6-day)/7)
		case 's':
			b.WriteString(strconv.FormatInt(t.Unix(), 10))
		case 'C':
			fmt.Fprintf(&b, "%02d", t.Year()/100)
		default:
			if layout, ok := stampLayouts[format[i]]; ok {
				b.WriteString(t.Format(layout))
				continue
			}
			b.WriteByte('%')
			b.WriteByte(format[i])
		}
	}
	return b.String()
}

// stampLayouts maps the percent codes that do have a layout onto it.
var stampLayouts = map[byte]string{
	'Y': "2006", 'y': "06", 'm': "01", 'd': "02", 'e': "_2",
	'H': "15", 'I': "03", 'M': "04", 'S': "05", 'p': "PM",
	'a': "Mon", 'A': "Monday", 'b': "Jan", 'B': "January", 'h': "Jan",
	'Z': "MST", 'z': "-0700",
	'F': "2006-01-02", 'T': "15:04:05", 'D': "01/02/06", 'R': "15:04",
	'c': "Mon Jan _2 15:04:05 2006", 'x': "01/02/06", 'X': "15:04:05",
}

// looksLikeNumber reports whether a value would be read as a number rather
// than as text, which is the question asked before trusting arithmetic on
// something that arrived as input.
func looksLikeNumber(v any) bool {
	switch n := v.(type) {
	case nil:
		return false
	case int, int64, float64, float32:
		return true
	case bool:
		return false
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return false
		}
		if _, err := strconv.ParseFloat(s, 64); err == nil {
			return true
		}
		switch strings.ToLower(strings.TrimLeft(s, "+-")) {
		case "inf", "infinity", "nan":
			return true
		}
		return false
	}
	return false
}

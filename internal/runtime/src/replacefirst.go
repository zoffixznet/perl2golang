package src

import "regexp"

// replaceFirst replaces the first match of re in s with repl and returns the
// result, leaving any later matches alone. repl is a template in which $1 or
// ${1} stands for the first capture group, exactly as it does for the
// ReplaceAll family.
//
// It exists because the regexp package offers only replace-every-match. A
// caller that wants every match should use re.ReplaceAllString directly.
func replaceFirst(re *regexp.Regexp, s, repl string) string {
	loc := re.FindStringSubmatchIndex(s)
	if loc == nil {
		return s
	}
	var out []byte
	out = append(out, s[:loc[0]]...)
	out = re.ExpandString(out, repl, s, loc)
	return string(out) + s[loc[1]:]
}

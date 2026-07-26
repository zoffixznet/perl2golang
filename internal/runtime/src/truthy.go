package src

// truthy reports whether s counts as true. Every string is true except
// the empty string and the single character "0", so " ", "00", "0.0" and
// "0E0" are all true even though they read as zero.
func truthy(s string) bool {
	return s != "" && s != "0"
}

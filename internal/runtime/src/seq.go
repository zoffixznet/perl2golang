package src

// seq returns every integer from from to to, with both ends included. It
// returns nothing at all when from is greater than to, rather than
// counting backwards.
func seq(from, to int) []int {
	if from > to {
		return nil
	}
	size := to - from + 1
	if size < 0 {
		size = 0 // the count is too large for an int; grow while going
	}
	out := make([]int, 0, size)
	for i := from; ; i++ {
		out = append(out, i)
		if i == to {
			return out
		}
	}
}

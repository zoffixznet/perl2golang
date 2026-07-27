package runtime

// notes explains, for each helper, the rule it implements and the shorter Go
// that would get that rule wrong. EmitAnnotated puts these at the top of the
// helper file it writes for the annotated program, and the report quotes them
// when it says which helpers a program needed.
//
// A note is prose, wrapped when it is printed, and is keyed by the primary
// name of the declaration. Every helper has one; a test enforces that.
var notes = map[string]string{
	"at": "Reading past the end of a list is not an error, it is a missing " +
		"value, so the helper answers with the zero value where a Go index " +
		"expression would panic. A negative index counts back from the end, " +
		"which Go's index expression cannot express at all.",

	"chop": "Removing the last character of a string is not the same as " +
		"removing its last byte: s[:len(s)-1] cuts a multi-byte character in " +
		"half and leaves an invalid string behind. The removed character is " +
		"returned alongside the shortened string because Perl's chop yields " +
		"it.",

	"fileExists": "A file exists when it can be stat'ed. Any error at all " +
		"means no, not only the one that says the file is missing, so testing " +
		"errors.Is(err, fs.ErrNotExist) would report a directory the program " +
		"cannot search as existing.",

	"dirNames": "Reading a directory hands back entries rather than names, " +
		"and it never includes \".\" or \"..\", so a filter written for those " +
		"two silently does nothing. The order is sorted, which the system call " +
		"underneath does not promise.",

	"fileSize": "The size of something that cannot be inspected is reported " +
		"as 0, the same as an empty file. os.Stat returns the error that " +
		"tells the two apart, and this throws it away, so anything that has " +
		"to know should call os.Stat directly.",

	"isDir": "One stat answers every question about a path. Asking three " +
		"separate questions with three helpers costs three system calls and " +
		"can see the filesystem change between them.",

	"isExecutable": "The permission bits say what the owner, the group and " +
		"everybody else may do. They do not say what this process may do, " +
		"which depends on who is running it.",

	"isFile": "A regular file is not merely something that exists: a " +
		"directory, a device and a socket all exist. Mode().IsRegular() is " +
		"the distinction, and it is not the same test as the one for a " +
		"directory negated.",

	"isReadable": "Whether a file can be read is answered by opening it, " +
		"because the permission bits do not know who is running. The answer " +
		"can also go stale the moment it is given, so code that is about to " +
		"read should just read and handle the error.",

	"isWritable": "The same approximation isExecutable makes: the bits say " +
		"what the owner and the group may do, not what this process may do, " +
		"and a read-only mount is invisible to them.",

	"formatNum": "Numbers become text with fifteen significant digits, which " +
		"is why 0.1 + 0.2 prints as 0.3. Go's own formatting prints the " +
		"shortest text that reads back exactly, which is 0.30000000000000004, " +
		"and the difference shows up in ordinary output.",

	"indexOf": "Searching from a position: strings.Index has no such " +
		"argument, and searching s[position:] instead returns an offset into " +
		"that slice rather than into s. A position before the start of the " +
		"string is not an error, it simply means the start.",

	"isTrue": "The truth rule of truthy, for a value whose type is only known " +
		"while the program runs. Go has no truth rule for a value at all: " +
		"`if v` does not compile, and testing v != nil would call the number " +
		"zero, the empty list and the string \"0\" true.",

	"joinList": "Putting a list into a string renders each element the way " +
		"toText does rather than the way fmt does, so a bool arrives as \"1\" " +
		"or as nothing, a missing value as nothing, and 0.1 + 0.2 as 0.3.",

	"lastIndexOf": "The last match at or before a position. strings.LastIndex " +
		"has no position argument, and the position says where a match may " +
		"start, so the window that has to be searched ends where a match " +
		"starting there would end.",

	"lcFirst": "Only the first character changes case. strings.ToLower would " +
		"lower the whole string, and s[0] would work on a byte instead of a " +
		"character.",

	"magicStr": "Which strings step character by character and which are " +
		"stepped as numbers. The rule is deliberately narrow, one run of " +
		"letters followed by one run of digits, which is why \"a9z\", \" a\" " +
		"and \"1.5\" are all stepped as numbers.",

	"nextMatch": "Walking a string one match at a time. The regexp package " +
		"has no cursor, so the position has to be carried by the caller and " +
		"the search runs over the remainder of the string. That means \"^\" " +
		"matches at the cursor rather than at the start of the whole string, " +
		"which is what a scan usually wants and is worth knowing about.",

	"refKind": "Asking a value what it is, for the cases where the answer " +
		"is not already in its declared type. reflect is the only way to ask, " +
		"and the answer is a word rather than a type, so nothing can be done " +
		"with the value afterwards without a further assertion. A type switch " +
		"answers the question and hands over the typed value at the same " +
		"time, and it is what this should become wherever the possible types " +
		"are known.",

	"replaceFirst": "The regexp package replaces every match or none: there is " +
		"no replace-the-first-one call. A substitution without the /g modifier " +
		"changes only the first match, so the first match is expanded by hand " +
		"and the rest of the string is copied through untouched.",

	"readLines": "Reading a handle in list context yields every line at once, " +
		"newlines included. bufio.Scanner strips the newline and reads one line " +
		"at a time, so neither half of that matches without help. Prefer the " +
		"scanner when the whole input does not need to be in memory.",

	"mod": "Go's % takes its sign from the left operand and Perl's takes it " +
		"from the right one, so -7 % 3 is -1 in Go and 2 in Perl. Negative " +
		"operands are common in wrap-around arithmetic, which is exactly " +
		"where the difference shows.",

	"ord": "The code point of the first character, and zero for the empty " +
		"string. s[0] panics on the empty string and yields a byte, not a " +
		"code point, on everything outside ASCII.",

	"parseNum": "Reading a number out of a string never fails: the longest " +
		"numeric prefix wins and anything else is zero. strconv.ParseFloat " +
		"rejects the empty string, anything with a space in front of it and " +
		"anything with trailing text, and it does so with an error that the " +
		"surrounding expression has nowhere to put.",

	"powInt": "Raising an integer to an integer power stays exact for the " +
		"whole range of an int. math.Pow works in float64 and starts losing " +
		"low bits above 2^53, so it returns a value that is close to the " +
		"answer rather than the answer.",

	"repeatList": "The same rule for a list, where slices.Repeat also panics " +
		"on a negative count. The elements are copied, so the result never " +
		"shares storage with the original slice.",

	"repeatStr": "Repeating a string a negative number of times gives the " +
		"empty string, where strings.Repeat panics. Counts are frequently " +
		"computed, so a count of -1 is a normal input rather than a bug.",

	"reverseStr": "Reversing a string reverses its characters. Reversing its " +
		"bytes takes every multi-byte character apart and produces a string " +
		"that no longer decodes.",

	"seq": "An inclusive range, where every Go loop and slice bound is half " +
		"open. The last value is part of the list, and a start above the end " +
		"gives nothing rather than counting backwards.",

	"splitPattern": "regexp.Split is a different function with the same name: " +
		"it discards the text captured by groups, keeps the empty fields at " +
		"the end, and reads its count the other way round, since a count of " +
		"zero returns nothing at all there and means unlimited here.",

	"sprintf": "Perl's sprintf accepts an argument that does not match its " +
		"conversion, so %d on \"12abc\" is 12, and it has conversions fmt " +
		"does not: positional %2$s, vector %vd, and %g defaulting to six " +
		"significant digits. fmt.Sprintf would answer %!d(string=12abc).",

	"strInc": "Incrementing a string of letters and digits carries within " +
		"each character's own range, so \"Az\" becomes \"Ba\" and \"zz\" " +
		"becomes \"aaa\". Any other string is incremented as a number " +
		"instead. There is no Go operator that does either.",

	"strRange": "A range between two strings is walked with strInc rather " +
		"than counted, and it stops when a value equals the end or would grow " +
		"longer than it. A loop that compared with < instead would never stop " +
		"for ends that are not in one sequence, such as \"b\" to \"a\".",

	"substr": "A negative offset counts back from the end, a negative length " +
		"stops short of it, and a window that runs off the string is clipped " +
		"to what overlaps. The equivalent Go slice expression panics on all " +
		"three.",

	"substrFrom": "The two-argument form of the same operation: from an " +
		"offset to the end, with the same negative offsets and the same " +
		"clipping instead of a panic.",

	"substrReplace": "Assigning to a substring replaces the middle of a " +
		"string in place. Go strings cannot be written to, so the helper " +
		"returns the new string and the caller assigns it back over the old " +
		"one.",

	"toNum": "The conversion parseNum performs, for a value whose type is " +
		"only known while the program runs, so that a number, a string that " +
		"looks like one, a bool and a missing value all end up as the number " +
		"the original program would have used.",

	"toText": "How a value becomes text: a missing value is empty, true is " +
		"\"1\" and false is empty, and a number follows the fifteen-digit " +
		"rule. fmt.Sprint would print <nil>, true and false instead, none of " +
		"which the original program would ever have produced.",

	"truthy": "The empty string and the single character \"0\" are false and " +
		"every other string is true, including \" \", \"00\" and \"0.0\". " +
		"Testing s != \"\" gets \"0\" wrong, and testing the number gets " +
		"\"0.0\" wrong.",

	"ucFirst": "Only the first character changes case. strings.ToUpper would " +
		"upper the whole string, and s[0] would work on a byte instead of a " +
		"character.",
}

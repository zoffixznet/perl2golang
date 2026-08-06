package runtime

// notes explains, for each helper, the rule it implements and the shorter Go
// that would get that rule wrong. EmitAnnotated puts these at the top of the
// helper file it writes for the annotated program, and the report quotes them
// when it says which helpers a program needed.
//
// A note is prose, wrapped when it is printed, and is keyed by the primary
// name of the declaration. Every helper has one; a test enforces that.
var notes = map[string]string{
	"as": "A plain type assertion stops the program the moment the guess " +
		"is wrong, and a value of no fixed type is where the guess can be " +
		"wrong. The two-result form asks rather than insists, so a value " +
		"that turns out to be something else leaves an empty collection or " +
		"a nil record instead of a stack trace, and the lines below it " +
		"still run.",

	"runProgram": "exec.Command takes the program and its arguments " +
		"separately and starts the program directly, so nothing re-splits an " +
		"argument that has a space or a semicolon in it. That is the whole " +
		"difference between the list form and the string form, and it is the " +
		"reason the list form is the one to write.",

	"runShell": "A command written as one string was taken apart by a shell, " +
		"which split the words, expanded the globs and would have run " +
		"whatever a semicolon introduced. Naming the shell here says that out " +
		"loud instead of pretending the words were separate.",

	"captureProgram": "Output collects what the program wrote to standard " +
		"output and leaves its standard error joined to this program's, which " +
		"is where it went before. The error and the output come back " +
		"separately, so a failure cannot be mistaken for empty output.",

	"captureShell": "The string form of captureProgram, for a command line " +
		"that was already written as text.",

	"captureLines": "A command read where a list was wanted gives one string " +
		"per line, with the newline still on the end, which is why the loops " +
		"that use it chomp. A last line with no newline is still a line.",

	"openRead": "Starting a program and reading its output is three calls " +
		"rather than one: the pipe, the start, and the wait at the end. The " +
		"order matters, because waiting before the output has been read " +
		"closes the pipe under the reader.",

	"openWrite": "The writing end of the same idea. Closing the handle is " +
		"what tells the program there is no more input, and only then can it " +
		"finish.",

	"pipeHandle": "A program running alongside this one, wrapped so that it " +
		"reads and writes like a file. The wrapper exists to own the wait: " +
		"Close finishes the conversation and then waits, which keeps the one " +
		"ordering rule in one place instead of at every call site.",

	"waitStatus": "The exit code in the high byte is the shape the original " +
		"read, so the code that shifts it right by eight goes on working. New " +
		"code should ask ExitCode directly. A child killed by a signal is the " +
		"case this cannot reproduce: the standard library reports the code as " +
		"-1 without saying which signal.",

	"permuteArgs": "flag.Parse stops at the first argument that is not an " +
		"option, and the parser this replaces carried on and sorted the two " +
		"apart. That difference is silent: `prog input.txt --verbose` leaves " +
		"--verbose among the file names, the option never takes effect, and " +
		"nothing is printed on either stream. Reordering the arguments first " +
		"is what keeps the command line meaning what it used to.",

	"permutePassThrough": "A parser told to pass unknown options through " +
		"leaves them in the argument list and carries on, which is how a " +
		"wrapper script forwards its caller's options to the command it runs. " +
		"flag stops at the first option it does not know, so an unknown one " +
		"is moved in with the operands before parsing and comes back out " +
		"among the leftovers, in the order it was written.",

	"permute": "Both reorderings are the same walk over the arguments, with " +
		"one decision differing: whether an option the flag set does not know " +
		"is left where it is for flag to complain about, or treated as an " +
		"operand and passed through.",

	"unbundleArgs": "flag reads -vvq as one option named vvq and reports it " +
		"as unknown, where a bundling parser reads it as three. Splitting the " +
		"runs before parsing keeps a command line that has always worked " +
		"working. A run is only taken apart when every letter in it is a " +
		"registered option, so a genuinely unknown -xyz is still reported as " +
		"itself rather than as three separate mysteries.",

	"unbundle": "Bundled letters end at the first one that takes a value: " +
		"everything after it is that value, which is what makes -j4 mean -j 4 " +
		"and -vvq mean three separate options.",

	"takesNoValue": "Whether an option takes a value decides whether the " +
		"word after it belongs to it. flag knows, through the IsBoolFlag " +
		"method its boolean values carry, and asking it is what keeps the " +
		"reordering in step with the parsing.",

	"flagGiven": "A bool that was never mentioned and one explicitly set to " +
		"false are the same value, so the zero value cannot answer whether an " +
		"option was given. Visit walks only the options that were set, which " +
		"is the whole question.",

	"intOption": "flag.IntVar reads a leading zero as octal and 0x as " +
		"hexadecimal, so a zero-padded number out of a config file or a " +
		"crontab would parse as a different number with nothing said about " +
		"it. This reads base ten and nothing else, and takes the underscore " +
		"separators a long number is often written with.",

	"floatOption": "flag.Float64Var accepts inf, nan and hexadecimal floats, " +
		"none of which the original parser would have taken. Rejecting them " +
		"here keeps a typo an error rather than a very large number.",

	"countOption": "Counting how many times an option was given has no " +
		"equivalent in flag: a bool records only that it appeared. The " +
		"function form is called once per occurrence, which is exactly the " +
		"count.",

	"negatableBool": "One option that can be spelled three ways is three " +
		"registrations against one variable. flag has no negation and no " +
		"aliases, but a second name pointing at the same destination is " +
		"ordinary usage, so this costs nothing but the lines.",

	"stringListOption": "A repeatable option collects its values, and flag " +
		"has no repeating kind. The function form is called once per " +
		"occurrence, in command-line order, so appending is the whole " +
		"implementation.",

	"stringMapOption": "The keyed form of a repeatable option. Splitting on " +
		"the first equals sign rather than all of them is what lets a value " +
		"contain one, and a missing separator is an error rather than a key " +
		"with an empty value.",

	"intMapOption": "stringMapOption for a numeric value. The number is " +
		"parsed once, here, so every later use of the map is arithmetic " +
		"rather than a conversion.",

	"optionalString": "An option whose value may be left off has to be " +
		"registered as a boolean one, and flag will not let a boolean option " +
		"take the next word. Only the attached form, --tag=x, means the same " +
		"thing as it did before; --tag x leaves x among the operands.",

	"sortedPairs": "Go randomises map iteration deliberately, so anything " +
		"printed straight from a map changes order between runs. Sorting the " +
		"keys is what keeps the output diffable.",

	"ptr": "A pointer type is how Go says a value may be absent, and it is " +
		"the shape a collection takes once the program has put nothing into " +
		"one of its slots. The helper exists because Go will not take the " +
		"address of a literal or of a map element, so the value needs a " +
		"variable of its own to point at.",

	"deref": "Reading a *T where a T is wanted is one line of Go, and it is " +
		"the line that crashes when the pointer is nil. Treating absent as " +
		"empty is usually what the original did, so the helper says that " +
		"once rather than at every read.",

	"replaceFirstFunc": "The computed-replacement form of replaceFirst. The " +
		"function is handed the matched text and nothing else, so a " +
		"replacement that reads a capture group has to find the groups again " +
		"inside it.",

	"grow": "Assigning past the end of a Perl array extends it and fills the " +
		"gap with undef. A Go slice has a length and a write past it panics, " +
		"so the growth is a step of its own. One append with a make of the " +
		"right size does it in a single reallocation, where a loop appending " +
		"one element at a time would do several.",

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

	"pick": "Perl picks several elements at once and gets a list back. Go's " +
		"index expression takes one index, and its slice expression takes a " +
		"contiguous range, so a scattered or computed set of indexes is a loop. " +
		"The loop goes through at, which is what makes a negative index and an " +
		"index past the end behave the way the original did.",

	"pickKeys": "The keyed form of pick. Reading a key a Go map does not hold " +
		"yields the value type's zero value rather than undef, so a slice of " +
		"keys that are not all present comes back the same length as the key " +
		"list, with zero values in the gaps.",

	"intList": "A list of indexes that came out of a dynamic value is a list " +
		"of any, and Go will not index a slice with one. Each element is read " +
		"as a number the way Perl reads it, once, so the change of type is " +
		"visible in one place instead of at every use.",

	"strList": "The same as intList for keys: a dynamic value used as a map " +
		"key has to become a string first, and Perl was doing that silently.",

	"anyList": "A slice of one element type is not a slice of any, and Go " +
		"has no conversion between them: the memory layouts differ. The copy " +
		"is the only way across, and it means the two no longer share " +
		"anything, which is worth knowing wherever one of them is written to.",

	"anyMap": "The same as anyList for a keyed collection, and the same " +
		"consequence: the result is a separate map, so writing to one does " +
		"not show up in the other.",

	"countChars": "Counting how many characters of a string come from a " +
		"given set. strings.Count counts occurrences of one substring and " +
		"strings.ContainsAny only reports whether any of them appear, so " +
		"neither answers this without a loop around it.",

	"mapChars": "strings.NewReplacer covers the plain character-for-" +
		"character case and is faster, because it builds a matcher once and " +
		"reuses it. The three switches here change what the plain case " +
		"means, and none of them has a counterpart in the strings package.",

	"countOther": "strings.ContainsRune asks about one character at a time, " +
		"which is the whole of this: there is no call that counts how many " +
		"of a string's characters are outside a set.",

	"programPath": "os.Executable returns an error because not every system " +
		"can answer, and it reports where the built binary is rather than " +
		"where any source file was. There is no script at run time to find.",

	"programDir": "The directory a program is in is rarely what a Go program " +
		"wants: data it needs is embedded with go:embed or passed in, because " +
		"a binary is meant to be copied somewhere else and still work.",

	"jsonCodec": "json.Marshal and json.Unmarshal are functions, and the " +
		"common case needs no object at all. Two of the settings a Perl " +
		"encoder is asked for are already true here: map keys come out " +
		"sorted, so output is stable without asking, and every number " +
		"arrives as a float64 whatever it looked like in the text.",

	"newJSONCodec": "The codec exists only because the settings were chosen " +
		"once and used in several places. A single call site is better " +
		"written as json.Marshal directly.",

	"digest": "hash.Hash is the same idea with better names: a hash is an " +
		"io.Writer, so io.Copy from a file into it is the whole of hashing a " +
		"file, and there is no separate call for that case.",

	"newDigest": "crypto/md5 is in the standard library and MD5 is not " +
		"suitable for anything security-related. crypto/sha256 is the same " +
		"shape and is what new code should use.",

	"hashHex": "md5.Sum takes and returns a fixed-size array rather than a " +
		"slice, which is why the [:] is needed to hand it to the encoder.",

	"hashBase64": "The padding is trimmed because the encoders that produce " +
		"these checksums leave it off, and a padded string will not compare " +
		"equal to an unpadded one.",

	"toBase64": "encoding/base64 emits one unbroken line, where a mail " +
		"encoder wraps at 76 characters and ends with a line break. The " +
		"difference is invisible until something compares two encodings.",

	"fromBase64": "The decoder rejects the line breaks an encoder added, so " +
		"they have to come out first. Decoding without padding accepts both " +
		"the padded and the unpadded spelling.",

	"callFn": "Go will not call an interface value: it has to be asserted " +
		"to a function type first, and the assertion names the whole " +
		"signature, so a table holding two functions that differ in one " +
		"argument cannot be called through one assertion. Giving the " +
		"collection a function type removes this entirely, and the call " +
		"becomes a direct one the compiler checks.",

	"fitTo": "The original turned an argument to whatever the sub wanted " +
		"without saying so. Doing the same through reflection is the price " +
		"of not knowing the signature; a declared function type makes the " +
		"compiler do it instead, at no cost at all.",

	"flatPairs": "Go randomises map iteration on purpose, so a list built " +
		"from a map without sorting first changes between runs. Sorting here " +
		"costs a little and makes the result diffable, which is what a list " +
		"built this way is nearly always for.",

	"scalarOf": "A Go function has one return type and a caller cannot ask " +
		"for a different one, so nothing here needs a notion of context. This " +
		"exists only for values whose type did not resolve, where the choice " +
		"between a collection and its length cannot be made at compile time.",

	"asList": "A list in this program is flat: a slice written into one " +
		"contributes its elements, not itself. When the value's type is only " +
		"known while the program runs, whether it has elements to contribute " +
		"has to be asked then too, which is the one question this answers.",

	"tailFrom": "Unpacking a list into variables tolerates a short list: the " +
		"variables past the end are left empty rather than stopping the " +
		"program. Go's own xs[i:] treats an i past the end as a mistake and " +
		"panics, so the tail of a list that may come up short goes through " +
		"this instead.",

	"fileStat": "os.Stat returns an fs.FileInfo and an error, and every " +
		"field is a method on it: Size, Mode, ModTime, IsDir. Nothing hands " +
		"back a list of thirteen numbers, so nothing has to remember which " +
		"index is which, and the ones that are not portable are simply not " +
		"there.",

	"modeBits": "Go splits a file mode into permission bits and type bits, " +
		"and asks about the type with named constants rather than with a " +
		"mask: fi.Mode()&fs.ModeSymlink != 0 is the readable form of what a " +
		"status call packed into one number.",

	"isLink": "os.Stat follows a symbolic link and answers about the target, " +
		"which is why -l needs os.Lstat and why the two disagree only when " +
		"you use the right one of them.",

	"readLink": "os.Readlink returns the target and an error, and a path " +
		"that is not a link is an error rather than an empty answer, which " +
		"is the more useful of the two behaviours.",

	"removeFiles": "os.Remove takes one path and returns an error, where a " +
		"count says only how many went and not why the rest stayed. The " +
		"count is the older shape; testing the error is the one that can act " +
		"on the answer.",

	"setMode": "os.Chmod takes one path and reports an error, where the " +
		"original took a list and reported a count. Permissions are written " +
		"in octal because that is what they are, and Go spells that 0o644.",

	"setTimes": "os.Chtimes takes time.Time values rather than whole " +
		"seconds, which is the same information with the unit attached.",

	"makeTree": "os.MkdirAll creates every missing level and reports only an " +
		"error, because that is all a caller usually needs. Which levels were " +
		"new is a separate question, and answering it means looking before " +
		"the call rather than after it.",

	"removeTree": "os.RemoveAll deletes the tree and reports an error alone. " +
		"Counting what went means walking it first, which is worth doing only " +
		"when something prints the number.",

	"tempDir": "os.MkdirTemp has no cleanup option: `defer os.RemoveAll(dir)` " +
		"is where a Go program says when the directory goes, and it says it " +
		"at the point the directory is made rather than at the end of a " +
		"scope somewhere else. Inside a test, t.TempDir() does both.",

	"tempFile": "os.CreateTemp returns the open file, and its name is " +
		"f.Name() rather than a second result. The random part goes where a " +
		"* appears in the pattern, which is how a suffix is asked for.",

	"timeParts": "time.Time answers each of these with its own method, and " +
		"a Go program calls the one it wants rather than taking nine numbers " +
		"and indexing. The two offsets are why the list form is worth " +
		"avoiding: the month is one less than its name and the year is 1900 " +
		"less than itself.",

	"stdOffset": "A zone's offset changes over the year, so asking a single " +
		"moment for it says nothing about whether that moment is inside " +
		"daylight saving. Comparing against the quieter half of the year is " +
		"what answers it.",

	"timeFrom": "time.Date takes the month as a time.Month and the year as " +
		"the year, so the two offsets have to be undone here. It also " +
		"normalises out-of-range values rather than refusing them, which is " +
		"how you add a month to the 31st and land where you expect.",

	"timeText": "A layout is written as an example timestamp rather than as " +
		"percent codes, and the example is always the same instant: " +
		"Mon Jan 2 15:04:05 MST 2006, which is 1 2 3 4 5 6 7 in order. The " +
		"underscore in _2 asks for space padding rather than zero padding.",

	"formatStamp": "Most percent codes have a layout and should be written " +
		"as one, because the layout is checked by eye and the code is not. " +
		"This exists for the few that have no layout at all, the day of the " +
		"year and the week number among them, and for a format that is only " +
		"known while the program runs.",

	"stampLayouts": "The mapping from percent codes to layout fragments, " +
		"kept beside the formatter that reads it.",

	"looksLikeNumber": "strconv.ParseFloat is the whole answer for text, and " +
		"it returns an error rather than a zero, which is the difference " +
		"between \"not a number\" and \"the number zero\". A value that is " +
		"already numeric needs no test at all: its type says so.",

	"dumpValues": "%#v writes Go syntax, with the types spelled out and map " +
		"keys in sorted order, which is why two dumps of equal maps compare " +
		"equal. Nothing reads that syntax back, so a dump used as a wire " +
		"format wants encoding/json instead.",

	"splitPath": "filepath.Split returns the directory and the final " +
		"component and stops there, because a volume is a Windows idea that " +
		"filepath.VolumeName answers separately. Keeping all three in one " +
		"result is what makes the three-way split portable.",

	"pathParts": "strings.Split on the separator keeps the empty components " +
		"that filepath.Clean would remove, which is what makes a leading " +
		"separator visible as an empty first element rather than vanishing.",

	"relPath": "filepath.Rel reports an error rather than an empty string " +
		"when no relative path exists, which happens when the two paths sit " +
		"on different roots or only one of them is absolute. Falling back to " +
		"the target keeps a printable answer in that case.",

	"absPath": "filepath.Abs is purely textual and succeeds for a path that " +
		"is not there; filepath.EvalSymlinks touches the disk and is what " +
		"makes a missing path detectable. Doing both, in that order, is what " +
		"resolves a relative path through symlinks.",

	"workingDir": "os.Getwd returns an error, because the directory a " +
		"process is in can be removed while it runs. Collapsing that to \"\" " +
		"loses the reason, so anything that must act on it should call " +
		"os.Getwd directly.",

	"absFrom": "filepath.Abs joins a relative path onto the working " +
		"directory and cleans the result, and it never touches the disk, so " +
		"it answers for a path that is not there. Following symbolic links " +
		"is a separate step, and a separate decision.",

	"parseFilename": "filepath.Ext takes everything after the last dot and " +
		"has no notion of a list of acceptable suffixes, so \"access.log.1\" " +
		"gives \".1\". Matching a pattern against the end of the name is how " +
		"you say which extension you meant.",

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

	"splice": "One call that removes, inserts and replaces has no Go " +
		"equivalent: slices.Delete, slices.Insert and slices.Replace each do " +
		"one of those jobs and none of them hands back what it took out. The " +
		"pointer is not decoration either. A slice header is a value, so a " +
		"function given []T can change the elements but cannot make the " +
		"caller's slice shorter.",

	"floatList": "A slice holds one element type, so a list of whole " +
		"numbers used where fractions are needed is a new slice rather than " +
		"a reinterpretation of the old one. The copy is the visible price of " +
		"the guarantee that every element really is a float64.",

	"typedList": "Going from a collection of values of no fixed type back " +
		"to one concrete element type is a copy with an assertion per " +
		"element, because the two are laid out differently in memory. An " +
		"element that turns out not to be a T lands as T's zero value rather " +
		"than stopping the program, which keeps one odd element from losing " +
		"the rest.",

	"typedMap": "The keyed form of typedList, with the same per-value " +
		"assertion and the same choice to keep going past a value that does " +
		"not fit.",

	"flipFlop": "The scalar range operator is a stateful toggle, not a " +
		"range: it remembers, across evaluations, whether an earlier one " +
		"turned it on. Go has no expression with a memory, so the state is a " +
		"variable declared next to the code, one per written occurrence of " +
		"the operator, which also makes the hidden state visible.",

	"hexNum": "hex takes the longest run of hex digits and never fails: " +
		"\"1g\" is 1 and \"g\" is 0. strconv.ParseInt would reject both with " +
		"an error, which is the better behaviour to move to once the inputs " +
		"are trusted to be numbers.",

	"octNum": "The prefix picks the base, the way character escapes spell " +
		"it: 0x is hex, 0b binary, 0o or nothing octal. ParseInt with base 0 " +
		"is close but not the same: it reads \"10\" as decimal where oct " +
		"read it as octal, and it rejects trailing junk instead of stopping " +
		"at it.",

	"radixNum": "The digit loop hexNum and octNum share: digits accumulate, " +
		"underscores are skipped, junk ends the read.",

	"readChunk": "Reading an exact number of bytes is io.ReadFull, not Read: " +
		"a plain Read may hand back fewer bytes than the buffer holds with no " +
		"error, which works on a file and quietly breaks on a pipe. The " +
		"original's read returned undef on failure and nearly nothing checked " +
		"it, so a failure here comes back as the same thing a short file " +
		"gives.",

	"tellPos": "tell has no direct counterpart: the position is asked for by " +
		"seeking zero bytes from where the handle already is. The -1 for a " +
		"handle that cannot say is the same answer tell gave.",

	"readAll": "Reading a whole handle in one piece is one call here, and it " +
		"leaves nothing set behind it. The mode this replaces was a global, so " +
		"forgetting to put it back changed how a later read behaved somewhere " +
		"else in the program.",

	"readRecords": "Splitting a whole input on a separator is the same work a " +
		"record-separated read did, said in one place rather than by a global " +
		"the read consulted. Each record keeps the separator that ended it, " +
		"which is what the original did too.",

	"paragraphs": "Paragraph mode is not simply splitting on a blank line: " +
		"leading blank lines are skipped, a run of them counts once, and each " +
		"paragraph keeps a single trailing blank line. Splitting on \"\\n\\n\" " +
		"alone gets all three of those wrong.",

	"readLines": "Reading a handle in list context yields every line at once, " +
		"newlines included. bufio.Scanner strips the newline and reads one line " +
		"at a time, so neither half of that matches without help. Prefer the " +
		"scanner when the whole input does not need to be in memory.",

	"notImplemented": "The stand-in for a construct the conversion refused. " +
		"Go has no expression that means \"nothing was written here yet\", and " +
		"the obvious spelling, a panic, would make every correctly converted " +
		"line below it unreachable. Returning the zero value of the type the " +
		"position wants keeps the rest of the program running, and saying so " +
		"on stderr keeps the gap from passing for a result.",

	"notImplementedHere": "The statement form of notImplemented, for a step " +
		"whose whole effect was a side effect. Dropping it silently would " +
		"change what the program means with nothing to show for it, and " +
		"panicking would end the run before the converted statements after it " +
		"got to prove themselves.",

	"saidGaps": "One gap is one piece of work whether the line runs once or a " +
		"million times, so each is named once. sync.Map rather than a plain " +
		"map because a converted program may start goroutines, and a map " +
		"written from two of them at once is a crash rather than a race the " +
		"program survives.",

	"unpackTemplate": "Perl's unpack is a whole binary-and-fixed-width parser " +
		"driven by a template string, and Go has no counterpart to the " +
		"template: encoding/binary reads integers one at a time with the byte " +
		"order written at the call, and a fixed-width text field is a slice " +
		"expression. The helper interprets the template the records were " +
		"designed around, and a code outside the set it documents stops the " +
		"program at the call that used it, naming the template.",

	"packTemplate": "The writing half of unpackTemplate, for the same reason: " +
		"the record layout lives in a template string the program was built " +
		"around. New Go code writes binary data with encoding/binary and " +
		"fixed-width text with fmt's width flags, %-10s and %08d.",

	"packItem": "One code of a template, held as data so both directions " +
		"interpret the template the same way.",

	"packItems": "The template scanner both directions share. Refusing an " +
		"unknown code up front is what keeps a typo in a template from " +
		"quietly producing a record with the wrong shape.",

	"packSize": "How wide each integer code is, written once rather than in " +
		"every case of both switches.",

	"unpackInt": "The integer codes differ in two ways only, width and byte " +
		"order, so one function decodes them all. The signed codes " +
		"sign-extend from their own width, which is why int8 and int16 appear " +
		"before the widening.",

	"packInt": "The encoding half of unpackInt. encoding/binary's Append " +
		"functions take the integer already narrowed, so the wrap a too-wide " +
		"value gets is the conversion to uint16 or uint32, visible in the " +
		"code.",

	"hexNibble": "One hex digit to its value. strconv wants a whole string " +
		"and an error path, which is more machinery than a digit needs here.",

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

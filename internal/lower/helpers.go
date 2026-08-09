package lower

// Names of the runtime support functions the generated code may call. They are
// emitted as source into the generated program, and only the ones actually
// used are emitted.
//
// The names are ordinary Go names on purpose. The clean program must read as
// though a Go developer wrote it, so nothing in it may be named after the
// language it came from.
const (
	hFormatNum        = "formatNum"     // float64 -> string, Perl's number stringification
	hParseNum         = "parseNum"      // string -> float64, Perl's leading-numeric-prefix rule
	hToText           = "toText"        // any -> string
	hToNum            = "toNum"         // any -> float64
	hIsTrue           = "isTrue"        // any -> bool, Perl's truthiness
	hTruthy           = "truthy"        // string -> bool, Perl's truthiness for text
	hMod              = "mod"           // Perl's %, which follows the right operand's sign
	hHexNum           = "hexNum"        // hex(), the longest run of hex digits
	hOctNum           = "octNum"        // oct(), with the prefix picking the base
	hSprintf          = "sprintf"       // Perl-compatible sprintf
	hSubstr           = "substr"        // 4-argument substr with Perl's offset rules
	hSubstrFrom       = "substrFrom"    // 2-argument substr
	hSubstrReplace    = "substrReplace" // 4-argument substr replacement
	hIndexOf          = "indexOf"       // Perl index() including the -1 result
	hLastIndexOf      = "lastIndexOf"   // Perl rindex()
	hSplitPattern     = "splitPattern"  // Perl split() including its trailing-empty rule
	hChop             = "chop"          // Perl chop()
	hStrInc           = "strInc"        // Perl's magic string increment
	hRepeatList       = "repeatList"    // the list form of the x operator
	hSplice           = "splice"        // remove, insert or replace a run of elements
	hReverseStr       = "reverseStr"    // reverse() in scalar context
	hUcFirst          = "ucFirst"
	hLcFirst          = "lcFirst"
	hOrd              = "ord"
	hChr              = "chr" // chr(), one byte below 256
	hPowInt           = "powInt" // ** on two integers, staying integral
	hFileExists       = "fileExists"
	hStrRange         = "strRange" // the string form of the range operator
	hJoinList         = "joinList" // interpolating an array into a string
	hAt               = "at"       // indexing that tolerates a short list, as Perl does
	hTailFrom         = "tailFrom" // the tail of a list, tolerating a short one
	hPtr              = "ptr"      // a value moved somewhere a *T slot can point at
	hDeref            = "deref"    // a *T read as a T, with nil reading as the zero value
	hGrow             = "grow"     // a slice extended to a length, as Perl extends an array
	hSeq              = "seq"      // the numeric range operator as a slice
	hSortedKeys       = "sortedKeys"
	hReplaceFirst     = "replaceFirst"     // s/// without the /g modifier
	hReplaceFirstFunc = "replaceFirstFunc" // the same, with a computed replacement
	hReadLines        = "readLines"        // reading a whole handle as a list of lines
	hReadLine         = "readLine"         // reading one line, or nil at the end
	hReading          = "reading"          // the buffered reader that owns a handle's unread bytes
	hReadAll          = "readAll"          // reading a whole handle as one string
	hReadRecords      = "readRecords"      // reading a whole handle split on a separator
	hReadChunk        = "readChunk"        // an exact-length read through io.ReadFull
	hTellPos          = "tellPos"          // the handle's position, by a zero-byte seek
	hRefKind          = "refKind"          // Perl ref(), answered from the runtime type
	hIsDir            = "isDir"            // -d
	hIsFile           = "isFile"           // -f
	hFileSize         = "fileSize"         // -s
	hIsReadable       = "isReadable"       // -r
	hIsWritable       = "isWritable"       // -w
	hIsExecutable     = "isExecutable"     // -x
	hDirNames         = "dirNames"         // opendir plus readdir, as a list of names
	hNextMatch        = "nextMatch"        // one step of a scalar-context //g scan
	hMatchAhead       = "matchAhead"       // a match with a trailing lookaround
	hReplaceAhead     = "replaceAhead"     // a substitution with a trailing lookaround
	hCountAhead       = "countAhead"       // how many such matches there are
	hAnyList          = "anyList"          // []T -> []any
	hAnyMap           = "anyMap"           // map[K]V -> map[K]any
	hTypedList        = "typedList"        // []any -> []T, asserting each element
	hTypedMap         = "typedMap"         // map[K]any -> map[K]V, asserting each value
	hCountChars       = "countChars"       // tr/// with an empty replacement list
	hCountOther       = "countOther"       // the same with a complemented search list
	hMapChars         = "mapChars"         // tr/// with the c, d and s modifiers
	hPick             = "pick"             // @a[...] where the indices are a computed list
	hPickKeys         = "pickKeys"         // @h{...} where the keys are a computed list
	hIntList          = "intList"          // []T -> []int, for a dynamic list of indices
	hStrList          = "strList"          // []T -> []string, for a dynamic list of keys
	hFloatList        = "floatList"        // []T -> []float64, when the elements have to carry fractions

	hUnpackTemplate = "unpackTemplate" // unpack, interpreting the template at run time
	hUnpackText     = "unpackText"     // unpack for the all-text templates, kept []string
	hPackTemplate   = "packTemplate"   // pack, the writing half of the same interpreter

	hDumpValues = "dumpValues" // a structure rendered as Go source
	hScalarOf   = "scalarOf"   // one value wanted where a list may be
	hAsList     = "asList"     // a list wanted where the value's type is unknown
	hFlatPairs     = "flatPairs"     // a keyed collection read where a list is wanted
	hFlatPairsText = "flatPairsText" // the same for a map of text, kept []string
	hCallFn     = "callFn"     // calling a function held in an untyped value
	hAs         = "as"         // reading an untyped value as a type, without panicking

	// Data formats.
	hJSONCodec    = "jsonCodec"    // the type a configured encoder becomes
	hNewJSONCodec = "newJSONCodec" // and its constructor
	hDigest       = "digest"       // the type an accumulating checksum becomes
	hNewDigest    = "newDigest"    // and its constructor
	hHashHex      = "hashHex"      // the checksum of one string, in hex
	hHashBase64   = "hashBase64"   // the same, in base64
	hToBase64     = "toBase64"     // text as wrapped base64
	hFromBase64   = "fromBase64"   // and back again

	// Time and value inspection.
	hTimeParts       = "timeParts"       // a moment as the nine numbers
	hTimeFrom        = "timeFrom"        // the nine numbers back as a moment
	hTimeText        = "timeText"        // a moment as a readable stamp
	hFormatStamp     = "formatStamp"     // a percent-coded time format
	hLooksLikeNumber = "looksLikeNumber" // would this read as a number

	// Paths.
	hFileStat      = "fileStat"      // the thirteen numbers a status call gives
	hIsLink        = "isLink"        // -l, which must not follow the link
	hReadLink      = "readLink"      // where a symbolic link points
	hRemoveFiles   = "removeFiles"   // unlink over a list of paths
	hSetMode       = "setMode"       // chmod over a list of paths
	hSetTimes      = "setTimes"      // utime over a list of paths
	hMakeTree      = "makeTree"      // a directory and everything above it
	hRemoveTree    = "removeTree"    // a directory and everything under it
	hTempDir       = "tempDir"       // a directory nothing else is using
	hTempFile      = "tempFile"      // a file nothing else is using
	hAbbrevTable   = "abbrevTable"   // Text::Abbrev, unambiguous prefixes to their words
	hCanonPath     = "canonPath"     // File::Spec canonpath, which keeps .. intact
	hSplitPath     = "splitPath"     // volume, directory and final component
	hPathParts     = "pathParts"     // a path broken into its components
	hRelPath       = "relPath"       // one path expressed relative to another
	hAbsPath       = "absPath"       // absolute, with symlinks followed
	hAbsFrom       = "absFrom"       // absolute, resolved against a base
	hWorkingDir    = "workingDir"    // the process's current directory
	hProgramPath   = "programPath"   // where the running binary is
	hProgramDir    = "programDir"    // the directory it sits in
	hParseFilename = "parseFilename" // name, directory and matching suffix

	// The stand-ins for a refusal. They keep a partly converted program
	// runnable: the expression form yields the zero value of the type the
	// position wants and the statement form yields nothing at all, and both
	// name the gap on stderr the first time it is reached.
	hNotImplemented     = "notImplemented"
	hNotImplementedHere = "notImplementedHere"

	// Running another program.
	hRunProgram     = "runProgram"     // start a program from a list of words
	hRunShell       = "runShell"       // start a command line through the shell
	hCaptureProgram = "captureProgram" // the same, keeping standard output
	hCaptureShell   = "captureShell"   // and for a command line
	hCaptureLines   = "captureLines"   // the list-context form of backticks
	hOpenRead       = "openRead"       // a handle reading a program's output
	hOpenWrite      = "openWrite"      // a handle writing a program's input

	// Command line options.
	hPermuteArgs        = "permuteArgs"        // options and operands sorted apart
	hPermutePassThrough = "permutePassThrough" // the same, keeping unknown options
	hUnbundleArgs       = "unbundleArgs"       // -vvq split into -v -v -q
	hFlagGiven          = "flagGiven"          // was this option written at all
	hIntOption          = "intOption"          // a base-ten whole number option
	hFloatOption        = "floatOption"        // a floating-point option
	hCountOption        = "countOption"        // an option that counts occurrences
	hNegatableBool      = "negatableBool"      // --name and --no-name together
	hStringListOption   = "stringListOption"   // a repeatable option
	hStringMapOption    = "stringMapOption"    // a repeatable NAME=VALUE option
	hIntMapOption       = "intMapOption"       // a repeatable NAME=NUMBER option
	hOptionalString     = "optionalString"     // an option whose value may be left off
	hSortedPairs        = "sortedPairs"        // a map of options rendered in name order
)

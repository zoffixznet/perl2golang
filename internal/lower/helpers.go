package lower

// Names of the runtime support functions the generated code may call. They are
// emitted as source into the generated program, and only the ones actually
// used are emitted.
//
// The names are ordinary Go names on purpose. The clean program must read as
// though a Go developer wrote it, so nothing in it may be named after the
// language it came from.
const (
	hFormatNum     = "formatNum"     // float64 -> string, Perl's number stringification
	hParseNum      = "parseNum"      // string -> float64, Perl's leading-numeric-prefix rule
	hToText        = "toText"        // any -> string
	hToNum         = "toNum"         // any -> float64
	hIsTrue        = "isTrue"        // any -> bool, Perl's truthiness
	hTruthy        = "truthy"        // string -> bool, Perl's truthiness for text
	hMod           = "mod"           // Perl's %, which follows the right operand's sign
	hSprintf       = "sprintf"       // Perl-compatible sprintf
	hSubstr        = "substr"        // 4-argument substr with Perl's offset rules
	hSubstrFrom    = "substrFrom"    // 2-argument substr
	hSubstrReplace = "substrReplace" // 4-argument substr replacement
	hIndexOf       = "indexOf"       // Perl index() including the -1 result
	hLastIndexOf   = "lastIndexOf"   // Perl rindex()
	hSplitPattern  = "splitPattern"  // Perl split() including its trailing-empty rule
	hChop          = "chop"          // Perl chop()
	hStrInc        = "strInc"        // Perl's magic string increment
	hRepeatList    = "repeatList"    // the list form of the x operator
	hReverseStr    = "reverseStr"    // reverse() in scalar context
	hUcFirst       = "ucFirst"
	hLcFirst       = "lcFirst"
	hOrd           = "ord"
	hPowInt        = "powInt" // ** on two integers, staying integral
	hFileExists    = "fileExists"
	hStrRange      = "strRange" // the string form of the range operator
	hJoinList      = "joinList" // interpolating an array into a string
	hAt            = "at"       // indexing that tolerates a short list, as Perl does
	hSeq           = "seq"      // the numeric range operator as a slice
	hSortedKeys    = "sortedKeys"
	hReplaceFirst  = "replaceFirst" // s/// without the /g modifier
	hReadLines     = "readLines"    // reading a whole handle as a list of lines
	hRefKind       = "refKind"      // Perl ref(), answered from the runtime type
	hIsDir         = "isDir"        // -d
	hIsFile        = "isFile"       // -f
	hFileSize      = "fileSize"     // -s
	hIsReadable    = "isReadable"   // -r
	hIsWritable    = "isWritable"   // -w
	hIsExecutable  = "isExecutable" // -x
	hDirNames      = "dirNames"     // opendir plus readdir, as a list of names
)

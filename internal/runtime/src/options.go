package src

import (
	"flag"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// This file holds the option-parsing helpers a converted command line needs.
//
// Go's flag package is a deliberately smaller thing than Getopt::Long: one
// name per option, no aliases, no counting, no negation, and no permutation of
// options with operands. Aliases are free, because registering a second name
// against the same destination is ordinary flag usage. The rest is here, and
// every helper writes through a pointer to an ordinary Go variable so that
// nothing downstream has to know an option produced it.

// permuteArgs reorders arguments the way an option parser that permutes does:
// options and their values first, then a "--", then the operands in the order
// they were written.
//
// This is the difference that costs the most and announces itself the least.
// flag.Parse stops at the first argument that is not an option, so `prog
// input.txt --verbose` leaves --verbose sitting among the file names with
// nothing said about it: no error, no warning, and an option that silently
// never took effect. Routing the arguments through here first restores the
// behaviour the script was written against.
//
// The explicit "--" matters: without it an operand beginning with a dash would
// be read as an option once it had been moved to the front. An option the flag
// set does not know is passed through untouched, so the diagnostic comes from
// flag rather than from here.
func permuteArgs(fs *flag.FlagSet, args []string) []string {
	return permute(fs, args, false)
}

// permutePassThrough is permuteArgs for a parser told to pass unknown options
// through instead of failing on them.
//
// An option the flag set does not know is moved in with the operands, so it
// survives parsing untouched and comes back out among the leftovers in the
// order it was written, which is where the original script looked for it.
func permutePassThrough(fs *flag.FlagSet, args []string) []string {
	return permute(fs, args, true)
}

// permute does the work for both, keeping options and their values in front
// of a "--" and everything else behind it.
func permute(fs *flag.FlagSet, args []string, passUnknown bool) []string {
	var opts, operands []string
	for i := 0; i < len(args); {
		a := args[i]
		if a == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if len(a) < 2 || a[0] != '-' {
			operands = append(operands, a)
			i++
			continue
		}
		name := strings.TrimLeft(a, "-")
		if j := strings.IndexByte(name, '='); j >= 0 {
			if passUnknown && fs.Lookup(name[:j]) == nil {
				operands = append(operands, a)
			} else {
				opts = append(opts, a)
			}
			i++
			continue
		}
		f := fs.Lookup(name)
		if f == nil && passUnknown {
			operands = append(operands, a)
			i++
			continue
		}
		opts = append(opts, a)
		i++
		if f != nil && !takesNoValue(f) && i < len(args) {
			opts = append(opts, args[i])
			i++
		}
	}
	if len(operands) > 0 {
		opts = append(opts, "--")
	}
	return append(opts, operands...)
}

// unbundleArgs splits the run-together short options a bundling parser
// accepts: `-vvq` becomes `-v -v -q`, and `-j4` becomes `-j 4`.
//
// The flag package reads `-vvq` as one option named "vvq" and reports it as
// unknown, so a script whose callers have always written `-vv` stops working
// on the first argument. What may be bundled is decided by asking the flag
// set: a run is only split when every letter in it is a registered option,
// which leaves a genuinely unknown `-xyz` to be reported as itself rather than
// as three separate mysteries.
func unbundleArgs(fs *flag.FlagSet, args []string) []string {
	out := make([]string, 0, len(args))
	for i, a := range args {
		if a == "--" {
			return append(out, args[i:]...)
		}
		if len(a) < 3 || a[0] != '-' || a[1] == '-' || strings.IndexByte(a, '=') >= 0 {
			out = append(out, a)
			continue
		}
		if split, ok := unbundle(fs, a[1:]); ok {
			out = append(out, split...)
			continue
		}
		out = append(out, a)
	}
	return out
}

// unbundle takes apart one run of bundled letters, stopping at the first one
// that takes a value: everything after it is that value.
func unbundle(fs *flag.FlagSet, letters string) ([]string, bool) {
	out := make([]string, 0, len(letters))
	for i := range letters {
		f := fs.Lookup(letters[i : i+1])
		if f == nil {
			return nil, false
		}
		out = append(out, "-"+letters[i:i+1])
		if takesNoValue(f) {
			continue
		}
		if rest := letters[i+1:]; rest != "" {
			out = append(out, rest)
		}
		return out, true
	}
	return out, true
}

// takesNoValue reports whether an option is one of the boolean kinds, which
// are the ones written without a value after them.
func takesNoValue(f *flag.Flag) bool {
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

// flagGiven reports whether the named option actually appeared on the command
// line.
//
// It is the answer to `defined $opt{name}` after the options have been parsed,
// which a zero value cannot give: a bool that was never mentioned and one
// explicitly set to false are the same value. Visit walks only the options
// that were set, which is exactly the question being asked.
func flagGiven(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case name, "no" + name, "no-" + name:
			found = true
		}
	})
	return found
}

// intOption registers an option that stores a whole number written in base
// ten, with an optional sign and the underscore separators a long number is
// often written with.
//
// flag.IntVar would accept `017` as octal and `0x1f` as hexadecimal, so a
// zero-padded number copied out of a config file or a crontab would parse as a
// different number than it did before, with nothing said about it.
func intOption(fs *flag.FlagSet, p *int, name, usage string) {
	fs.Func(name, usage, func(s string) error {
		n, err := strconv.ParseInt(strings.ReplaceAll(s, "_", ""), 10, 64)
		if err != nil {
			return fmt.Errorf("integer number expected")
		}
		*p = int(n)
		return nil
	})
}

// floatOption registers an option that stores a floating-point number.
//
// flag.Float64Var would accept `inf`, `nan` and the hexadecimal float syntax,
// none of which the original option parser would have taken.
func floatOption(fs *flag.FlagSet, p *float64, name, usage string) {
	fs.Func(name, usage, func(s string) error {
		clean := strings.ReplaceAll(s, "_", "")
		lower := strings.ToLower(clean)
		if strings.ContainsAny(lower, "xnia") {
			return fmt.Errorf("number expected")
		}
		f, err := strconv.ParseFloat(clean, 64)
		if err != nil {
			return fmt.Errorf("number expected")
		}
		*p = f
		return nil
	})
}

// countOption registers an option that counts how many times it was given,
// which is how a verbosity level is usually spelled.
func countOption(fs *flag.FlagSet, p *int, name, usage string) {
	fs.BoolFunc(name, usage, func(v string) error {
		if v == "" || v == "true" {
			*p++
			return nil
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		*p = n
		return nil
	})
}

// negatableBool registers an option under three names: the plain one, which
// turns it on, and the two negated spellings, which turn it off.
//
// A destination that was never mentioned keeps whatever it started with, so a
// default of true survives until something says otherwise. Telling "never
// asked for" apart from "explicitly off" needs flagGiven, because a bool has
// no third state.
func negatableBool(fs *flag.FlagSet, p *bool, name string, usage string) {
	fs.BoolVar(p, name, *p, usage)
	off := func(string) error { *p = false; return nil }
	fs.BoolFunc("no"+name, "turn off -"+name, off)
	fs.BoolFunc("no-"+name, "turn off -"+name, off)
}

// stringListOption registers an option that may be given more than once and
// collects every value, in the order they were written.
func stringListOption(fs *flag.FlagSet, p *[]string, name, usage string) {
	fs.Func(name, usage, func(v string) error {
		*p = append(*p, v)
		return nil
	})
}

// stringMapOption registers an option whose value is a NAME=VALUE pair, of
// which there may be any number. Only the first equals sign separates them, so
// a value may contain more.
func stringMapOption(fs *flag.FlagSet, p *map[string]string, name, usage string) {
	fs.Func(name, usage, func(v string) error {
		k, val, found := strings.Cut(v, "=")
		if !found {
			return fmt.Errorf("key %q requires a value", k)
		}
		if *p == nil {
			*p = map[string]string{}
		}
		(*p)[k] = val
		return nil
	})
}

// intMapOption is stringMapOption for a NAME=NUMBER pair.
func intMapOption(fs *flag.FlagSet, p *map[string]int, name, usage string) {
	fs.Func(name, usage, func(v string) error {
		k, val, found := strings.Cut(v, "=")
		if !found {
			return fmt.Errorf("key %q requires a value", k)
		}
		n, err := strconv.Atoi(strings.ReplaceAll(val, "_", ""))
		if err != nil {
			return fmt.Errorf("integer number expected")
		}
		if *p == nil {
			*p = map[string]int{}
		}
		(*p)[k] = n
		return nil
	})
}

// optionalString registers an option whose value may be left off, in which
// case it stores the empty string.
//
// This is the one form that cannot be made faithful. An option that may be
// written bare has to be registered as a boolean one, and flag will not let a
// boolean option take the next word: `--tag x` sets the tag to the empty
// string and leaves `x` among the operands, where the original would have
// taken it. Only `--tag=x` means the same thing in both.
func optionalString(fs *flag.FlagSet, p *string, name, usage string) {
	fs.BoolFunc(name, usage, func(v string) error {
		if v == "true" {
			*p = ""
			return nil
		}
		*p = v
		return nil
	})
}

// sortedPairs renders a map of options as NAME=VALUE pairs in name order.
//
// Go randomises map iteration on purpose, so anything printed from a map has
// to be sorted explicitly or the program stops being diffable between runs.
func sortedPairs(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+m[k])
	}
	return out
}

// Command perl2go converts Perl 5 scripts to Go and generates teaching
// material explaining the Go it produced.
//
// Everything this command does lives in [perl2go/internal/cli], so the
// terminal behaviour can be tested without building a binary or spawning a
// process. main is only the hand-off: arguments in, exit status out.
package main

import (
	"os"

	"perl2go/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

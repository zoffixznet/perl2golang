#!/usr/bin/perl
# TRAP: STDOUT is block-buffered when writing to a file or pipe; STDERR
# is unbuffered. Interleaved print/warn lines REORDER under redirection:
# with `2>&1 > file` all err lines land BEFORE all out lines.
# (Go's fmt.Println/os.Stderr are both unbuffered: order is preserved.)
use strict;
use warnings;

print STDOUT "out 1\n";
print STDERR "err 1\n";
print STDOUT "out 2\n";
print STDERR "err 2\n";
print STDOUT "out 3\n";

# On a terminal these five lines alternate (STDOUT is line-buffered to a
# tty). Redirected to one file, the observed order is:
#   err 1, err 2, out 1, out 2, out 3

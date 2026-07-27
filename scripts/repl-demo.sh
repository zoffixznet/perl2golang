#!/bin/sh
# Run a canned session through the real binary and check the transcript.
#
# The session is testdata/repl/tour.pl: a handful of snippets, a multi-line
# sub, one line that does not parse, one construct with no Go equivalent, and
# a few meta commands. Everything the REPL does is visible in the output, so
# the check below is a list of things that must appear in it.
#
# Usage: scripts/repl-demo.sh [path-to-perl2go]
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
bin=${1:-}
if [ -z "$bin" ]; then
	bin="$root/bin/perl2go"
	[ -x "$bin" ] || (cd "$root" && go build -o bin/perl2go ./cmd/perl2go)
fi

session="$root/testdata/repl/tour.pl"
transcript=$(mktemp)
trap 'rm -f "$transcript"' EXIT

"$bin" repl --no-history --color never <"$session" >"$transcript" 2>&1
cat "$transcript"

fail=0
check() {
	if ! grep -qF -- "$1" "$transcript"; then
		echo "MISSING: $1" >&2
		fail=1
	fi
}

check 'perl> my @nums = (3, 1, 4, 1, 5);'
check 'nums := []int{3, 1, 4, 1, 5}'
check 'concepts: '
check ' ...>     my $s = shift;'
check 'func trim('
check 'not added to the session'
check '! P2G8001'
check 'package main'

if [ "$fail" -ne 0 ]; then
	echo "the session did not produce the expected transcript" >&2
	exit 1
fi
echo
echo "the session produced everything expected of it"

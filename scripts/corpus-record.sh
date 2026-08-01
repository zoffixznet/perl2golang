#!/bin/sh
# Records a corpus entry's expected output by running it under real perl, from
# inside its own directory and with its own cmd and stdin, which is exactly how
# the scorecard runs it.
#
# The program is run twice and the two captures compared. An entry whose output
# differs between two runs of the same perl cannot be scored against a byte
# comparison, so it is reported rather than recorded.
#
# Usage: scripts/corpus-record.sh tier2 my-case
set -eu

tier="${1:?usage: corpus-record.sh <tier1|tier2|tier3|tier4|domain> <name>}"
name="${2:?usage: corpus-record.sh <tier1|tier2|tier3|tier4|domain> <name>}"

root="$(cd "$(dirname "$0")/.." && pwd)"
dir="$root/testdata/corpus/$tier/$name"

if [ ! -f "$dir/input.pl" ]; then
    echo "no program at $dir/input.pl" >&2
    exit 1
fi
if ! command -v perl >/dev/null 2>&1; then
    echo "perl is not installed, so there is nothing to record from" >&2
    exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# The entry's own contract: arguments from cmd, standard input from stdin, and
# the entry directory as the working directory so files/... resolves.
args=""
[ -f "$dir/cmd" ] && args="$(cat "$dir/cmd")"
stdin=/dev/null
[ -f "$dir/stdin" ] && stdin="$dir/stdin"

run() {
    # shellcheck disable=SC2086
    (cd "$dir" && perl input.pl $args > "$1" 2> "$2" < "$stdin"; echo $? > "$3")
}

run "$tmp/out1" "$tmp/err1" "$tmp/exit1"
run "$tmp/out2" "$tmp/err2" "$tmp/exit2"

if ! cmp -s "$tmp/out1" "$tmp/out2" || ! cmp -s "$tmp/exit1" "$tmp/exit2"; then
    echo "two runs of $tier/$name disagree, so it cannot be recorded as an expectation:" >&2
    diff "$tmp/out1" "$tmp/out2" >&2 || true
    echo "make it deterministic, or move it to tier4 with an expectation.md" >&2
    exit 1
fi

cp "$tmp/out1" "$dir/expected_stdout"
cp "$tmp/exit1" "$dir/expected_exit"

status="$(cat "$dir/expected_exit")"
echo "recorded $(wc -c < "$dir/expected_stdout") bytes of stdout and exit status $status"

if [ -s "$tmp/err1" ]; then
    if [ -f "$dir/allow_stderr" ]; then
        echo "the program wrote to stderr, which this entry allows:"
    else
        echo "the program wrote to stderr and this entry does not allow it:"
        echo "fix the program, or create $dir/allow_stderr if the output is intended"
    fi
    sed 's/^/  /' "$tmp/err1"
fi

echo "check the manifest row still matches: make score ARGS=\"-tier $tier -only $name\""

#!/bin/sh
# Scaffolds a new corpus entry and, when perl is available, records the
# expected output by actually running the script.
# Usage: scripts/corpus-add.sh tier2 my-new-case
set -eu

tier="${1:?usage: corpus-add.sh <tier1|tier2|tier3|tier4> <name>}"
name="${2:?usage: corpus-add.sh <tier1|tier2|tier3|tier4> <name>}"

root="$(cd "$(dirname "$0")/.." && pwd)"
dir="$root/testdata/corpus/$tier/$name"

if [ -e "$dir" ]; then
    echo "$dir already exists" >&2
    exit 1
fi

mkdir -p "$dir"
cat > "$dir/input.pl" <<'EOF'
#!/usr/bin/perl
use strict;
use warnings;

print "edit me\n";
EOF
: > "$dir/cmd"

if command -v perl >/dev/null 2>&1; then
    (cd "$dir" && perl input.pl > expected_stdout; echo $? > expected_exit)
    echo "recorded expected_stdout and expected_exit by running perl"
else
    echo "perl not found: fill in expected_stdout and expected_exit by hand"
fi

echo "created $dir"

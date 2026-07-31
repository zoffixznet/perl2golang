#!/bin/sh
# Scaffolds a new corpus entry, records the expected output by actually running
# the script when perl is available, and registers the entry in the manifest so
# the scorecard picks it up.
# Usage: scripts/corpus-add.sh tier2 my-new-case
set -eu

tier="${1:?usage: corpus-add.sh <tier1|tier2|tier3|tier4|domain> <name>}"
name="${2:?usage: corpus-add.sh <tier1|tier2|tier3|tier4|domain> <name>}"

case "$tier" in
    tier1|tier2|tier3|tier4|domain) ;;
    *)
        echo "unknown tier $tier: expected tier1, tier2, tier3, tier4 or domain" >&2
        exit 1
        ;;
esac

root="$(cd "$(dirname "$0")/.." && pwd)"
dir="$root/testdata/corpus/$tier/$name"
manifest="$root/testdata/corpus/MANIFEST.json"

if [ -e "$dir" ]; then
    echo "$dir already exists" >&2
    exit 1
fi
if grep -q "\"testdata/corpus/$tier/$name\"" "$manifest"; then
    echo "the manifest already lists $tier/$name" >&2
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

status=0
if command -v perl >/dev/null 2>&1; then
    (cd "$dir" && perl input.pl > expected_stdout; echo $? > expected_exit)
    status="$(cat "$dir/expected_exit")"
    echo "recorded expected_stdout and expected_exit by running perl"
else
    printf 'edit me\n' > "$dir/expected_stdout"
    echo 0 > "$dir/expected_exit"
    echo "perl not found: expected_stdout and expected_exit are guesses, check them by hand"
fi

# Tier 4 entries pass by reporting a construct honestly rather than by
# reproducing its behaviour, and carry an expectation.md saying which.
kind=convert
if [ "$tier" = tier4 ]; then
    kind=honest-failure
    cat > "$dir/expectation.md" <<'EOF'
# What the tool must say about this file

Categories: reported

Replace the line above with the categories this entry accepts, and say below
what a correct report looks like.
EOF
fi

# The manifest is a JSON array, one object per entry, and the loader sorts it,
# so a new entry goes on the end. The last two lines are the closing brace of
# the final entry and the closing bracket of the array.
new="$manifest.new"
sed '$d' "$manifest" > "$new"
{
    sed '$d' "$new"
    printf '  },\n'
    printf '  {\n'
    printf '    "tier": "%s",\n' "$tier"
    printf '    "name": "%s",\n' "$name"
    printf '    "path": "testdata/corpus/%s/%s",\n' "$tier" "$name"
    printf '    "args": [],\n'
    printf '    "has_stdin": false,\n'
    printf '    "has_files": false,\n'
    printf '    "allow_stderr": false,\n'
    printf '    "expected_exit": %s,\n' "$status"
    printf '    "deterministic": true,\n'
    printf '    "kind": "%s"\n' "$kind"
    printf '  }\n'
    printf ']\n'
} > "$manifest.tmp"
mv "$manifest.tmp" "$manifest"
rm -f "$new"

echo "created $dir"
echo "registered $tier/$name in testdata/corpus/MANIFEST.json"
echo "next: write input.pl, then rerun this entry's expectations with"
echo "  (cd $dir && perl input.pl > expected_stdout; echo \$? > expected_exit)"
echo "and check it with: go run ./cmd/score -tier $tier -only $name"

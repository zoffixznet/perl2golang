#!/bin/sh
# Converts a corpus script into a temporary directory, builds both generated
# programs, and runs the clean one, so you can see the whole flow end to end.
set -eu

root="$(cd "$(dirname "$0")/.." && pwd)"
bin="$root/bin/perl2golang"
src="$root/testdata/corpus/tier2/06-array-of-hashes/input.pl"

if [ ! -x "$bin" ]; then
    echo "build the binary first: make build" >&2
    exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

cp -r "$(dirname "$src")/files" "$work/" 2>/dev/null || true
echo "== converting $src"
"$bin" convert "$src" -o "$work/out"

echo
echo "== generated files"
find "$work/out" -type f | sed "s|$work/||" | sort

if command -v go >/dev/null 2>&1; then
    echo
    echo "== building and running the clean program"
    (cd "$work/out" && go build -o prog . && cd "$work" && "$work/out/prog")
else
    echo "go toolchain not found; skipping build-and-run step"
fi

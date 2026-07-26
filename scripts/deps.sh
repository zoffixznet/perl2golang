#!/bin/sh
# Checks for the system tools the Makefile targets rely on. On Debian-like
# systems it offers to install anything missing via apt-get. It never installs
# without showing the exact command first, and it exits successfully when
# nothing is missing.
set -eu

missing_pkgs=""
missing_tools=""

need() {
    tool="$1"
    pkg="$2"
    if ! command -v "$tool" >/dev/null 2>&1; then
        missing_tools="$missing_tools $tool"
        case " $missing_pkgs " in
            *" $pkg "*) ;;
            *) missing_pkgs="$missing_pkgs $pkg" ;;
        esac
    fi
}

need go golang-go
need gofmt golang-go
need make make
need git git
# perl is optional: only the behavioural-equivalence tests use it, and they
# skip cleanly when it is absent. Still worth reporting.
if ! command -v perl >/dev/null 2>&1; then
    echo "note: perl not found. Conversion works without it; only the"
    echo "      behavioural-equivalence tests need it (they skip otherwise)."
fi

if [ -z "$missing_tools" ]; then
    echo "all required tools present"
    exit 0
fi

echo "missing tools:$missing_tools"

if ! command -v apt-get >/dev/null 2>&1; then
    echo "apt-get not found; install the tools above with your package manager."
    exit 1
fi

cmd="sudo apt-get install -y$missing_pkgs"
echo "about to run: $cmd"
$cmd

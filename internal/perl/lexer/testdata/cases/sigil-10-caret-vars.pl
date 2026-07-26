#!/usr/bin/perl
# CASE sigil-10: caret variables. `$^X` is a two-character name; `${^GLOBAL_PHASE}`
# is a braced name whose first character is a literal control-ish `^` marker. The
# `^` inside `${^NAME}` is part of the NAME, not an operator.
use strict; use warnings;

print "sigil-10 global-phase: ${^GLOBAL_PHASE}\n";
print "sigil-10 perl-exe: ", ($^X ne "" ? "set" : "unset"), "\n";
print "sigil-10 os: $^O\n";
print "sigil-10 warnings-bit: ", (defined $^W ? $^W : "undef"), "\n";
print "sigil-10 taint: ", (defined ${^TAINT} ? ${^TAINT} : "undef"), "\n";
print "sigil-10 unicode: ", (defined ${^UNICODE} ? "defined" : "undef"), "\n";
print "sigil-10 utf8cache: ", (defined ${^UTF8CACHE} ? "defined" : "undef"), "\n";

{
    local ${^WARNING_BITS};
    print "sigil-10 warning-bits-localized: ok\n";
}

# In a string, ${^NAME} interpolates.
print "sigil-10 interp: phase=${^GLOBAL_PHASE} os=$^O\n";

# Contrast: `^` as the XOR operator, and `$x ^ $y`.
my ($a, $b) = (0b1100, 0b1010);
printf "sigil-10 xor: %04b\n", $a ^ $b;

# `$^` alone is the format-top-of-page variable name (a legal 2-char name).
print "sigil-10 dollar-caret-name-length: ", length('$^L'), "\n";

#!/usr/bin/perl
# CASE num-01: radix prefixes. `0x`, `0b`, `0o` and a bare leading `0` (octal).
# The prefix letters are case-insensitive and underscores are allowed inside.
use strict; use warnings;

print "num-01 hex:       ", 0x1F, " ", 0X1f, " ", 0xdead_beef, "\n";
print "num-01 binary:    ", 0b1010, " ", 0B1010, " ", 0b1010_1010, "\n";
print "num-01 octal-new: ", 0o17, " ", 0O17, "\n";
print "num-01 octal-old: ", 017, " ", 0_17, "\n";
print "num-01 zero:      ", 0, " ", 00, " ", 0x0, "\n";

# String-to-number conversion is a DIFFERENT rule: "0x1F" numifies to 0.
print "num-01 string-hex: ", 0 + "0x1F", " but hex() gives ", hex("0x1F"), "\n";
print "num-01 oct-fn: ", oct("0x1F"), " ", oct("0b101"), " ", oct("017"), " ", oct("17"), "\n";

# A leading 0 on a decimal-looking literal is octal, which surprises people.
print "num-01 leading-zero-trap: 010 == ", 010, " (not 10)\n";

# 09 is an ILLEGAL octal digit -- compile error.
my $out = `$^X -e 'print 09;' 2>&1`;
$out =~ s/\s+\z//;
print "num-01 illegal-octal: ", ($out =~ /Illegal octal digit/ ? "COMPILE ERROR" : "[$out]"), "\n";

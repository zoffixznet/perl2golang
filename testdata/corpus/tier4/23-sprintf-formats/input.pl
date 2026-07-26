#!/usr/bin/perl
# TRAP: sprintf/printf behaviours Go's fmt cannot reproduce one-to-one:
# %vd vectors, %s float stringification (%.15g), %#b, UV clamping.
use strict;
use warnings;

printf "vector: %vd\n", "1.22.333";        # per-char version vector
printf "float via %%s: %s\n", 0.1 + 0.2;   # Perl %s uses %.15g => "0.3"
printf "full: %.17g\n", 0.1 + 0.2;         # what Go's %v would print
printf "binary: %b / %#b\n", 10, 10;       # %#b prefix: no Go equivalent
printf "neg width: [%*s]\n", -8, "ab";     # runtime negative width
printf "round: %.2f\n", 2.675;             # .67 or .68? binary says .67
{
    no warnings;
    printf "big to %%d: %d\n", 1e20;       # silently clamps to UV max
}
my $ref = [ 1, 2 ];
my $str = sprintf "%s", $ref;              # stringifies to ARRAY(0x...)
printf "ref: %s\n",
    ( $str =~ /^ARRAY\(0x[0-9a-f]+\)$/ ? "ARRAY(0x<addr>)" : "UNEXPECTED:$str" );

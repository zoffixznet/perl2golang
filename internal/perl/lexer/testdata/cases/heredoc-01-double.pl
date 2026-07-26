#!/usr/bin/perl
# CASE heredoc-01: `<<"EOT"` -- double-quoted heredoc, interpolating.
# The body starts on the NEXT physical line; the rest of the current line is
# still ordinary code.
use strict; use warnings;

my $who = "world";
my $t = <<"EOT" . "[tail]\n";
hello $who
line two
EOT
print "heredoc-01 body+tail:\n$t";
print "heredoc-01 length: ", length($t), "\n";

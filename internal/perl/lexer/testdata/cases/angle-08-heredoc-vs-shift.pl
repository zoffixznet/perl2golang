#!/usr/bin/perl
# CASE angle-08: `<<` is a heredoc opener in TERM position and a left-shift
# operator in OPERATOR position. `<<=` is shift-assign. Also `1 << 2` vs `<<"EOT"`.
use strict; use warnings;

my $shifted = 1 << 4;                 # operator position -> left shift
print "angle-08 left-shift: $shifted\n";

my $v = 3;
$v <<= 2;                             # shift-assign, one token
print "angle-08 shift-assign: $v\n";

my $text = <<"EOT";                   # term position -> heredoc
heredoc body
EOT
print "angle-08 heredoc: $text";

# Both on one line: a shift whose result is printed next to a heredoc.
my $both = (2 << 3) . "|" . <<'END';
tail
END
print "angle-08 both: $both";

# `<<` where the left operand is a parenthesised expression (operator position).
print "angle-08 paren-shift: ", ((1+1) << 2), "\n";

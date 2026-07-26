#!/usr/bin/perl
# CASE heredoc-06: two (or three) heredocs opened on ONE line. Bodies follow in
# the order the openers appeared, back to back, after the end of that line.
use strict; use warnings;

my $both = <<"ONE" . "-mid-" . <<"TWO";
body one
ONE
body two
TWO
print "heredoc-06 two:\n$both";

my @three = (<<'A', <<'B', <<'C');
aaa
A
bbb
B
ccc
C
print "heredoc-06 three: ", join("", map { s/\n/,/r } @three), "\n";

# Two heredocs with the SAME terminator on one line: first body ends at the first
# terminator line, second body starts immediately after.
my $same = <<'X' . <<'X';
p
X
q
X
print "heredoc-06 same-terminator: ", ($same =~ s/\n/|/gr), "\n";

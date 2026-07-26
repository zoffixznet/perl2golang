#!/usr/bin/perl
# CASE heredoc-05: the heredoc body begins after the current PHYSICAL line, even
# when the statement continues over several lines. The lexer must queue pending
# heredocs and consume their bodies at the next newline, then resume the
# expression where it left off.
use strict; use warnings;

my $x = join("",
    <<"A",
first
A
    "middle\n",
    <<"B",
second
B
    "last\n");
print "heredoc-05 joined:\n$x";

# A heredoc opened inside a parenthesised expression that spans lines.
my $y = (
    <<"C"
inside parens
C
);
print "heredoc-05 parens: $y";

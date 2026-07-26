#!/usr/bin/perl
# CASE slash-06: `/` after `}` is AMBIGUOUS and depends on what the `}` closed.
#   * `}` closing a hash/array SUBSCRIPT  -> operator state -> division
#   * `}` closing a BLOCK                 -> term state     -> match
# The lexer must know which kind of `{` the brace matched.
use strict; use warnings;

my %h = (a => 10);
my $r = $h{a} / 2;              # subscript close  -> division
print "slash-06 subscript-div: $r\n";

my $rd = $h{a}/2;               # same, no spaces
print "slash-06 subscript-div-tight: $rd\n";

my @l = ("foo", "bar");
my @g = grep { /foo/ } @l;      # block close: the `}` here ends the grep BLOCK
print "slash-06 block-then-list: ", join(",", @g), "\n";

# `}` of a bare block followed by a statement that starts with a match.
{ my $t = 1; }
local $_ = "zzz";
print "slash-06 after-bare-block: ", (/z/ ? "match" : "no"), "\n";

# deref-subscript close, still operator state
my $ar = [8];
print "slash-06 deref-sub-div: ", ${$ar}[0] / 2, "\n";

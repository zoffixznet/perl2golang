#!/usr/bin/perl
# CASE pod-01: `#` starts a comment ONLY outside a quoting context. Inside a
# string, inside a quote-like delimiter, and as a delimiter itself it is data.
use strict; use warnings;

my $a = "has # inside";            # <- this one IS a comment
my $b = 'single # inside';
my $c = q#delimited by hash#;
my $d = qq{brace with # inside};
print "pod-01 a=[$a] b=[$b] c=[$c] d=[$d]\n";

# `#` inside a regex WITHOUT /x is a literal character.
print "pod-01 regex-plain: ", ("a#b" =~ /a#b/ ? "match" : "no"), "\n";

# `#` inside a regex WITH /x starts a comment that runs to end of line.
print "pod-01 regex-x: ", ("ab" =~ /a  # comment
                                    b/x ? "match" : "no"), "\n";
print "pod-01 regex-x-escaped: ", ("a#b" =~ /a \# b/x ? "match" : "no"), "\n";

# `#` inside a character class under /x is still literal.
print "pod-01 regex-x-class: ", ("a#b" =~ /a [#] b/x ? "match" : "no"), "\n";

# A `#` immediately after a sigil is a variable, not a comment: see pod-03.
my @arr = (1,2,3);
print "pod-01 lastindex: $#arr\n";

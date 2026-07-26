#!/usr/bin/perl
# CASE slash-05: `return /x/ ? 1 : 0` -- `/` after the keyword `return` is a MATCH.
# `return` is a term-expecting keyword, so `/` opens a pattern; the `?` that follows
# the closing `/` is then the ternary, not the (removed) ?PATTERN? match.
use strict; use warnings;

sub has_x { local $_ = shift; return /x/ ? 1 : 0 }
print "slash-05 axb: ", has_x("axb"), " abc: ", has_x("abc"), "\n";

# Same shape after other term-expecting keywords.
sub w { local $_ = shift; if (/y/) { return "y" } elsif (/z/) { return "z" } return "-" }
print "slash-05 keywords: ", w("y"), w("z"), w("q"), "\n";

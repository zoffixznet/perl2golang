#!/usr/bin/perl
# CASE stmt-07: a sub's PROTOTYPE changes how later calls to it are PARSED. The
# same characters produce different token groupings depending on a declaration
# that may live in another file. This is the hard limit of static lexing.
use strict; use warnings;

# Declared with ($): takes exactly one scalar, so `one "a", "b"` parses as
# `one("a"), "b"`.
sub one ($) { return "one[" . join("|", @_) . "]" }
# Declared with (@): a list operator, so it swallows everything to the right.
sub many (@) { return "many[" . join("|", @_) . "]" }
# Declared with (&@): takes a BLOCK first, like grep/map.
sub with_block (&@) { my ($cb, @l) = @_; return "blk[" . join("|", map { $cb->($_) } @l) . "]" }

print "stmt-07 scalar-proto: ", join(" + ", one "a", "b"), "\n";
print "stmt-07 list-proto:   ", join(" + ", many "a", "b"), "\n";
print "stmt-07 block-proto:  ", with_block { uc $_[0] } "x", "y";
print "\n";

# `one` with a list argument imposes scalar context on it.
my @l = (1,2,3);
print "stmt-07 scalar-context: ", one(@l), "\n";
# The (@) prototype swallows the trailing "\n" into the argument list, so the
# newline appears INSIDE the joined result rather than after it.
print "stmt-07 list-context:   ", (many @l, "END"), "\n";

# Without the prototype, the SAME source text parses differently.
my $out = `$^X -e 'sub f { "f[".join(q{|},\@_)."]" } print join(q{ + }, f "a", "b");' 2>&1`;
$out =~ s/\s+\z//;
print "stmt-07 no-prototype: [$out]\n";

# A prototype declared AFTER the call site does not apply.
my $out2 = `$^X -e 'print join(q{ + }, g "a", "b"); sub g (\$) { "g[".join(q{|},\@_)."]" }' 2>&1`;
$out2 =~ s/\s+/ /g; $out2 =~ s/\s+\z//;
print "stmt-07 late-prototype: [$out2]\n";

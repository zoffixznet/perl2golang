#!/usr/bin/perl
# CASE stmt-01: `{` starts a BLOCK or an anonymous HASHREF. Perl guesses by
# peeking at the first token(s) inside; `+{` and `{;` are the explicit
# disambiguators. This is a genuine lookahead problem, not a state-machine one.
use strict; use warnings;

my @l = (1,2);

# map with a BLOCK.
my @doubled = map { $_ * 2 } @l;
print "stmt-01 map-block: @doubled\n";

# map with an expression that Perl guesses is a HASHREF ...
my @refs = map { { value => $_ } } @l;
print "stmt-01 map-hashref: ", ref($refs[0]), " value=", $refs[0]{value}, "\n";

# `+{` forces HASHREF.
my @forced_h = map +{ v => $_ }, @l;
print "stmt-01 plus-brace: ", ref($forced_h[0]), " v=", $forced_h[0]{v}, "\n";

# `{;` forces BLOCK.
my @forced_b = map {; "k$_" } @l;
print "stmt-01 semi-brace: ", join(",", @forced_b), "\n";

# The classic ambiguity: `map { $_ => 1 } @l` -- perl guesses BLOCK here
# (because `$_` is a scalar followed by `=>`, it actually guesses hashref-ish and
# fails). Proven in a child so this file still compiles.
my $out = `$^X -e 'my \@l=(1,2); my %h = map { \$_ => 1 } \@l; print join(q{,}, sort keys %h);' 2>&1`;
$out =~ s/\s+/ /g; $out =~ s/\s+\z//;
print "stmt-01 dollar-fatcomma: [$out]\n";

# The same shape with a leading string constant DOES break.
my $out2 = `$^X -e 'my \@l=(1,2); my %h = map { "k" . \$_ => 1 } \@l; print join(q{,}, sort keys %h);' 2>&1`;
$out2 =~ s/\s+/ /g; $out2 =~ s/\s+\z//;
print "stmt-01 string-fatcomma: ", ($out2 =~ /error|odd|Not a/i ? "AMBIGUOUS: $out2" : "[$out2]"), "\n";

# A bare block at statement level is a loop that runs once.
my $n = 0;
{ $n++ }
print "stmt-01 bare-block: $n\n";

# An empty `{}` in term position is an empty hashref.
my $e = {};
print "stmt-01 empty: ", ref($e), " keys=", scalar(keys %$e), "\n";

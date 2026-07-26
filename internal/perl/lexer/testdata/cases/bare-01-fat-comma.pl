#!/usr/bin/perl
# CASE bare-01: a bareword immediately before `=>` is AUTOQUOTED, even if it is a
# keyword, an operator name, or a declared sub. Also `-bareword` becomes the
# string "-bareword", and a bareword inside `{ }` used as a hash subscript is
# autoquoted too.
use strict; use warnings;

sub print_it { "SUBCALL" }

my %h = (
  print   => 1,   # keyword
  if      => 2,   # keyword
  q       => 3,   # quote operator
  x       => 4,   # repetition operator
  y       => 5,   # transliteration operator
  sub     => 6,   # keyword
  print_it=> 7,   # a real sub in scope: still autoquoted
  -minus  => 8,   # leading minus
  Foo     => 9,
);
print "bare-01 keys: ", join(",", map { "$_=$h{$_}" } sort keys %h), "\n";

# Subscript autoquoting.
print "bare-01 subscript: $h{print} $h{if} $h{sub} $h{x}\n";

# `-bareword` in EXPRESSION position also becomes a string.
my $m = -foo;
print "bare-01 minus-bareword: [$m] ref=", (ref(\$m) eq 'SCALAR' ? 'SCALAR' : ref(\$m)), "\n";

# But `-e` and friends are FILE TESTS when followed by a term.
print "bare-01 filetest: ", (-e $0 ? "script exists" : "missing"), "\n";
print "bare-01 minus-e-bareword: [", -e, "]\n" if 0;   # not run: -e with no arg uses $_

# A `::`-QUALIFIED bareword before `=>` is NOT autoquoted: under strict subs it
# is a compile error. Autoquoting applies only to a simple identifier.
my $out = `$^X -e 'use strict; my %q = ( Foo::Bar => 1 ); print join(q{,}, keys %q);' 2>&1`;
$out =~ s/\s+/ /g; $out =~ s/\s+\z//;
print "bare-01 qualified-strict: ", ($out =~ /not allowed while "strict subs"/ ? "COMPILE ERROR" : "[$out]"), "\n";
my $out2 = `$^X -e 'my %q = ( Foo::Bar => 1 ); print join(q{,}, keys %q);' 2>&1`;
$out2 =~ s/\s+\z//;
print "bare-01 qualified-no-strict: [$out2]\n";

# A bareword with a hyphen inside is NOT one token: `a-b => 1` is a subtraction.
my %r = ( 'a-b' => 1 );
print "bare-01 hyphen-key-needs-quotes: ", join(",", keys %r), "\n";

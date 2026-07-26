#!/usr/bin/perl
# CASE quote-04: with bracketing delimiters the lexer must COUNT nesting depth.
# `q{a{b}c}` is one string; `q{a{b}` is unterminated.
use strict; use warnings;

print "quote-04 brace: ", q{a{b}c}, "\n";
print "quote-04 deep:  ", q{L1 {L2 {L3} L2} L1}, "\n";
print "quote-04 paren: ", q(a(b(c)d)e), "\n";
print "quote-04 brack: ", q[a[b]c], "\n";
print "quote-04 angle: ", q<a<b>c>, "\n";

# Escaped delimiters do NOT change the depth.
print "quote-04 escaped: ", q{open \{ still one string}, "\n";

# Nesting inside a regex delimiter.
my $s = "x{1}y";
print "quote-04 regex-brace: ", ($s =~ m{x\{1\}y} ? "match" : "no"), "\n";

# Nesting across a substitution's two parts.
(my $t = "aXb") =~ s{X}{ {mid} };
print "quote-04 subst-nested: $t\n";

# Unbalanced open brace is fatal.
my $out = `$^X -e 'my \$z = q{a{b}; print \$z;' 2>&1`;
$out =~ s/\s+\z//;
print "quote-04 unbalanced: ", ($out =~ /Can't find string terminator/ ? "FATAL" : "[$out]"), "\n";

#!/usr/bin/perl
# CASE sigil-01: `$x`, `$$x`, `${x}`, `${$x}`. The sigil run and the optional
# braces are all part of the variable token; `${x}` is the variable named `x`,
# NOT a dereference of a bareword.
use strict; use warnings;

our $x = "package-x";
my $val = "the value";
my $ref = \$val;

print "sigil-01 plain: ", ${x}, "\n";        # same as $x (package var)
print "sigil-01 deref: $$ref\n";
print "sigil-01 braced-deref: ${$ref}\n";
print "sigil-01 same: ", ($$ref eq ${$ref} ? "yes" : "no"), "\n";

# Double dereference.
my $rr = \$ref;
print "sigil-01 double: ", $$$rr, " / ", ${${$rr}}, "\n";

# `${ EXPR }` with an arbitrary expression inside.
print "sigil-01 expr-block: ", ${ \ "from a block" }, "\n";

# Whitespace inside the braces is allowed.
print "sigil-01 spaced-braces: ", ${   $ref   }, "\n";

# Whitespace BETWEEN the sigil and the name is also allowed.
my $y = "why";
print "sigil-01 sigil-space: ", $ y, "\n";

#!/usr/bin/perl
# CASE sigil-07: `*name` is a typeglob in TERM position and multiplication in
# OPERATOR position. `*{"string"}` is a symbolic glob deref.
use strict; use warnings;

our $v = "scalar";
our @v = ("array");
sub v { "code" }

no strict 'refs';
print "sigil-07 glob-name: ", *v, "\n";
print "sigil-07 via-glob-scalar: ", ${*v{SCALAR}}, "\n";
print "sigil-07 via-glob-array: ", join(",", @{*v{ARRAY}}), "\n";
print "sigil-07 via-glob-code: ", *v{CODE}->(), "\n";

# Glob assignment creates an alias.
*alias = \&v;
print "sigil-07 alias: ", alias(), "\n";

# Symbolic glob with a computed name.
my $name = "dyn";
*{"main::$name"} = sub { "dynamic sub" };
print "sigil-07 symbolic: ", dyn(), "\n";

# Multiplication, in operator position.
print "sigil-07 multiply: ", 6 * 7, "\n";
my $m = 3; $m *= 4;
print "sigil-07 mult-assign: $m\n";
print "sigil-07 power: ", 2 ** 10, "\n";

# The trap: `*` right after `(` or `,` is a glob; after a term it is multiply.
my @mixed = (*STDOUT, 2 * 3);
print "sigil-07 mixed: ", (ref(\$mixed[0]) eq 'GLOB' ? "GLOB" : ref(\$mixed[0])), " / $mixed[1]\n";

#!/usr/bin/perl
# CASE stmt-04: indirect object syntax -- `new Foo(...)` means `Foo->new(...)`.
# The lexer cannot tell `new Foo(1)` from a call to a sub `new` with the bareword
# `Foo` as an argument without knowing what is declared.
use strict; use warnings;
no strict 'subs';

package Widget;
sub new { my ($c, @a) = @_; return bless { args => "@a" }, $c }
sub show { return ref($_[0]) . "(" . $_[0]{args} . ")" }
package main;

my $a = new Widget(1,2);
print "stmt-04 indirect: ", $a->show, "\n";

my $b = Widget->new(1,2);
print "stmt-04 arrow: ", $b->show, "\n";

print "stmt-04 same-class: ", (ref($a) eq ref($b) ? "yes" : "no"), "\n";

# Indirect object with no parens at all.
my $c = new Widget;
print "stmt-04 no-parens: ", $c->show, "\n";

# The classic collision: a sub named `new` in main.
sub make { return "sub-make(" . join(",", @_) . ")" }
print "stmt-04 plain-sub: ", make(Widget), "\n";

# `print STDERR ...` is itself indirect object syntax for the filehandle slot.
print STDOUT "stmt-04 print-indirect-object\n";

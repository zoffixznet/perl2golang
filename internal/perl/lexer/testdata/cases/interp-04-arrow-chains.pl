#!/usr/bin/perl
# CASE interp-04: inside a string, `->` interpolates ONLY when followed by `[`,
# `{`, or a postfix-deref token. `$obj->method` does NOT call the method: the
# arrow and name are literal text after the value of $obj.
use strict; use warnings;

package Obj;
sub new { bless { f => "FIELD", a => [10,20], deep => { x => "DEEPX" } }, shift }
sub meth { "METHOD-RESULT" }
package main;

my $o = Obj->new;

print "interp-04 hash-field: [$o->{f}]\n";
print "interp-04 array-field: [$o->{a}[1]]\n";
print "interp-04 arrow-arrow: [$o->{deep}->{x}]\n";
print "interp-04 no-arrow-between: [$o->{deep}{x}]\n";

my $m = "$o->meth";
print "interp-04 method-not-called: ", ($m =~ /->meth\z/ ? "literal '->meth' kept" : "UNEXPECTED [$m]"), "\n";
print "interp-04 method-needs-trick: [@{[ $o->meth ]}]\n";

my $ar = [1,2,3];
print "interp-04 arrayref-index: [$ar->[0]]\n";

# `->@*` in a string is literal unless the postderef_qq feature is enabled.
my $pd = "$ar->@*";
print "interp-04 postfix-default: ", ($pd =~ /->\@\*\z/ ? "literal ->\@* kept" : "interpolated"), "\n";
{
    use feature 'postderef_qq';
    print "interp-04 postfix-with-feature: [$ar->@*]\n";
}

# `->` at the very end of a string is literal.
my $tail = "$ar->";
$tail =~ s/ARRAY\(0x[0-9a-f]+\)/ARRAYREF/;
print "interp-04 trailing-arrow: [$tail]\n";

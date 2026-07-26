#!/usr/bin/perl
# CASE sigil-08: package-qualified names. `$::x` == `$main::x`. `::` is a name
# separator inside the identifier token, so it must not be lexed as two colons
# (which would collide with labels and the ternary). `'` is a legacy separator.
use strict; use warnings;

$main::x = "main-x";
$Foo::Bar::baz = "deep";
$Foo::Bar::Baz::qux = "deeper";

print "sigil-08 double-colon-shorthand: $::x\n";
print "sigil-08 explicit-main: $main::x\n";
print "sigil-08 same: ", ($::x eq $main::x ? "yes" : "no"), "\n";
print "sigil-08 nested: $Foo::Bar::baz / $Foo::Bar::Baz::qux\n";

# Arrays, hashes and subs qualify the same way.
@Foo::list = (1,2,3);
%Foo::map  = (k => "v");
sub Foo::hello { "hello from Foo" }
print "sigil-08 array: @Foo::list hash: $Foo::map{k} sub: ", Foo::hello(), "\n";
print "sigil-08 lastindex: $#Foo::list\n";

# The legacy apostrophe separator still parses in this perl.
$main'legacy = "apostrophe";
print "sigil-08 apostrophe: $main'legacy (== $main::legacy)\n";

# Ternary colons and a label, for contrast with `::`.
my $t = 1 ? "yes" : "no";
LOOP: for my $i (1..2) { last LOOP if $i == 2 }
print "sigil-08 ternary: $t label: ok\n";

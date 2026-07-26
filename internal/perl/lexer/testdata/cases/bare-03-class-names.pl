#!/usr/bin/perl
# CASE bare-03: a bareword before `->` is a CLASS NAME (a string), including a
# `::`-qualified one and one with a TRAILING `::`. `Foo::Bar` on its own is a
# bareword; under strict subs it must resolve to a sub or be an error.
use strict; use warnings;

package Foo::Bar;
sub new  { my $c = shift; return bless { c => $c }, $c }
sub who  { return $_[0]{c} }
sub cls  { return $_[0] }
package main;

my $o = Foo::Bar->new;
print "bare-03 instance: ", $o->who, "\n";
print "bare-03 class-method: ", Foo::Bar->cls, "\n";
print "bare-03 trailing-colons: ", Foo::Bar::->cls, "\n";
print "bare-03 string-class: ", "Foo::Bar"->cls, "\n";

my $class = "Foo::Bar";
print "bare-03 dynamic-class: ", $class->cls, "\n";

# `Foo::Bar::new(...)` is a plain function call, not a method call.
print "bare-03 function-call: ", ref(Foo::Bar::new("Foo::Bar")), "\n";

# A bareword that is NOT a known sub, under strict subs.
my $out = `$^X -e 'use strict; my \$x = SomeBareword; print \$x;' 2>&1`;
$out =~ s/\s+/ /g; $out =~ s/\s+\z//;
print "bare-03 strict-subs: ", ($out =~ /Bareword .* not allowed/ ? "COMPILE ERROR" : "[$out]"), "\n";

# Without strict, a bareword is its own name as a string.
my $out2 = `$^X -e 'my \$x = SomeBareword; print \$x;' 2>&1`;
$out2 =~ s/\s+\z//;
print "bare-03 no-strict: [$out2]\n";

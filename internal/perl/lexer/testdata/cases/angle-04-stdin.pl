#!/usr/bin/perl
# CASE angle-04: `<STDIN>` -- a bareword handle inside angles. The contents are a
# package-qualifiable identifier, so `<Foo::BAR>` is also readline.
use strict; use warnings;

close STDIN;
open STDIN, '<', \"first line\nsecond line\n" or die;

my $l = <STDIN>;
chomp $l;
print "angle-04 stdin: $l\n";

my @rest = <STDIN>;
print "angle-04 remaining: ", scalar(@rest), "\n";

# Package-qualified handle name inside the angles.
package My::IO;
our $buf = "pkg line\n";
package main;
open My::IO::H, '<', \$My::IO::buf or die;
my $p = <My::IO::H>;
chomp $p;
close My::IO::H;
print "angle-04 qualified: $p\n";

#!/usr/bin/perl
# CASE angle-02: `<$fh>` is a readline token. `<` followed by a simple scalar
# followed by `>` with NOTHING else between is readline; anything more complex
# is not (see the glob case).
use strict; use warnings;

my $data = "alpha\nbeta\ngamma\n";
open my $fh, '<', \$data or die;

my $line = <$fh>;
chomp $line;
print "angle-02 scalar-context: $line\n";

my @rest = <$fh>;
chomp @rest;
print "angle-02 list-context: ", join(",", @rest), "\n";

# while (<$fh>) implicitly assigns to $_ and adds defined().
open my $fh2, '<', \$data or die;
my $n = 0;
while (<$fh2>) { $n++ }
print "angle-02 while-lines: $n\n";

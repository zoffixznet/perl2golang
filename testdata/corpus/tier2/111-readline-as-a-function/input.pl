#!/usr/bin/perl
use strict;
use warnings;

# <$fh> only reads a line when what is between the angles is a simple scalar.
# For a handle that lives in a container, `<$h{in}>` is a filename glob and
# not a read at all, so the way to read one is `readline`, spelled out. Code
# that keeps its handles in a structure uses it constantly.

my $path = 'readline-source.txt';
open my $out, '>', $path or die "open $path: $!\n";
print {$out} "alpha\nbeta\ngamma\n";
close $out;

my %handle;
open( $handle{in}, '<', $path ) or die "open $path: $!\n";

my $first = readline( $handle{in} );
chomp $first;
print "first line: $first\n";

my $count = 1;
while ( defined( my $line = readline( $handle{in} ) ) ) {
    chomp $line;
    $count++;
    print "line $count: $line\n";
}
close $handle{in};

# The list form of the same call takes everything left.
open my $rest, '<', $path or die "open $path: $!\n";
my @all = readline($rest);
close $rest;
printf "read %d line(s) in one go\n", scalar @all;

unlink $path;

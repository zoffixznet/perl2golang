#!/usr/bin/perl
use strict;
use warnings;

# A trailing comma is legal in more places than a list: it can sit at the end
# of an if condition too, where the comma operator's answer is the value the
# test sees. Generated code does this a lot, and installed modules carry it.

my $mode = 'UTF-8';
if ( $mode eq 'UTF-8', ) {
    print "matched with a trailing comma\n";
}

my @seen;
for my $name ( 'alpha', 'beta', ) {
    push @seen, $name;
}
print "walked: ", join( ',', @seen ), "\n";

print "done\n";

__END__
Anything under the marker is data, not code. A program that carries both the
trailing comma above and this section must still convert to the end.

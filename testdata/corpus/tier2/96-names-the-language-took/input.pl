#!/usr/bin/perl
# Report renderer whose helper names happen to be names Go has spoken for:
# fmt and json read naturally in Perl and collide with imports in Go, and
# toText collides with the support code the conversion itself brings along.
# A buried sub is package-global however deep its declaration sits.
use strict;
use warnings;

sub fmt { my ($n) = @_; return $n == int $n ? sprintf( '%d', $n ) : sprintf( '%.2f', $n ) }
sub json { my ($k, $v) = @_; return "\"$k\": " . ( $v =~ /^[\d.]+$/ ? $v : "\"$v\"" ) }
sub toText { my ($x) = @_; return defined $x ? "$x" : '(none)' }

my %row = ( name => 'widget', price => 12.5, qty => 4, note => undef );

{
    # The formatter for one locale lives in its own block.
    sub money { my ($n) = @_; return '$' . fmt($n) }
}

print "line: ", money( $row{price} * $row{qty} ), "\n";
for my $k ( sort keys %row ) {
    print "  ", json( $k, toText( $row{$k} ) ), "\n";
}
printf "as int: %s, as float: %s\n", fmt(7), fmt(7.25);

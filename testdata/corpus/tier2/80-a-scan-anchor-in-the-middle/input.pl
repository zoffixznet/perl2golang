#!/usr/bin/perl
use strict;
use warnings;

# \G in the two places it cannot be turned into a start-of-text anchor: inside
# an alternation, and in a pattern that is not walking the string with /g.

print "--- \\G inside an alternation ---\n";
my $list = 'red, green , blue';
my @items;
pos($list) = 0;
while ( $list =~ /(?:\G|,)\s*(\w+)/gc ) {
    push @items, $1;
}
printf "items: %s\n", join '|', @items;

print "--- \\G with no walk to anchor to ---\n";
my $stamp = '2024-07-19';
if ( $stamp =~ /\G(\d{4})/ ) {
    print "year at the start: $1\n";
}
else {
    print "no year at the start\n";
}

print "--- the position after a failed match with and without /c ---\n";
my $text = 'aa bb';
pos($text) = 0;
$text =~ /\G(\w+)/gc;
printf "after a match: %d\n", pos($text);
$text =~ /\Gzzz/gc;
printf "after a failure with /c: %s\n",
    ( defined pos($text) ? pos($text) : 'undef' );
$text =~ /\Gzzz/g;
printf "after a failure without /c: %s\n",
    ( defined pos($text) ? pos($text) : 'undef' );

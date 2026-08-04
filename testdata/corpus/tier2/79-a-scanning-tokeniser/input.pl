#!/usr/bin/perl
use strict;
use warnings;

# A hand-written lexer: the position lives on the string, every alternative is
# anchored at it with \G, and /c leaves the position alone when a pattern
# fails so the next alternative gets a turn from the same place.

my $expr = '12 + 34 * (5 - 6) / 7';
my @tokens;
pos($expr) = 0;
while ( pos($expr) < length $expr ) {
    if    ( $expr =~ /\G\s+/gc )       { next }
    elsif ( $expr =~ /\G(\d+)/gc )     { push @tokens, "NUM($1)" }
    elsif ( $expr =~ /\G([-+*\/])/gc ) { push @tokens, "OP($1)" }
    elsif ( $expr =~ /\G([()])/gc )    { push @tokens, "PAREN($1)" }
    else                               { push @tokens, 'ERR'; last }
}
printf "%d tokens: %s\n", scalar @tokens, join ' ', @tokens;

print "--- the same walk over a key=value string ---\n";
my $config = 'host=web1 port=8080 tls=on';
my %setting;
pos($config) = 0;
while ( $config =~ /\G(\w+)=(\S+)\s*/gc ) {
    $setting{$1} = $2;
}
printf "read %d settings, stopped at %d of %d\n",
    scalar keys %setting, pos($config), length $config;
for my $k ( sort keys %setting ) {
    printf "  %-5s %s\n", $k, $setting{$k};
}

print "--- a branch whose test costs something ---\n";
my @queue = ( 'a', 'b', 'c' );
my @taken;
for ( 1 .. 5 ) {
    if    ( @taken >= 2 )                     { push @taken, 'full' }
    elsif ( defined( my $next = shift @queue ) ) { push @taken, $next }
    else                                      { push @taken, '-' }
}
printf "taken: %s, %d left in the queue\n", join( ',', @taken ), scalar @queue;

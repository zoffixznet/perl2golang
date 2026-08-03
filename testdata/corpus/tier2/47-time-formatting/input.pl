#!/usr/bin/perl
# Taking a fixed timestamp apart and putting it back together in the shapes a
# report wants. Everything is UTC and the epoch is written in, so the output
# is the same wherever and whenever this runs.
use strict;
use warnings;
use POSIX qw(strftime);
use Scalar::Util qw(blessed reftype looks_like_number);

my $epoch = 1_700_000_000;    # 2023-11-14T22:13:20Z

my @g = gmtime $epoch;
printf "parts: sec=%d min=%d hour=%d mday=%d mon=%d year=%d wday=%d yday=%d\n",
    @g[ 0 .. 7 ];
printf "rebuilt: %04d-%02d-%02dT%02d:%02d:%02dZ\n",
    $g[5] + 1900, $g[4] + 1, @g[ 3, 2, 1, 0 ];

print "-- formats that map to a layout --\n";
printf "iso:      %s\n", strftime( '%Y-%m-%dT%H:%M:%SZ', @g );
printf "date:     %s\n", strftime( '%Y-%m-%d',           @g );
printf "clock:    %s\n", strftime( '%H:%M:%S',           @g );
printf "compact:  %s\n", strftime( '%Y%m%d-%H%M%S',      @g );
printf "named:    %s\n", strftime( '%a %b %d %Y',        @g );
printf "long:     %s\n", strftime( '%A, %d %B %Y',       @g );
printf "12 hour:  %s\n", strftime( '%I:%M %p',           @g );

print "-- formats with no layout at all --\n";
printf "day of year: %s\n", strftime( '%j', @g );
printf "weekday:     %s\n", strftime( '%w', @g );
printf "mixed:       %s\n", strftime( 'day %j of %Y', @g );

print "-- moving in whole seconds --\n";
for my $delta ( 0, 3600, 86_400 ) {
    printf "  +%-6d %s\n", $delta, strftime( '%Y-%m-%d %H:%M:%S', gmtime( $epoch + $delta ) );
}

print "-- what is this value --\n";
for my $text ( 'hello', '42', '3.5e2', '12abc', ' 7 ' ) {
    printf "  %-7s reftype=%-6s number=%s\n",
        "'$text'", ( reftype($text) || '-' ),
        ( looks_like_number($text) ? 'yes' : 'no' );
}

my $ref  = { a => 1, b => 'two' };
my $list = [ 1, 2 ];
my $code = sub { 1 };
printf "  %-7s reftype=%s\n", 'hash',  reftype($ref);
printf "  %-7s reftype=%s\n", 'array', reftype($list);
printf "  %-7s reftype=%s\n", 'code',  reftype($code);
printf "  blessed on a plain reference: %s\n", ( blessed($ref) ? 'yes' : 'no' );

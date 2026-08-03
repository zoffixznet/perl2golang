#!/usr/bin/perl
# Time as a quantity rather than as a moment: differences, durations, whole
# calendar steps, and parsing a stamp back into an epoch. This is where Perl's
# "a time is just a number of seconds" habit and Go's separate Time and
# Duration types disagree most, and none of it converts yet.
use strict;
use warnings;
use POSIX qw(strftime mktime);
use Time::Local qw(timegm);

my $start = 1_700_000_000;    # 2023-11-14T22:13:20Z
my $end   = 1_700_090_000;

# A difference of two times is a duration, and Perl leaves it a number.
my $delta = $end - $start;
printf "delta: %d seconds\n", $delta;
printf "as h:m:s %02d:%02d:%02d\n",
    int( $delta / 3600 ), int( ( $delta % 3600 ) / 60 ), $delta % 60;

# Rounding a moment down to a boundary, done with modulo on the epoch.
my $hour = $start - $start % 3600;
my $day  = $start - $start % 86_400;
printf "hour bucket: %s\n", strftime( '%Y-%m-%dT%H:%M:%SZ', gmtime $hour );
printf "day bucket:  %s\n", strftime( '%Y-%m-%dT%H:%M:%SZ', gmtime $day );

# Going the other way: a written-out date back into an epoch.
my $parsed = timegm( 0, 0, 12, 25, 11, 2023 );
printf "christmas noon: %d -> %s\n", $parsed,
    strftime( '%Y-%m-%dT%H:%M:%SZ', gmtime $parsed );

# Whole calendar steps, which are not a fixed number of seconds at all.
my @g = gmtime $start;
$g[4] += 1;    # next month, by bumping the month field
my $next_month = timegm(@g);
printf "one month on:  %s\n", strftime( '%Y-%m-%d', gmtime $next_month );

my $later = $start + 45 * 86_400;    # 45 days on, counted in seconds
printf "45 days after: %s\n", strftime( '%Y-%m-%d', gmtime $later );

# Comparing moments, which Perl does with plain numeric operators.
my @stamps = ( $end, $start, $parsed );
my @sorted = sort { $a <=> $b } @stamps;
printf "earliest: %s\n", strftime( '%Y-%m-%d', gmtime $sorted[0] );
printf "latest:   %s\n", strftime( '%Y-%m-%d', gmtime $sorted[-1] );
printf "start is before end: %s\n", ( $start < $end ? 'yes' : 'no' );

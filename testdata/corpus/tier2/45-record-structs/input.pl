#!/usr/bin/perl
# Hash references used as records, in the four places a script puts them: in a
# list, in a lookup by key, nested inside each other, and read back by a field
# name the program works out while it runs.
use strict;
use warnings;

sub new_reading {
    my ( $station, $celsius, $ok ) = @_;
    return {
        station => $station,
        celsius => $celsius,
        ok      => $ok,
        notes   => [],
    };
}

my @readings = (
    new_reading( 'north', 12,  1 ),
    new_reading( 'south', 31,  1 ),
    new_reading( 'inner', -4,  0 ),
);

# a field added after construction, and a list field appended to
for my $r (@readings) {
    push @{ $r->{notes} }, 'hot'    if $r->{celsius} > 25;
    push @{ $r->{notes} }, 'frozen' if $r->{celsius} < 0;
    $r->{fahrenheit} = sprintf '%.1f', $r->{celsius} * 9 / 5 + 32;
}

printf "%-6s %5d %8s ok=%d notes=[%s]\n",
    $_->{station}, $_->{celsius}, $_->{fahrenheit}, $_->{ok},
    join( ',', @{ $_->{notes} } )
    for @readings;

# sorted by a field, which needs the field to have a type
print "-- coldest first --\n";
for my $r ( sort { $a->{celsius} <=> $b->{celsius} } @readings ) {
    printf "  %-6s %d\n", $r->{station}, $r->{celsius};
}

# a record reached through a hash, filled in only when it is missing
my %latest;
for my $r (@readings) {
    my $slot = $latest{ $r->{station} } ||= {
        station => $r->{station},
        celsius => 0,
        ok      => 1,
        notes   => [],
    };
    $slot->{celsius} = $r->{celsius};
    $slot->{ok}      = $r->{ok};
}
print "-- latest --\n";
for my $name ( sort keys %latest ) {
    printf "  %-6s %4d ok=%d\n", $name, $latest{$name}{celsius}, $latest{$name}{ok};
}

# a record inside a record
my $site = {
    name    => 'ridge',
    tallest => new_reading( 'peak', 3, 1 ),
    count   => scalar @readings,
};
printf "site %s has %d readings, tallest at %s (%d)\n",
    $site->{name}, $site->{count}, $site->{tallest}{station},
    $site->{tallest}{celsius};

# several fields read at once
my ( $st, $c, $ok ) = @{ $readings[0] }{qw(station celsius ok)};
printf "first: %s %d %d\n", $st, $c, $ok;

# a field named by a value rather than written out
for my $field (qw(station celsius ok)) {
    printf "  %-8s = %s\n", $field, $readings[1]{$field};
}

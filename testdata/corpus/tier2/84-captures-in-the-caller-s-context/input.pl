#!/usr/bin/perl
use strict;
use warnings;

# The neighbour of entry 59: a sub that returns a match, called in three
# contexts that Perl distinguishes and Go cannot, plus one that uses
# wantarray to answer differently in each.

my @rows = ( 'ada 1815', 'grace 1906', 'no year here' );

sub parts { my ($t) = @_; return $t =~ /^(\w+)\s+(\d{4})$/ }

# List context: the groups.
print "--- list context ---\n";
for my $row (@rows) {
    my ( $name, $year ) = parts($row);
    printf "%-6s %s\n", $name // '(none)', $year // '(none)';
}

# Boolean context: did it match.
print "--- boolean context ---\n";
printf "%s: %s\n", $_, ( parts($_) ? 'parsed' : 'not parsed' ) for @rows;

# Scalar context on the sub itself, which in Perl is the last capture.
print "--- scalar context ---\n";
for my $row (@rows) {
    my $one = parts($row);
    printf "%s -> %s\n", $row, ( defined $one ? $one : '(undef)' );
}

# The explicit form: a sub that asks which context it was called in.
sub either {
    my ($t) = @_;
    my @got = $t =~ /^(\w+)\s+(\d{4})$/;
    return wantarray ? @got : scalar @got;
}
print "--- wantarray ---\n";
for my $row (@rows) {
    my @all   = either($row);
    my $count = either($row);
    printf "%-12s list=%d scalar=%s\n", $row, scalar @all, $count;
}

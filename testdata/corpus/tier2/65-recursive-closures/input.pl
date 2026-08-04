#!/usr/bin/perl
use strict;
use warnings;

# Two shapes of code reference that still come out wrong.

# 1. A comparator chosen at run time, called with records. The closure takes
#    its arguments through @_, so it cannot say what they are, and reading a
#    field out of one asks the value to be a hash when it is a record.
my @rows = (
    { name => 'pear',  n => 3 },
    { name => 'fig',   n => 9 },
    { name => 'apple', n => 3 },
);
for my $key ( 'name', 'count' ) {
    my $cmp = $key eq 'count'
        ? sub { $_[0]{n} <=> $_[1]{n} or $_[0]{name} cmp $_[1]{name} }
        : sub { $_[0]{name} cmp $_[1]{name} };
    my @sorted = sort { $cmp->( $a, $b ) } @rows;
    print "by $key: ", join( ' ', map { "$_->{name}/$_->{n}" } @sorted ), "\n";
}

# 2. A closure that calls itself. The name is declared first and filled in
#    afterwards, because the sub cannot refer to a variable that does not
#    exist yet.
my %seen;
my $fib;
$fib = sub {
    my ($n) = @_;
    return $n if $n < 2;
    $seen{$n}++;
    return $fib->( $n - 1 ) + $fib->( $n - 2 );
};
print "fib(10) = ", $fib->(10), "\n";
print "levels memo-visited: ", scalar keys %seen, "\n";

# 3. A closure stored in a hash that calls its neighbour through the hash.
my %calc;
%calc = (
    double => sub { return $_[0] * 2 },
    quad   => sub { return $calc{double}->( $calc{double}->( $_[0] ) ) },
);
print "quad(5) = ", $calc{quad}->(5), "\n";

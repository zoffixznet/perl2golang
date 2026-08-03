#!/usr/bin/perl
# What a callback's caller cannot say in Go: which context it wanted. A Perl
# sub returns a list or one value depending on where it was called, and the
# same sub called both ways gives two different answers. A Go function has one
# return type, so the callback has to pick.
use strict;
use warnings;

my %readers = (
    words => sub { split /\s+/, $_[0] },
    pairs => sub { my ($s) = @_; map { [ $_, length $_ ] } split /,/, $s },
    first => sub { ( split /\s+/, $_[0] )[0] },
);

my $line = 'alpha beta gamma';

print "-- called for a list --\n";
my @words = $readers{words}->($line);
printf "words: %s\n", join( '|', @words );
printf "count: %d\n", scalar @words;

print "-- the same sub, called for one value --\n";
my $howmany = $readers{words}->($line);
printf "in scalar context: %s\n", $howmany;
printf "explicit scalar:   %s\n", scalar( $readers{words}->($line) );

print "-- a callback returning a list of references --\n";
my @pairs = $readers{pairs}->('a,bb,ccc');
printf "pairs: %s\n", join( ' ', map { "$_->[0]=$_->[1]" } @pairs );

print "-- a callback that already picks one --\n";
printf "first: %s\n", $readers{first}->($line);

print "-- passed on to another sub --\n";
sub apply_all {
    my ( $text, @fns ) = @_;
    return map { scalar $_->($text) } @fns;
}
my @results = apply_all( $line, $readers{words}, $readers{first} );
printf "applied: %s\n", join( ' ', @results );

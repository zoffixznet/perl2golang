#!/usr/bin/perl
use strict;
use warnings;

# Reading a header row and zipping it against each data row is what a hash
# slice is for, and it is the shape half of the CSV scripts in the world are
# built on.

my @header = qw(id name dept city);
my @rows   = (
    'E001,Jane Doe,D10,lisbon',
    'E002,Bo Li,D20,osaka',
    'E003,Ada Byron',
);

my @required = qw(id name);
my @optional = qw(dept city);

print "--- records ---\n";
my @records;
for my $row (@rows) {
    my @fields = split /,/, $row;
    my %rec;
    @rec{@header} = @fields;    # short rows leave the tail undef
    push @records, \%rec;
    printf "%-5s %-9s %s\n", $rec{id}, $rec{name},
      join ' ', map { defined $rec{$_} ? "$_=$rec{$_}" : "$_=?" } @optional;
}

print "--- only the required columns ---\n";
for my $rec (@records) {
    my %slim;
    @slim{@required} = @{$rec}{@required};
    printf "  %s\n", join ',', map { "$_=$slim{$_}" } @required;
}

print "--- dropping columns ---\n";
my %first = %{ $records[0] };
my @dropped = delete @first{@optional};
printf "dropped %d value(s): %s\n", scalar @dropped, join ',', @dropped;
printf "left: %s\n", join ',', sort keys %first;

print "--- a lookup table built from two lists ---\n";
my @codes = qw(D10 D20 D30);
my @names = ( 'Design', 'Delivery' );
my %dept;
@dept{@codes} = @names;    # one code short of a name
for my $c (@codes) {
    printf "  %s => %s\n", $c, ( defined $dept{$c} ? $dept{$c} : '(unnamed)' );
}

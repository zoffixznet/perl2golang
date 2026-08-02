#!/usr/bin/perl
# The options-hash form of an option block, which is how most scripts that
# take more than two options are written: one hash of defaults, one call
# naming every option and its type, and the leftovers in @ARGV.
use strict;
use warnings;
use Getopt::Long;

my %opt = (
    limit   => 10,
    format  => 'text',
    header  => 1,
    factor  => 1.5,
    verbose => 0,
);

GetOptions( \%opt,
    'limit|n=i',
    'format|f=s',
    'header!',
    'factor=f',
    'verbose|v+',
    'tag=s@',
    'rename=s%',
) or die "usage: $0 [options] WORD...\n";

printf "limit=%d format=%s header=%d factor=%.2f verbose=%d\n",
    $opt{limit}, $opt{format}, $opt{header}, $opt{factor}, $opt{verbose};
printf "tags=[%s]\n", join( ',', @{ $opt{tag} || [] } );
printf "rename=[%s]\n",
    join( ',', map { "$_=$opt{rename}{$_}" } sort keys %{ $opt{rename} || {} } );
printf "words=[%s]\n", join( ',', @ARGV );

die "limit must be positive\n" if $opt{limit} <= 0;

my $shown = 0;
for my $word ( sort @ARGV ) {
    last if $shown >= $opt{limit};
    my $out = exists $opt{rename}{$word} ? $opt{rename}{$word} : $word;
    if ( $opt{format} eq 'csv' ) { printf "%d,%s\n", ++$shown, $out }
    else                         { printf "%2d %s\n", ++$shown, $out }
}
printf "shown %d of %d\n", $shown, scalar @ARGV;
print "chatty\n" if $opt{verbose} >= 2;

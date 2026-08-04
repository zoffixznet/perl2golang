#!/usr/bin/perl
use strict;
use warnings;
use Getopt::Long;

# The commonest way an option block is written: one pair per option, the
# destination named on the spot. Plus the two Configure settings a wrapper
# script reaches for.

Getopt::Long::Configure( 'bundling', 'pass_through' );

my %opt = (
    verbose => 0,
    quiet   => 0,
    jobs    => 1,
    format  => 'text',
    tag     => undef,
    define  => {},
    only    => [],
);

GetOptions(
    'v+'        => \$opt{verbose},
    'q'         => \$opt{quiet},
    'j=i'       => \$opt{jobs},
    'format|f=s' => \$opt{format},
    'tag:s'     => \$opt{tag},
    'define|D=s%' => $opt{define},
    'only|o=s@' => $opt{only},
) or die "usage: $0 [options] TARGET...\n";

printf "verbose=%d quiet=%d jobs=%d format=%s\n",
    $opt{verbose}, $opt{quiet}, $opt{jobs}, $opt{format};
printf "tag given: %s\n", defined $opt{tag} ? "yes ('$opt{tag}')" : 'no';
printf "define: %s\n", join( ' ', map { "$_=$opt{define}{$_}" } sort keys %{ $opt{define} } );
printf "only: %s\n", ( @{ $opt{only} } ? join( ',', @{ $opt{only} } ) : '(all)' );

# Everything the parser did not take is still in @ARGV, in order.
printf "left over: %s\n", ( @ARGV ? join( ' ', @ARGV ) : '(none)' );
my $first = shift @ARGV;
printf "first target: %s, %d more\n", $first, scalar @ARGV;

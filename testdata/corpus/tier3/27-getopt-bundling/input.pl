#!/usr/bin/perl
# The option forms that Go's flag package cannot be taught: single-letter
# options run together, an unknown option passed through to a wrapped
# command, and an option whose value may be left off.
use strict;
use warnings;
use Getopt::Long;

Getopt::Long::Configure( 'bundling', 'pass_through' );

my ( $verbose, $quiet, $jobs, $tag ) = ( 0, 0, 1, undef );
GetOptions(
    'v+'   => \$verbose,
    'q'    => \$quiet,
    'j=i'  => \$jobs,
    'tag:s' => \$tag,
) or die "bad options\n";

printf "verbose=%d quiet=%d jobs=%d tag=%s\n",
    $verbose, $quiet, $jobs, defined $tag ? "'$tag'" : 'undef';
printf "passed through: %s\n", ( @ARGV ? "@ARGV" : '(none)' );

#!/usr/bin/perl
use strict;
use warnings;
use Getopt::Long;

# The parts of an option block that have no flag-package shape at all: a
# destination that is code, the unique-prefix abbreviation the parser does by
# default, and an optional value that takes the next word when there is one.

my @trace;
my %opt = ( level => 'info', dry_run => 0 );

GetOptions(
    'level=s'   => \$opt{level},
    'dry-run!'  => \$opt{dry_run},
    'trace=s'   => sub { my ( $name, $value ) = @_; push @trace, "$name=$value" },
    'louder'    => sub { $opt{level} = 'debug' },
    '<>'        => sub { push @trace, "operand=$_[0]" },
) or die "bad options\n";

printf "level=%s dry_run=%d\n", $opt{level}, $opt{dry_run};
printf "trace: %s\n", ( @trace ? join( ' | ', @trace ) : '(none)' );
printf "argv after: %s\n", ( @ARGV ? join( ' ', @ARGV ) : '(none)' );

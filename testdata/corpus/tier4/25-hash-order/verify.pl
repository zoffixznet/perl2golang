#!/usr/bin/perl
# The invariant oracle for this entry. The program's output is legitimately
# different every run, so a byte diff cannot check it; this script checks the
# invariants instead. It reads one run's stdout on STDIN. Checking a
# conversion it also receives the path to the conversion report and to the
# generated main.go; checking perl's own output it receives no arguments.
#
# Complaints never quote the run's own values: the first complaint becomes
# the recorded failure reason, and a reason that embeds a random key order
# would change the scorecard on every run. Rerun the program by hand to see
# the values.
use strict;
use warnings;

my ( $report_path, $main_go ) = @ARGV;

my $bad = 0;
sub complain { warn "$_[0]\n"; $bad = 1 }

# The deterministic checks come first, so that while they fail the recorded
# reason does not depend on how the keys happened to come out.

# Checking a conversion, the report must admit that hash iteration order
# reaches the output; silence here is the naive conversion this entry exists
# to catch.
my $report = '';
if ( defined $report_path ) {
    if ( open my $fh, '<', $report_path ) {
        local $/;
        $report = <$fh> // '';
        close $fh;
    }
    else {
        complain("cannot read the conversion report: $!");
    }
    complain("the report says nothing about iteration order reaching the output")
        unless $report =~ /iteration order/i and $report =~ /output/i;
}

# Sorting the keys would make the output deterministic and differently wrong;
# it is only acceptable when the report says the order was changed.
if ( defined $main_go and open my $gh, '<', $main_go ) {
    local $/;
    my $go = <$gh> // '';
    close $gh;
    if ( $go =~ /\bsort\.|slices\.Sort/ and $report !~ /order/i ) {
        complain("the generated Go sorts the keys and the report does not say the order was changed");
    }
}

my @lines = <STDIN>;
chomp @lines;
complain( "expected 3 lines of output, got " . scalar @lines ) unless @lines == 3;

my $keys  = ( $lines[0] // '' ) =~ /^keys:\s+(.*)$/ ? $1 : do { complain("the first line is not a keys: line");   '' };
my $again = ( $lines[1] // '' ) =~ /^again: (.*)$/  ? $1 : do { complain("the second line is not an again: line"); '' };
my $csv   = ( $lines[2] // '' ) =~ /^csv: (.*)$/    ? $1 : do { complain("the third line is not a csv: line");    '' };

my %val = ( alpha => 1, beta => 2, gamma => 3, delta => 4, epsilon => 5 );
my @keys = split /,/, $keys;
my %seen;
$seen{$_}++ for @keys;
if ( @keys != 5 or grep { !exists $val{$_} or $seen{$_} != 1 } @keys ) {
    complain("the keys: line must hold alpha,beta,gamma,delta,epsilon exactly once each");
}

# Perl's hash order is random per process but stable within one: repeated
# iterations of an unmodified hash agree. A conversion that loses that
# stability prints two different orders here.
complain("within one run the keys: and again: lines must agree")
    unless $again eq $keys;

my $want_csv = join ",", map { "$_=" . ( $val{$_} // '?' ) } @keys;
complain("the csv: line must follow the keys: order with the recorded values")
    unless $csv eq $want_csv;

exit( $bad ? 1 : 0 );

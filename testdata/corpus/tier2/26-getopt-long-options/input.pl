#!/usr/bin/perl
use strict;
use warnings;
use Getopt::Long qw(GetOptionsFromArray);

# A reporting tool with a realistic option set: negatable booleans, a
# repeatable string option, an integer with a default, a key=value hash
# option, an incrementable verbosity counter and a bare flag.

my %opt = (
    header    => 1,       # --header / --no-header
    threshold => 50,
    format    => 'text',
    verbose   => 0,
    include   => [],
    rename    => {},
    quiet     => 0,
);

Getopt::Long::Configure('bundling', 'no_ignore_case');

my $ok = GetOptions(
    'header!'       => \$opt{header},
    'threshold|t=i' => \$opt{threshold},
    'format|f=s'    => \$opt{format},
    'include|i=s@'  => $opt{include},
    'rename=s%'     => $opt{rename},
    'verbose|v+'    => \$opt{verbose},
    'quiet|q'       => \$opt{quiet},
);
die "$0: bad options\n" unless $ok;

printf "header=%d threshold=%d format=%s verbose=%d quiet=%d\n",
    $opt{header}, $opt{threshold}, $opt{format}, $opt{verbose}, $opt{quiet};
printf "include=[%s]\n", join(',', @{ $opt{include} });
printf "rename=%s\n", join(' ', map { "$_=>$opt{rename}{$_}" } sort keys %{ $opt{rename} });
printf "remaining args: %s\n", (@ARGV ? "@ARGV" : '(none)');

my $file = shift @ARGV;
die "$0: no input file given\n" unless defined $file;

my %want = map { $_ => 1 } @{ $opt{include} };

my @rows;
open my $fh, '<', $file or die "$0: $file: $!\n";
while (my $line = <$fh>) {
    chomp $line;
    my ($host, $metric, $value) = split /\t/, $line;
    next if %want && !$want{$metric};
    $host = $opt{rename}{$host} if exists $opt{rename}{$host};
    push @rows, { host => $host, metric => $metric, value => $value };
}
close $fh or die "close: $!\n";

my @hot = grep { $_->{value} >= $opt{threshold} } @rows;

if ($opt{header} && !$opt{quiet}) {
    printf "%-10s %-6s %5s\n", 'HOST', 'METRIC', 'VALUE';
}

if ($opt{format} eq 'csv') {
    printf "%s,%s,%d\n", $_->{host}, $_->{metric}, $_->{value} for @hot;
}
else {
    printf "%-10s %-6s %5d\n", $_->{host}, $_->{metric}, $_->{value} for @hot;
}

printf "%d of %d rows over threshold %d\n", scalar @hot, scalar @rows, $opt{threshold};
print "verbose detail enabled\n" if $opt{verbose} >= 2;

# Parsing a second, independent argument list without touching @ARGV.
my @other = ('--format', 'json', '-vvv', '--no-header', 'extra.tsv');
my %o2 = (format => 'text', verbose => 0, header => 1);
GetOptionsFromArray(\@other,
    'format=s'  => \$o2{format},
    'verbose|v+' => \$o2{verbose},
    'header!'   => \$o2{header},
) or die "$0: second parse failed\n";
printf "second parse: format=%s verbose=%d header=%d leftover=%s\n",
    $o2{format}, $o2{verbose}, $o2{header}, "@other";

#!/usr/bin/perl
use strict;
use warnings;

# Writes a report, appends a footer in a second pass, reads it back, and
# tidies up after itself. Shows three-arg open for '>' and '>>', printing
# to a lexical handle, and checking the result of close().

my $in  = 'files/sales.txt';
my $out = 'sales-report.txt';

my %totals;
my @regions;

open my $src, '<', $in or die "cannot read $in: $!\n";
while (my $line = <$src>) {
    chomp $line;
    next unless $line =~ /\S/;
    my ($region, $amount) = split ' ', $line;
    push @regions, $region unless exists $totals{$region};
    $totals{$region} += $amount;
}
close $src or die "close $in: $!\n";

open my $rpt, '>', $out or die "cannot write $out: $!\n";
printf {$rpt} "%-8s %8s\n", 'REGION', 'TOTAL';
print $rpt '-' x 17, "\n";
my $grand = 0;
for my $r (sort keys %totals) {
    printf {$rpt} "%-8s %8d\n", $r, $totals{$r};
    $grand += $totals{$r};
}
close $rpt or die "cannot close $out: $!\n";

open my $app, '>>', $out or die "cannot append to $out: $!\n";
print $app '-' x 17, "\n";
printf {$app} "%-8s %8d\n", 'TOTAL', $grand;
print $app "regions in file order: ", join(',', @regions), "\n";
close $app or die "cannot close $out after append: $!\n";

open my $back, '<', $out or die "cannot re-read $out: $!\n";
print while <$back>;
close $back or die "close: $!\n";

my $bytes = -s $out;
printf "report is %d bytes across %d lines\n", $bytes, count_lines($out);

sub count_lines {
    my ($file) = @_;
    open my $fh, '<', $file or die "count_lines: $!\n";
    my $n = 0;
    $n++ while <$fh>;
    close $fh or die "count_lines close: $!\n";
    return $n;
}

# Writing to a handle opened on a scalar reference: an in-memory file.
my $buffer = '';
open my $mem, '>', \$buffer or die "in-memory open failed: $!\n";
print $mem "line $_\n" for 1 .. 3;
close $mem or die "close mem: $!\n";
printf "in-memory buffer holds %d bytes / %d lines\n",
    length($buffer), scalar(split /\n/, $buffer);

# Clean up the artefact this script created so the run leaves no trace.
my $removed = unlink $out;
printf "unlinked %d file(s); still present? %s\n", $removed, (-e $out ? 'yes' : 'no');

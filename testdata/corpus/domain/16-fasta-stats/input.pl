#!/usr/bin/perl
# fasta-stats -- assembly QC numbers for a FASTA file.
#
# The seq lab's standard first look at any new assembly: per-contig
# table, GC%, ambiguity counts, N50/L50.  Case-insensitive because half
# our tools emit soft-masked (lowercase) sequence.  Zero-length records
# happen when the assembler emits a header and dies; we keep them in the
# table but out of the length stats (decided after the great N50 argument
# of 2018).
use strict;
use warnings;

my $file = shift @ARGV or die "usage: $0 <fasta>\n";
open my $fh, '<', $file or die "open $file: $!\n";

# ---- parse: id -> record, order preserved separately ----
my (@order, %seq);
my $cur;
while (my $line = <$fh>) {
    chomp $line;
    next if $line =~ /^\s*$/;
    if ($line =~ /^>(\S+)(?:\s+(.*))?$/) {
        my ($id, $desc) = ($1, $2);
        die "duplicate sequence id '$id' at line $.\n" if exists $seq{$id};
        $cur = $seq{$id} = { id => $id, desc => $desc // '', seq => '' };
        push @order, $id;
    } else {
        die "sequence data before first header at line $.\n" unless $cur;
        $line =~ s/\s+//g;
        $cur->{seq} .= uc $line;
    }
}
close $fh;
die "no sequences found\n" unless @order;

# ---- per-sequence stats ----
for my $id (@order) {
    my $r = $seq{$id};
    my $s = $r->{seq};
    $r->{len} = length $s;
    $r->{gc}  = ($s =~ tr/GC//);
    $r->{at}  = ($s =~ tr/AT//);
    $r->{n}   = ($s =~ tr/N//);
    $r->{amb} = $r->{len} - $r->{gc} - $r->{at} - $r->{n};
    # longest N run, the assembler gap signature
    $r->{gap} = 0;
    while ($s =~ /(N+)/g) {
        $r->{gap} = length $1 if length $1 > $r->{gap};
    }
}

# ---- assembly-level stats over non-empty contigs ----
my @lens = sort { $b <=> $a } map { $seq{$_}{len} } grep { $seq{$_}{len} } @order;
my $total = 0;
$total += $_ for @lens;

my ($n50, $l50, $running) = (0, 0, 0);
for my $len (@lens) {
    $running += $len;
    $l50++;
    if ($running * 2 >= $total) { $n50 = $len; last }
}

# ---- report ----
printf "%-14s %6s %6s %6s %5s %5s  %s\n",
    'ID', 'LEN', 'GC%', 'N', 'AMB', 'GAP', 'DESC';
for my $id (@order) {
    my $r = $seq{$id};
    my $gc_pct = $r->{len} - $r->{n} > 0
        ? sprintf('%.1f', 100 * $r->{gc} / ($r->{len} - $r->{n}))
        : '-';
    printf "%-14s %6d %6s %6d %5d %5d  %s\n",
        $r->{id}, $r->{len}, $gc_pct, $r->{n}, $r->{amb}, $r->{gap},
        elide($r->{desc});
}

print "\n";
printf "sequences:      %d (%d empty)\n",
    scalar @order, scalar(@order) - scalar @lens;
printf "total length:   %d\n", $total;
printf "min/max:        %d / %d\n", $lens[-1], $lens[0];
printf "mean length:    %.1f\n", $total / @lens;
printf "N50:            %d (L50=%d)\n", $n50, $l50;
my $all_gc = 0; my $all_acgt = 0;
for my $id (@order) {
    $all_gc   += $seq{$id}{gc};
    $all_acgt += $seq{$id}{gc} + $seq{$id}{at};
}
printf "overall GC%%:    %.2f%%\n", $all_acgt ? 100 * $all_gc / $all_acgt : 0;
exit 0;

sub elide {
    my ($s) = @_;
    return length $s > 26 ? substr($s, 0, 23) . '...' : $s;
}

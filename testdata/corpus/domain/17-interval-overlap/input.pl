#!/usr/bin/perl
# peak2gene -- annotate BED peaks with overlapping GFF genes.
#
# The eternal coordinate-convention trap: BED is 0-based half-open,
# GFF is 1-based closed.  Everything internal is converted to 1-based
# closed on load, ONCE, and commented, because we have been burned by
# off-by-ones at both ends of this pipeline.
use strict;
use warnings;

my ($gff_file, $bed_file) = @ARGV;
die "usage: $0 <genes.gff> <peaks.bed>\n" unless defined $bed_file;

# ---- load genes: chrom -> strand -> list of [start, end, name] ----
my %genes;
my $gene_count = 0;
open my $gf, '<', $gff_file or die "open $gff_file: $!\n";
while (<$gf>) {
    next if /^#/;
    chomp;
    my ($chrom, undef, $type, $start, $end, undef, $strand, undef, $attrs)
        = split /\t/;
    next unless defined $attrs and $type eq 'gene';
    my ($name) = $attrs =~ /Name=([^;]+)/;
    $name //= sprintf 'unnamed_%d', $gene_count + 1;
    push @{ $genes{$chrom}{$strand} }, [$start, $end, $name];
    $gene_count++;
}
close $gf;

# sort each strand's list by start so the scan below can early-exit
for my $chrom (keys %genes) {
    for my $strand (keys %{ $genes{$chrom} }) {
        @{ $genes{$chrom}{$strand} }
            = sort { $a->[0] <=> $b->[0] } @{ $genes{$chrom}{$strand} };
    }
}

# ---- scan peaks ----
my (@no_hit, %gene_hits);
my ($peaks, $hits) = (0, 0);

open my $bf, '<', $bed_file or die "open $bed_file: $!\n";
printf "%-8s %-18s %6s  %s\n", 'PEAK', 'SPAN', 'SCORE', 'OVERLAPPING GENES';
while (<$bf>) {
    chomp;
    my ($chrom, $bstart, $bend, $pname, $score) = split /\t/;
    next unless defined $pname;
    $peaks++;
    # BED half-open [start,end) -> 1-based closed
    my ($pstart, $pend) = ($bstart + 1, $bend);

    my @over;
    for my $strand (sort keys %{ $genes{$chrom} || {} }) {
        for my $g (@{ $genes{$chrom}{$strand} }) {
            last if $g->[0] > $pend;           # sorted by start: done
            next if $g->[1] < $pstart;         # gene ends before peak
            my $ov_start = $pstart > $g->[0] ? $pstart : $g->[0];
            my $ov_end   = $pend   < $g->[1] ? $pend   : $g->[1];
            my $ov = $ov_end - $ov_start + 1;
            push @over, { name => $g->[2], strand => $strand, bp => $ov };
            $gene_hits{ $g->[2] } += $ov;
        }
    }

    my $span = sprintf '%s:%d-%d', $chrom, $pstart, $pend;
    if (@over) {
        $hits++;
        my $desc = join ', ',
            map  { sprintf '%s(%s,%dbp)', $_->{name}, $_->{strand}, $_->{bp} }
            sort { $b->{bp} <=> $a->{bp} or $a->{name} cmp $b->{name} } @over;
        printf "%-8s %-18s %6d  %s\n", $pname, $span, $score, $desc;
    } else {
        printf "%-8s %-18s %6d  -\n", $pname, $span, $score;
        push @no_hit, $pname;
    }
}
close $bf;

print "\n";
printf "peaks: %d, with overlap: %d, without: %d\n",
    $peaks, $hits, scalar @no_hit;
print "orphan peaks: @no_hit\n" if @no_hit;

print "coverage per gene (bp under peaks):\n";
for my $g (sort { $gene_hits{$b} <=> $gene_hits{$a} or $a cmp $b } keys %gene_hits) {
    printf "  %-6s %5d\n", $g, $gene_hits{$g};
}
exit 0;

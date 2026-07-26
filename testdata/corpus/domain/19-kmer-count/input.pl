#!/usr/bin/perl
# kmer-count -- canonical k-mer spectrum of a read set.
#
# "Canonical" = the lexically smaller of a k-mer and its reverse
# complement, so both strands of the same sequence count together.
# K-mers containing anything but ACGT are skipped (Ns from the base
# caller).  This is the slow honest version; the C++ one replaced it in
# prod years ago but THIS one is still what the course notes teach.
use strict;
use warnings;

my ($file, $k, $top_n) = @ARGV;
$k     ||= 8;
$top_n ||= 10;
die "usage: $0 <reads.fasta> [k] [top-n]\n" unless defined $file;
die "k must be a small positive integer\n" unless $k =~ /^\d+$/ and $k >= 2 and $k <= 32;

open my $fh, '<', $file or die "open $file: $!\n";

my %count;
my ($reads, $bases, $skipped) = (0, 0, 0);

my $seq = '';
while (my $line = <$fh>) {
    chomp $line;
    if ($line =~ /^>/) {
        process($seq) if length $seq;
        $seq = '';
        $reads++;
    } else {
        $seq .= uc $line;
    }
}
process($seq) if length $seq;    # the last record, the classic forgotten one
close $fh;

# ---- spectrum stats ----
my $distinct = keys %count;
my $total    = 0;
my $singletons = 0;
for my $c (values %count) {
    $total += $c;
    $singletons++ if $c == 1;
}

printf "reads=%d bases=%d k=%d\n", $reads, $bases, $k;
printf "kmers: %d total, %d distinct, %d singletons, %d skipped (non-ACGT)\n",
    $total, $distinct, $singletons, $skipped;

# histogram of multiplicities (how many kmers occur exactly c times)
my %mult;
$mult{$_}++ for values %count;
print "multiplicity histogram:\n";
for my $c (sort { $a <=> $b } keys %mult) {
    printf "  %3dx %d kmer(s)\n", $c, $mult{$c};
}

print "top $top_n canonical kmers:\n";
my @ranked = sort { $count{$b} <=> $count{$a} or $a cmp $b } keys %count;
splice @ranked, $top_n if @ranked > $top_n;
for my $kmer (@ranked) {
    printf "  %s %4d\n", $kmer, $count{$kmer};
}
exit 0;

# ----------------------------------------------------------------------
sub process {
    my ($s) = @_;
    $bases += length $s;
    my $last = length($s) - $k;
    for my $i (0 .. $last) {
        my $kmer = substr $s, $i, $k;
        if ($kmer =~ tr/ACGT//c) {    # any char outside ACGT?
            $skipped++;
            next;
        }
        $count{ canonical($kmer) }++;
    }
}

sub canonical {
    my ($kmer) = @_;
    (my $rc = reverse $kmer) =~ tr/ACGT/TGCA/;
    return $kmer lt $rc ? $kmer : $rc;
}

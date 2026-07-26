#!/usr/bin/perl
# translate-cds -- translate annotated CDS records to protein FASTA.
#
# Headers carry table=/strand=/frame= attributes (the annotation
# pipeline writes them); minus-strand records get reverse-complemented
# before translation.  Records with a bad table die individually and are
# reported at the end -- one bad annotation must not sink the batch.
use strict;
use warnings;
use FindBin;
use lib $FindBin::Bin;
use CodonTable qw(translate revcomp table_names);

my $file = shift @ARGV or die "usage: $0 <cds.fasta>\n";
open my $fh, '<', $file or die "open $file: $!\n";

# ---- parse fasta with attribute headers ----
my @records;
my $cur;
while (<$fh>) {
    chomp;
    next unless /\S/;
    if (/^>(\S+)\s*(.*)$/) {
        my ($id, $attr_str) = ($1, $2);
        my %attrs = $attr_str =~ /(\w+)=(\S+)/g;
        $cur = { id => $id, attrs => \%attrs, seq => '' };
        push @records, $cur;
    } else {
        s/\s+//g;
        $cur->{seq} .= $_ if $cur;
    }
}
close $fh;

# ---- translate each record ----
my (@errors, %aa_freq);
my $ok = 0;

for my $r (@records) {
    my $table  = $r->{attrs}{table}  // 'standard';
    my $strand = $r->{attrs}{strand} // '+';
    my $frame  = $r->{attrs}{frame}  // 0;

    my $dna = $strand eq '-' ? revcomp($r->{seq}) : $r->{seq};

    my ($protein, $info) = eval { translate($table, $dna, $frame) };
    if (!defined $protein) {
        my $err = $@;
        chomp $err;
        push @errors, "$r->{id}: $err";
        next;
    }
    $ok++;

    my @flags;
    push @flags, 'no-start'                          unless $info->{starts_with_start};
    push @flags, "internal-stops=$info->{internal_stops}" if $info->{internal_stops};
    push @flags, "partial-tail=$info->{partial_tail}bp"   if $info->{partial_tail};
    push @flags, 'no-terminal-stop'                  unless $protein =~ /\*$/;

    print ">$r->{id} len=", length $protein,
        (@flags ? ' ' . join(',', @flags) : ''), "\n";
    print wrap60($protein);

    $aa_freq{$_}++ for split //, $protein;
}

# ---- batch summary ----
print "# translated $ok/", scalar @records, " records\n";
if (@errors) {
    print "# FAILED: $_\n" for @errors;
}
my @top = sort { $aa_freq{$b} <=> $aa_freq{$a} or $a cmp $b } keys %aa_freq;
splice @top, 5 if @top > 5;
print "# top residues: ",
    join(' ', map { "$_=$aa_freq{$_}" } @top), "\n";
print "# tables available: ", join(', ', table_names()), "\n";
exit(@errors ? 1 : 0);

sub wrap60 {
    my ($s) = @_;
    my $out = '';
    while (length $s > 60) {
        $out .= substr($s, 0, 60, '') . "\n";
    }
    $out .= "$s\n" if length $s;
    return $out;
}

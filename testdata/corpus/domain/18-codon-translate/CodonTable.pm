package CodonTable;
# Codon translation tables.  Table 1 (standard) and table 2 (vertebrate
# mitochondrial) because those are the only two this lab has ever needed.
# Data layout is deliberately nested: table -> section -> codon, so that
# adding the plastid table someday is a data change, not a code change.
use strict;
use warnings;
use Exporter 'import';
our @EXPORT_OK = qw(translate revcomp table_names codon_lookup);

my %TABLES = (
    standard => {
        name   => 'Standard',
        starts => { ATG => 1, CTG => 1, TTG => 1 },
        stops  => { TAA => 1, TAG => 1, TGA => 1 },
        codons => {},   # filled from the compact map below
    },
    mito => {
        name   => 'Vertebrate Mitochondrial',
        starts => { ATG => 1, ATA => 1, ATT => 1 },
        stops  => { TAA => 1, TAG => 1, AGA => 1, AGG => 1 },
        codons => {},
    },
);

# Compact amino-acid map in TCAG order, the classic textbook layout.
my $AA_STANDARD = 'FFLLSSSSYY**CC*WLLLLPPPPHHQQRRRRIIIMTTTTNNKKSSRRVVVVAAAADDEEGGGG';
my %MITO_DIFF = (ATA => 'M', TGA => 'W', AGA => '*', AGG => '*');

{
    my @bases = qw(T C A G);
    my $i = 0;
    for my $b1 (@bases) {
        for my $b2 (@bases) {
            for my $b3 (@bases) {
                my $codon = "$b1$b2$b3";
                my $aa = substr $AA_STANDARD, $i++, 1;
                $TABLES{standard}{codons}{$codon} = $aa;
                $TABLES{mito}{codons}{$codon} = $MITO_DIFF{$codon} // $aa;
            }
        }
    }
}

sub table_names { return sort keys %TABLES }

# codon_lookup(table, codon) -> aa | 'X' for ambiguous | dies on garbage
sub codon_lookup {
    my ($table, $codon) = @_;
    my $t = $TABLES{$table} or die "unknown codon table '$table'\n";
    $codon = uc $codon;
    $codon =~ tr/U/T/;    # tolerate RNA input
    return $t->{codons}{$codon} if exists $t->{codons}{$codon};
    return 'X' if $codon =~ /^[ACGTN]{3}$/;         # Ns translate to X
    die "invalid codon '$codon'\n";
}

# translate(table, dna, frame) -> (protein, \%info)
# frame is 0/1/2; a trailing partial codon is dropped and noted.
sub translate {
    my ($table, $dna, $frame) = @_;
    $frame ||= 0;
    die "frame must be 0, 1 or 2\n" unless $frame =~ /^[012]$/;
    my $t = $TABLES{$table} or die "unknown codon table '$table'\n";

    my %info = (internal_stops => 0, starts_with_start => 0, partial_tail => 0);
    my $protein = '';
    my $len = length $dna;
    my $pos = $frame;
    while ($pos + 3 <= $len) {
        my $codon = uc substr $dna, $pos, 3;
        $codon =~ tr/U/T/;
        my $aa = codon_lookup($table, $codon);
        if ($pos == $frame) {
            $info{starts_with_start} = $t->{starts}{$codon} ? 1 : 0;
        }
        $protein .= $aa;
        $pos += 3;
    }
    $info{partial_tail} = $len - $pos;
    # stops anywhere except the final position are suspicious in a CDS
    my $body = $protein;
    $body =~ s/\*$//;
    $info{internal_stops} = () = $body =~ /\*/g;
    return ($protein, \%info);
}

sub revcomp {
    my ($dna) = @_;
    (my $rc = reverse $dna) =~ tr/ACGTacgtN/TGCAtgcaN/;
    return $rc;
}

1;

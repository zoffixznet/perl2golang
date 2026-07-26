#!/usr/bin/perl
# "Did you mean?" engine for a CLI: Text::Abbrev prefix matching first,
# then Levenshtein distance and an LCS-based similarity ratio for ranking
# suggestions. Reads attempted commands from stdin.
use strict;
use warnings;
use Text::Abbrev;

my @commands = qw(
  status stash stage checkout cherry-pick commit config
  push pull fetch merge rebase reset restore revert log
);

# Text::Abbrev builds unambiguous-prefix => command mappings.
my %abbrev = abbrev @commands;

# ---- levenshtein: classic DP over a 2-row rolling matrix ---------------
sub levenshtein {
    my ( $s, $t ) = @_;
    my @prev = 0 .. length $t;
    for my $i ( 1 .. length $s ) {
        my @cur = ($i);
        my $sc = substr $s, $i - 1, 1;
        for my $j ( 1 .. length $t ) {
            my $cost = $sc eq substr( $t, $j - 1, 1 ) ? 0 : 1;
            my $min  = $prev[$j] + 1;                       # deletion
            my $ins  = $cur[ $j - 1 ] + 1;                  # insertion
            my $sub  = $prev[ $j - 1 ] + $cost;             # substitution
            $min = $ins if $ins < $min;
            $min = $sub if $sub < $min;
            push @cur, $min;
        }
        @prev = @cur;
    }
    return $prev[-1];
}

# ---- LCS length via DP, then a similarity ratio ------------------------
sub lcs_len {
    my ( $s, $t ) = @_;
    my @row = (0) x ( length($t) + 1 );
    for my $i ( 1 .. length $s ) {
        my $diag = 0;
        for my $j ( 1 .. length $t ) {
            my $tmp = $row[$j];
            $row[$j] =
              substr( $s, $i - 1, 1 ) eq substr( $t, $j - 1, 1 )
              ? $diag + 1
              : ( $row[$j] >= $row[ $j - 1 ] ? $row[$j] : $row[ $j - 1 ] );
            $diag = $tmp;
        }
    }
    return $row[-1];
}

sub similarity {
    my ( $s, $t ) = @_;
    my $longer = length($s) > length($t) ? length($s) : length($t);
    return $longer ? lcs_len( $s, $t ) / $longer : 1;
}

sub suggest {
    my ($input) = @_;
    my @scored = map {
        {
            cmd  => $_,
            dist => levenshtein( $input, $_ ),
            sim  => similarity( $input, $_ ),
        }
    } @commands;
    @scored = sort {
        $a->{dist} <=> $b->{dist}
          or $b->{sim} <=> $a->{sim}
          or $a->{cmd} cmp $b->{cmd}
    } @scored;
    return grep { $_->{dist} <= 3 } @scored[ 0 .. 2 ];
}

# ---- drive it from stdin -----------------------------------------------
while ( my $try = <STDIN> ) {
    chomp $try;
    next unless length $try;

    if ( grep { $_ eq $try } @commands ) {
        print "$try: exact match\n";
    }
    elsif ( my $full = $abbrev{$try} ) {
        print "$try: unambiguous prefix of '$full'\n";
    }
    else {
        my @sugg = suggest($try);
        if (@sugg) {
            print "$try: unknown; did you mean "
              . join( ', ',
                map { sprintf "%s (d=%d s=%.2f)", @{$_}{qw(cmd dist sim)} }
                  @sugg )
              . "?\n";
        }
        else {
            print "$try: unknown, no suggestion within distance 3\n";
        }
    }
}

# ---- fixed self-checks -------------------------------------------------
print "--- invariants ---\n";
printf "lev(kitten,sitting)=%d\n", levenshtein( 'kitten', 'sitting' );
printf "lev(x,'')=%d lev('','')=%d\n", levenshtein( 'x', '' ),
  levenshtein( '', '' );
printf "lcs(GAC,AGCAT)=%d\n", lcs_len( 'GAC', 'AGCAT' );
printf "sym: d(a,b)==d(b,a): %s\n",
  levenshtein(qw(rebase restore)) == levenshtein(qw(restore rebase))
  ? 'yes'
  : 'no';
my $ambiguous = grep { $_ eq 'st' } keys %abbrev;
printf "'st' is ambiguous: %s\n", $ambiguous ? 'no' : 'yes';

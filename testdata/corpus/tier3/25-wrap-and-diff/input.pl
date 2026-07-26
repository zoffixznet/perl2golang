#!/usr/bin/perl
# Text toolbox: greedy word-wrap with long-token splitting and hanging
# indent, plus an LCS-based line diff of two policy files.
use strict;
use warnings;

my ( $old_file, $new_file, $width ) = @ARGV;
$old_file //= 'files/policy-v1.txt';
$new_file //= 'files/policy-v2.txt';
$width    //= 34;

# ---- word wrap ---------------------------------------------------------
sub wrap_text {
    my ( $text, $w, $indent ) = @_;
    $indent //= '';
    my $avail = $w - length $indent;
    die "width too small for indent\n" if $avail < 8;

    my @words = split ' ', $text;
    my @lines;
    my $cur = '';
    for my $word (@words) {
        # split words longer than a whole line, hyphenating
        while ( length $word > $avail ) {
            my $take = $avail - 1;
            $take -= length($cur) + 1 if length $cur;
            if ( $take < 4 ) {    # no room on this line, flush first
                push @lines, $cur;
                $cur  = '';
                next;
            }
            my $head = substr $word, 0, $take;
            $word = substr $word, $take;
            $cur = length $cur ? "$cur $head-" : "$head-";
            push @lines, $cur;
            $cur = '';
        }
        if ( !length $cur ) { $cur = $word }
        elsif ( length($cur) + 1 + length($word) <= $avail ) {
            $cur .= " $word";
        }
        else { push @lines, $cur; $cur = $word }
    }
    push @lines, $cur if length $cur;
    # first line unindented, continuation lines carry the hanging indent
    return map { $_ ? $indent . $lines[$_] : $lines[$_] } 0 .. $#lines;
}

my $slurp = sub {
    my ($p) = @_;
    open my $fh, '<', $p or die "open $p: $!\n";
    my @l = <$fh>;
    chomp @l;
    return @l;
};

# demonstrate the wrapper on a nasty paragraph
my $para =
    'Deployment gating uses the internal '
  . 'continuous-integration-and-delivery-orchestrator service; '
  . 'exceptions need sign-off.';
print "--- wrapped (width $width, hanging indent) ---\n";
my @wrapped = wrap_text( $para, $width, '    ' );
printf "%2d|%s\n", length $_, $_ for @wrapped;
die "wrap overflow!\n" if grep { length > $width } @wrapped;

# ---- LCS diff ----------------------------------------------------------
my @a = $slurp->($old_file);
my @b = $slurp->($new_file);

# full DP table for traceback
my @lcs;
for my $i ( 0 .. @a ) { $lcs[$i][0] = 0 }
for my $j ( 0 .. @b ) { $lcs[0][$j] = 0 }
for my $i ( 1 .. @a ) {
    for my $j ( 1 .. @b ) {
        $lcs[$i][$j] =
          $a[ $i - 1 ] eq $b[ $j - 1 ]
          ? $lcs[ $i - 1 ][ $j - 1 ] + 1
          : ( $lcs[ $i - 1 ][$j] >= $lcs[$i][ $j - 1 ]
            ? $lcs[ $i - 1 ][$j]
            : $lcs[$i][ $j - 1 ] );
    }
}

# traceback produces reversed edit script
my ( @script, $i, $j ) = ();
( $i, $j ) = ( scalar @a, scalar @b );
while ( $i > 0 || $j > 0 ) {
    if ( $i > 0 && $j > 0 && $a[ $i - 1 ] eq $b[ $j - 1 ] ) {
        unshift @script, [ ' ', $a[ --$i ] ]; $j--;
    }
    elsif ( $j > 0
        && ( $i == 0 || $lcs[$i][ $j - 1 ] >= $lcs[ $i - 1 ][$j] ) )
    {
        unshift @script, [ '+', $b[ --$j ] ];
    }
    else {
        unshift @script, [ '-', $a[ --$i ] ];
    }
}

print "--- diff $old_file $new_file ---\n";
my ( $added, $removed ) = ( 0, 0 );
for my $step (@script) {
    my ( $op, $line ) = @$step;
    $added++   if $op eq '+';
    $removed++ if $op eq '-';
    print "$op $line\n";
}
printf "summary: +%d -%d unchanged %d (lcs %d)\n", $added, $removed,
  scalar(@script) - $added - $removed, $lcs[-1][-1];

# similarity ratio from the LCS
printf "similarity: %.1f%%\n",
  100 * 2 * $lcs[-1][-1] / ( @a + @b );

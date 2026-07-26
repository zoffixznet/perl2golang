package TextUtil::Stats;
# Nested package name living in TextUtil/Stats.pm.
use strict;
use warnings;
use Exporter 'import';
use List::Util qw(sum0 max min);

our @EXPORT_OK = qw(word_freq summarize);

sub word_freq {
    my ($text) = @_;
    my %freq;
    $freq{ lc $1 }++ while $text =~ /([A-Za-z']+)/g;
    return \%freq;
}

sub summarize {
    my ($freq) = @_;
    my @words = sort {
        $freq->{$b} <=> $freq->{$a}    # most frequent first,
          or $a cmp $b                 # then alphabetically
    } keys %$freq;
    my @lens = map { length } @words;
    return {
        distinct => scalar @words,
        total    => sum0( values %$freq ),
        longest  => max(@lens),
        shortest => min(@lens),
        top      => [ @words[ 0 .. ( $#words < 2 ? $#words : 2 ) ] ],
        package  => __PACKAGE__,
    };
}

1;

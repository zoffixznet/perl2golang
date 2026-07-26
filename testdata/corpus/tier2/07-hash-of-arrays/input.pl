#!/usr/bin/perl
use strict;
use warnings;

# Log triage: groups lines into a hash of arrays keyed by severity, then
# reports each bucket in a stable order.

my %by_level;
my @order;

open my $fh, '<', 'files/app.log' or die "cannot read log: $!\n";
while (my $line = <$fh>) {
    chomp $line;
    next unless $line =~ /^(\w+)\s+(.*)$/;
    my ($level, $msg) = ($1, $2);
    push @order, $level unless exists $by_level{$level};
    push @{ $by_level{$level} }, $msg;
}
close $fh or die "close: $!\n";

my %rank = (ERROR => 0, WARN => 1, INFO => 2, DEBUG => 3);

for my $level (sort { $rank{$a} <=> $rank{$b} } keys %by_level) {
    my $msgs = $by_level{$level};
    printf "%-5s (%d)\n", $level, scalar @$msgs;
    printf "  - %s\n", $_ for @$msgs;
}

print "first-seen order: ", join(' ', @order), "\n";

# Invert: message keyword -> list of levels it appeared under.
my %keyword;
for my $level (keys %by_level) {
    for my $msg (@{ $by_level{$level} }) {
        my ($word) = $msg =~ /^(\w+)/;
        push @{ $keyword{$word} }, $level;
    }
}
for my $word (sort keys %keyword) {
    printf "%-10s => %s\n", $word, join(',', sort @{ $keyword{$word} });
}

# Flatten back out, counting elements across all buckets.
my $total = 0;
$total += scalar @{ $by_level{$_} } for keys %by_level;
print "total messages: $total\n";

my @errors = @{ $by_level{ERROR} };
print "last error: $errors[-1]\n";

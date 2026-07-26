use strict;
use warnings;

my %count;
my $total = 0;
my $lines = 0;
while (my $line = <STDIN>) {
    chomp $line;
    $lines++;
    for my $word (split /\s+/, lc $line) {
        next if $word eq '';
        $count{$word}++;
        $total++;
    }
}
print "lines:  $lines\n";
print "words:  $total\n";
print "unique: ", scalar(keys %count), "\n";
print "--- by frequency, then alphabetically ---\n";
for my $w (sort { $count{$b} <=> $count{$a} or $a cmp $b } keys %count) {
    printf "%-8s %2d\n", $w, $count{$w};
}

print "--- words seen more than once ---\n";
my @repeats;
for my $w (sort keys %count) {
    push @repeats, $w if $count{$w} > 1;
}
print @repeats ? join(", ", @repeats) . "\n" : "none\n";
my $max = 0;
my $best = '';
for my $w (sort keys %count) {
    if ($count{$w} > $max) { $max = $count{$w}; $best = $w }
}
print "most common: $best ($max)\n";

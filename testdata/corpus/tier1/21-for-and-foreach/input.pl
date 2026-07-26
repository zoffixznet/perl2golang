use strict;
use warnings;

for (my $i = 0; $i < 5; $i++) { print "c-style $i\n" }
for (my $i = 10; $i > 0; $i -= 3) { print "step $i\n" }
for (my ($i, $j) = (0, 10); $i < $j; $i++, $j--) { print "converge $i $j\n" }

print "--- foreach ---\n";
my @colors = qw(red green blue);
foreach my $c (@colors) { print "named  $c\n" }
foreach (@colors)       { print "topic  $_\n" }
for my $c (@colors)     { print "for is foreach: $c\n" }

print "--- index-driven ---\n";
for my $i (0 .. $#colors) { print "$i => $colors[$i]\n" }

print "--- the loop variable aliases the element ---\n";
my @nums = (1 .. 4);
for my $n (@nums) { $n *= 10 }
print "after aliasing loop: @nums\n";
my @words = qw(a b c);
$_ .= "!" for @words;
print "after topic aliasing: @words\n";

print "--- nested ---\n";
for my $r (1 .. 3) {
    my @row;
    for my $c (1 .. 4) { push @row, $r * $c }
    print join("\t", @row), "\n";
}

print "--- ranges and reverse ---\n";
print join(",", 1 .. 5), "\n";
print join(",", reverse 1 .. 5), "\n";
print join(",", 'aa' .. 'ad'), "\n";

print "--- the topic is restored after the loop ---\n";
$_ = "outer";
for (1 .. 3) { }
print "topic is still '$_'\n";

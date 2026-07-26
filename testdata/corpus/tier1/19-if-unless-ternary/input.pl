use strict;
use warnings;

for my $score (95, 82, 71, 64, 50) {
    my $grade;
    if    ($score >= 90) { $grade = 'A' }
    elsif ($score >= 80) { $grade = 'B' }
    elsif ($score >= 70) { $grade = 'C' }
    elsif ($score >= 60) { $grade = 'D' }
    else                 { $grade = 'F' }
    printf "%3d -> %s\n", $score, $grade;
}

print "--- unless ---\n";
my $n = 0;
unless ($n) { print "n is false\n" }
unless ($n > 10) { print "not big\n" } else { print "big\n" }
my @list = ();
unless (@list) { print "list is empty\n" }

print "--- ternary ---\n";
for my $v (1, 2, 3, 4) {
    print "$v is ", ($v % 2 == 0 ? "even" : "odd"), "\n";
}
my $x = 5;
my $label = $x < 0  ? "negative"
          : $x == 0 ? "zero"
          :           "positive";
print "5 is $label\n";
for my $t (-3, 0, 3) {
    printf "%2d: %s\n", $t, ($t < 0 ? "negative" : $t == 0 ? "zero" : "positive");
}

print "--- ternary as an lvalue target picker ---\n";
my ($evens, $odds) = (0, 0);
for my $i (1 .. 6) {
    ($i % 2 ? $odds : $evens) += $i;
}
print "evens=$evens odds=$odds\n";

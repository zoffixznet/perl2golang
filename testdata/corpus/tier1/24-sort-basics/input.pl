use strict;
use warnings;

my @words = qw(banana Apple cherry date apple Cherry);
print "default (string):  ", join(" ", sort @words), "\n";
print "case insensitive:  ", join(" ", sort { lc($a) cmp lc($b) } @words), "\n";
print "descending:        ", join(" ", sort { $b cmp $a } @words), "\n";
print "reverse sort:      ", join(" ", reverse sort @words), "\n";

print "--- numbers ---\n";
my @nums = (10, 9, 100, 2, 33, -5);
print "default (string):  ", join(" ", sort @nums), "\n";
print "numeric ascending: ", join(" ", sort { $a <=> $b } @nums), "\n";
print "numeric descending:", join(" ", sort { $b <=> $a } @nums), "\n";
print "original untouched: @nums\n";

print "--- by a computed key ---\n";
print "by length then name: ",
      join(" ", sort { length($a) <=> length($b) or $a cmp $b } @words), "\n";
print "by last character:   ",
      join(" ", sort { substr($a, -1) cmp substr($b, -1) or $a cmp $b } @words), "\n";

print "--- sort is stable-looking only if the comparator is total ---\n";
my @dup = (3, 1, 3, 1, 2);
print "with duplicates:   ", join(" ", sort { $a <=> $b } @dup), "\n";

print "--- sorting the keys of a hash ---\n";
my %len;
$len{$_} = length $_ for @words;
for my $w (sort keys %len) {
    printf "%-7s %d\n", $w, $len{$w};
}

print "--- sort in scalar-ish spots ---\n";
my @sorted = sort { $a <=> $b } @nums;
print "min=$sorted[0] max=$sorted[-1]\n";

#!/usr/bin/perl
use strict;
use warnings;

# Parentheses in Perl are two different things wearing the same costume: they
# group an expression, and they build a list. Which one you get is decided by
# the context around them, not by what is inside them.

print "--- grouping around one value ---\n";
my $n = 13;
my $m = 3;
my $d = 5;
printf "(%d - %d) / %d      = %s\n", $n, $m, $d, ($n - $m) / $d;
printf "%d - %d / %d        = %s\n", $n, $m, $d, $n - $m / $d;
printf "(%d + 1) * 2        = %d\n", $n, ($n + 1) * 2;
printf "((%d + 1) * 2) %% 7  = %d\n", $n, (($n + 1) * 2) % 7;
printf "-(%d - %d)           = %d\n", $m, $n, -($m - $n);
printf "2 ** (1 + 2)        = %d\n", 2 ** (1 + 2);

print "--- the identity Perl guarantees for %% ---\n";
for my $pair ("13:5", "-13:5", "13:-5", "-13:-5") {
    my ($a, $b) = split /:/, $pair;
    my $mod = $a % $b;
    my $quo = ($a - $mod) / $b;
    printf "%-7s mod=%3d  quotient=%3d  quotient*b+mod=%4d\n",
        $pair, $mod, $quo, $quo * $b + $mod;
}

print "--- an array where a number is wanted ---\n";
my @items = ("pen", "cup", "map", "key");
my $count = @items;
print "count via assignment: $count\n";
print "count plus one:       ", @items + 1, "\n";
print "twice the count:      ", 2 * @items, "\n";
print "is it four?           ", (@items == 4 ? "yes" : "no"), "\n";
print "last index:           $#items\n";

print "--- an array where text is wanted ---\n";
print "there are " . @items . " items\n";
print "the word 'items' is " . length("items") . " characters\n";

print "--- grouping in a comparison ---\n";
my $left = 10;
my $right = 4;
print "difference over 5?    ", (($left - $right) > 5 ? "yes" : "no"), "\n";
print "sum under 20?         ", (($left + $right) < 20 ? "yes" : "no"), "\n";

print "--- the comma operator in scalar context ---\n";
my @queue = ("a", "b", "c");
my $second = (shift @queue, shift @queue);
print "the comma expression yielded: $second\n";
print "and it consumed both, leaving: @queue\n";

print "--- parentheses that really are a list ---\n";
my @three = (1, 2, 3);
my @one = (9);
my ($only) = @items;
print "three: @three\n";
print "one:   @one\n";
print "only:  $only\n";

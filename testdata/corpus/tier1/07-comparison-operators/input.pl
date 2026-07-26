use strict;
use warnings;

my ($x, $y) = (10, 9);
print "10 == 9  -> ", ($x == $y ? "true" : "false"), "\n";
print "10 eq 9  -> ", ($x eq $y ? "true" : "false"), "\n";
my ($p, $q) = ("10", "10.0");
print "'10' == '10.0' -> ", ($p == $q ? "true" : "false"), "\n";
print "'10' eq '10.0' -> ", ($p eq $q ? "true" : "false"), "\n";

print "--- the numeric and string comparison chains ---\n";
printf "%2d <  %2d -> %s\n", 2, 10, (2 <  10 ? "true" : "false");
printf "%2s lt %2s -> %s\n", 2, 10, ("2" lt "10" ? "true" : "false");
printf "%2d >= %2d -> %s\n", 5, 5, (5 >= 5 ? "true" : "false");
printf "%2s ge %2s -> %s\n", "b", "a", ("b" ge "a" ? "true" : "false");
printf "!= -> %s   ne -> %s\n", (3 != 4 ? "true" : "false"), ("a" ne "a" ? "true" : "false");

print "--- spaceship and cmp ---\n";
printf "%d <=> %d = %2d\n", 3, 7, 3 <=> 7;
printf "%d <=> %d = %2d\n", 7, 3, 7 <=> 3;
printf "%d <=> %d = %2d\n", 5, 5, 5 <=> 5;
printf "%-6s cmp %-6s = %2d\n", "apple", "banana", "apple" cmp "banana";
printf "%-6s cmp %-6s = %2d\n", "b", "a", "b" cmp "a";
printf "%-6s cmp %-6s = %2d\n", "x", "x", "x" cmp "x";
printf "%-6s cmp %-6s = %2d\n", "Zed", "apple", "Zed" cmp "apple";

print "--- same data, two sorts ---\n";
my @nums = (10, 9, 100, 2, 33);
print "string sort:  ", join(",", sort @nums), "\n";
print "numeric sort: ", join(",", sort { $a <=> $b } @nums), "\n";

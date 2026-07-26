use strict;
use warnings;

printf "%%d      : %d\n", 42;
printf "%%d trunc: %d\n", 42.9;
printf "%%d neg  : %d\n", -42.9;
printf "%%s      : %s\n", "text";
printf "%%s num  : %s\n", 42.5;
printf "%%f      : %f\n", 3.14159;
printf "%%.2f    : %.2f\n", 3.14159;
printf "%%.0f    : %.0f %.0f %.0f\n", 0.5, 1.5, 2.5;

print "--- width and alignment ---\n";
printf "[%5d]\n", 42;
printf "[%-5d]\n", 42;
printf "[%05d]\n", 42;
printf "[%05d]\n", -42;
printf "[%8s]\n", "ab";
printf "[%-8s]\n", "ab";
printf "[%.3s]\n", "abcdef";
printf "[%*d]\n", 6, 42;

print "--- radix ---\n";
printf "%%x %%X : %x %X\n", 255, 255;
printf "%%#x    : %#x\n", 255;
printf "%%o %%#o: %o %#o\n", 8, 8;
printf "%%b %%#b: %b %#b\n", 10, 10;

print "--- exponent and general ---\n";
printf "%%e : %e\n", 12345.6789;
printf "%%g : %g\n", 0.0000123;
printf "%%g : %g\n", 1234567.0;
printf "%%g : %g\n", 100.0;

print "--- signs and literals ---\n";
printf "%%+d: %+d %+d\n", 5, -5;
printf "%% d: [% d]\n", 5;
printf "literal percent: 50%%\n";

print "--- sprintf into a variable ---\n";
my $row = sprintf("%-10s|%6.2f|%4d", "widget", 19.5, 7);
print "$row\n";
print "length of that row: ", length($row), "\n";
printf "%s has %d items costing %.2f each\n", "cart", 3, 6.5;

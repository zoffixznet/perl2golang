use strict;
use warnings;

my $verbose = 0;
my $debug   = 1;
print "&& -> ", (($verbose && $debug) ? "yes" : "no"), "\n";
print "|| -> ", (($verbose || $debug) ? "yes" : "no"), "\n";
print "!  -> ", ((!$verbose)          ? "yes" : "no"), "\n";

print "--- these return an operand, not a boolean ---\n";
my $name  = "" || "default";
my $first = "alpha" && "beta";
my $zero  = 0 || "";
print "'' || 'default'   = '$name'\n";
print "'alpha' && 'beta' = '$first'\n";
print "0 || ''           = '$zero'\n";

print "--- short circuiting ---\n";
my $calls = 0;
sub bump { $calls++; return 1 }
my $r1 = 0 && bump();
my $r2 = 1 || bump();
print "after 0&&f and 1||f, calls=$calls\n";
my $r3 = 1 && bump();
my $r4 = 0 || bump();
print "after 1&&f and 0||f, calls=$calls\n";

print "--- low precedence word forms ---\n";
sub note { print "note: $_[0]\n"; return 1 }
my $falsy = 0;
my $r = $falsy or note("'or' binds looser than '=', so \$r was already assigned");
print "r='$r'\n";
my $x = 5;
$x > 3  and print "x is big\n";
$x > 10 or  print "x is not huge\n";
print "not-form\n" if not $x > 10;

print "--- combined ---\n";
for my $n (1 .. 6) {
    if ($n % 2 == 0 and $n > 2) { print "$n: even and above two\n" }
    elsif ($n % 3 == 0 or $n == 1) { print "$n: divisible by three or is one\n" }
    else { print "$n: neither\n" }
}
print "xor: ", ((1 xor 0) ? "true" : "false"), " ", ((1 xor 1) ? "true" : "false"), "\n";

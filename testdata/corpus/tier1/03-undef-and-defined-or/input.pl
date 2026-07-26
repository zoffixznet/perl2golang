use strict;
use warnings;

my $x;
print defined($x) ? "defined\n" : "undef\n";
my $y = $x // "fallback";
print "$y\n";
my $zero = 0;
my $dor  = $zero // "dflt";
my $or   = $zero || "dflt";
print "defined-or=$dor  logical-or=$or\n";
my $empty = "";
print "empty // : ", ($empty // "dflt"), "\n";
print "empty ||  : ", ($empty || "dflt"), "\n";
my %conf = (host => "localhost", port => 0);
print "port=", ($conf{port} // 8080), "\n";
print "user=", ($conf{user} // "anonymous"), "\n";
my $count;
$count //= 0;
$count++;
print "count=$count\n";
my $name = "";
$name ||= "unnamed";
print "name=$name\n";
my @vals = (1, undef, 3, undef, 5);
my $total = 0;
my $missing = 0;
for my $v (@vals) {
    if (defined $v) { $total += $v }
    else            { $missing++ }
}
print "total=$total missing=$missing\n";
$x = "set";
print "now: $x\n";
undef $x;
print defined($x) ? "defined\n" : "undef again\n";

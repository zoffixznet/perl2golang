use strict;
use warnings;

my %age = (
    alice => 30,
    bob   => 25,
    carol => 35,
);
print "alice is $age{alice}\n";
$age{dave} = 28;
$age{'with space'} = 1;

print "--- sorted iteration ---\n";
for my $name (sort keys %age) {
    printf "%-12s %3d\n", $name, $age{$name};
}
print "pairs: ", scalar(keys %age), "\n";
my $total = 0;
$total += $_ for values %age;
print "sum of values: $total\n";

print "--- exists vs defined vs truth ---\n";
$age{eve} = 0;
$age{frank} = undef;
for my $k (qw(alice eve frank zed)) {
    printf "%-6s exists=%d defined=%d true=%d\n",
        $k,
        (exists  $age{$k} ? 1 : 0),
        (defined $age{$k} ? 1 : 0),
        ($age{$k}         ? 1 : 0);
}

print "--- delete ---\n";
my $gone = delete $age{bob};
print "deleted bob (was $gone)\n";
print "keys now: ", join(",", sort keys %age), "\n";

print "--- a lookup does not create the key ---\n";
my $probe = $age{ghost};
print "after reading \$age{ghost}, exists=",
      (exists $age{ghost} ? 1 : 0), "\n";
%age = ();
print "cleared: ", scalar(keys %age), "\n";

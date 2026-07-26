use strict;
use warnings;

my @a = (10, 20, 30, 40);
my $count = @a;
my ($first) = @a;
print "scalar context: $count\n";
print "list context:   $first\n";
print "scalar():       ", scalar(@a), "\n";
print "numeric use:    ", @a + 0, "\n";
print "boolean:        ", (@a ? "nonempty" : "empty"), "\n";

print "--- the same call in two contexts ---\n";
my @rev = reverse @a;
my $rev = reverse @a;
print "list reverse:   @rev\n";
print "scalar reverse: $rev\n";

print "--- split in two contexts ---\n";
my @parts = split /,/, "a,b,c";
my $nparts = split /,/, "a,b,c";
print "split list:     @parts\n";
print "split scalar:   $nparts\n";

print "--- hash context ---\n";
my %h = (x => 1, y => 2, z => 3);
my $nkeys = keys %h;
my @flat = %h;
print "keys scalar:    $nkeys\n";
print "flattened len:  ", scalar(@flat), "\n";
print "hash boolean:   ", (%h ? "nonempty" : "empty"), "\n";

print "--- a sub sees its own context ---\n";
sub ctx {
    print "called in ",
          (wantarray ? "list" : defined(wantarray) ? "scalar" : "void"),
          " context\n";
    return 1;
}
my @l = ctx();
my $s = ctx();
ctx();
print "got back @l and $s\n";

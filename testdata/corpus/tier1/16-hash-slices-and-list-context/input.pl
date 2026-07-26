use strict;
use warnings;

my %conf = (host => "localhost", port => 8080, debug => 1, name => "svc");
my @vals = @conf{qw(host port)};
print "slice:        @vals\n";
my ($h, $p) = @conf{'host', 'port'};
print "unpacked:     h=$h p=$p\n";
@conf{qw(user pass)} = ("admin", "hunter2");
print "keys now:     ", join(",", sort keys %conf), "\n";
my %sub;
@sub{qw(host port)} = @conf{qw(host port)};
for my $k (sort keys %sub) { print "sub $k=$sub{$k}\n" }
delete @conf{qw(pass debug)};
print "after delete slice: ", join(",", sort keys %conf), "\n";

print "--- hash in list context ---\n";
my @flat = %sub;
print "flattened to ", scalar(@flat), " elements\n";
my %rebuilt = @flat;
print "rebuilt keys: ", join(",", sort keys %rebuilt), "\n";
my %inverted = reverse %sub;
print "inverted keys: ", join(",", sort keys %inverted), "\n";

print "--- building a hash from two arrays ---\n";
my @names = qw(red green blue);
my @codes = ("#f00", "#0f0", "#00f");
my %color;
@color{@names} = @codes;
for my $k (sort keys %color) { print "$k -> $color{$k}\n" }

print "--- merging with later keys winning ---\n";
my %defaults = (color => "red", size => "M", qty => 1);
my %opts = (%defaults, size => "L", qty => 3);
for my $k (sort keys %opts) { print "opt $k=$opts{$k}\n" }

print "--- a set built from a list ---\n";
my %seen;
$seen{$_} = 1 for qw(a b a c b a);
print "distinct: ", join(",", sort keys %seen), "\n";
print "is 'b' present: ", (exists $seen{b} ? "yes" : "no"), "\n";

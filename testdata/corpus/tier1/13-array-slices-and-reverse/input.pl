use strict;
use warnings;

my @w = qw(alpha beta gamma delta epsilon);
print "pick 1 and 3:   @w[1, 3]\n";
print "range slice:    @w[0 .. 2]\n";
print "negative slice: @w[-2, -1]\n";
print "reordered:      @w[4, 0, 2]\n";
my @idx = (4, 0);
print "slice by list:  @w[@idx]\n";
my @some = @w[1 .. 3];
print "captured slice: @some (", scalar(@some), ")\n";
@w[0, 1] = ("A", "B");
print "after assign:   @w\n";
@w[1, 0] = @w[0, 1];
print "swapped:        @w\n";

print "--- reverse and sort do not mutate ---\n";
print "reversed:       ", join(",", reverse @w), "\n";
print "sorted:         ", join(",", sort @w), "\n";
print "original:       @w\n";

print "--- reverse in scalar context reverses characters ---\n";
my $s = reverse "stressed";
print "scalar reverse: $s\n";
print "forced:         ", scalar(reverse("hello")), "\n";
print "list of one:    ", join("", reverse("hello")), "\n";

print "--- tail slices ---\n";
print "everything but the first: ", join(",", @w[1 .. $#w]), "\n";
print "last two:                 ", join(",", @w[$#w-1 .. $#w]), "\n";
my ($first, @rest) = @w;
print "first=$first rest=@rest\n";

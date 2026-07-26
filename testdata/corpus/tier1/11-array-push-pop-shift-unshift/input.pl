use strict;
use warnings;

my @stack;
push @stack, 1;
push @stack, 2, 3;
print "stack: @stack\n";
my $top = pop @stack;
print "popped $top, left: @stack\n";
my @queue = ("b", "c");
unshift @queue, "a";
push @queue, "d";
print "queue: @queue\n";
my $head = shift @queue;
print "shifted $head, left: @queue\n";
my @empty;
my $nothing = pop @empty;
print "pop on empty array: ", (defined $nothing ? $nothing : "undef"), "\n";
print "empty array still has ", scalar(@empty), " elements\n";

print "--- push returns the new length ---\n";
my @a = (1, 2);
my $len = push @a, 3, 4;
print "push returned $len, array is @a\n";
my $ulen = unshift @a, 0;
print "unshift returned $ulen, array is @a\n";

print "--- push flattens ---\n";
my @b = (1);
my @more = (2, 3);
push @b, @more, 4, (5, 6);
print "b = @b (", scalar(@b), " elements)\n";

print "--- a queue drained front to back ---\n";
my @work = (10, 20, 30);
while (defined(my $item = shift @work)) {
    print "handling $item, ", scalar(@work), " left\n";
}

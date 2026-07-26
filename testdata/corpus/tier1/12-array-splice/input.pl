use strict;
use warnings;

my @a = ('a' .. 'g');
print "start:            @a\n";
my @removed = splice(@a, 2, 3);
print "removed 3 at 2:   @removed\n";
print "remaining:        @a\n";
splice(@a, 1, 0, 'X', 'Y');
print "inserted at 1:    @a\n";
my @tail = splice(@a, -2);
print "spliced tail:     @tail\n";
print "remaining:        @a\n";
splice(@a, 1, 1, 'Z');
print "replaced one:     @a\n";
my $one = splice(@a, 0, 1);
print "scalar splice:    $one\n";
print "remaining:        @a (", scalar(@a), ")\n";

print "--- more shapes ---\n";
my @b = (1 .. 6);
splice(@b, 3);
print "truncate at 3:    @b\n";
splice(@b, 0, 0, 0);
print "prepend:          @b\n";
splice(@b, 1, 2, 'p', 'q', 'r');
print "grow in place:    @b\n";
my @all = splice(@b);
print "splice everything: @all / left ", scalar(@b), "\n";

print "--- negative length ---\n";
my @c = ('a' .. 'f');
my @mid = splice(@c, 1, -2);
print "splice(1,-2) took @mid, left @c\n";

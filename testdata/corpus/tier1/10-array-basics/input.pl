use strict;
use warnings;

my @fruit = ("apple", "banana", "cherry", "date");
print "count:      ", scalar(@fruit), "\n";
print "last index: $#fruit\n";
print "first:      $fruit[0]\n";
print "last:       $fruit[-1]\n";
print "2nd last:   $fruit[-2]\n";
print "interp:     @fruit\n";
print "joined:     ", join(" | ", @fruit), "\n";
my @nums = (1 .. 10);
print "range:      @nums\n";
my @letters = ('a' .. 'e');
print "letters:    @letters\n";
my @qw = qw(one two three);
print "qw:         @qw\n";
my @zeros = (0) x 5;
print "repeated:   @zeros\n";
my @pattern = (1, 2) x 3;
print "pattern:    @pattern\n";
my @copy = @fruit;
$copy[0] = "APPLE";
print "copy is independent: $fruit[0] vs $copy[0]\n";
print "out of range read is undef: ",
      (defined $fruit[99] ? "defined" : "undef"), "\n";
print "count unchanged after that read: ", scalar(@fruit), "\n";
$fruit[6] = "fig";
print "after assigning index 6, count=", scalar(@fruit),
      " last index=$#fruit\n";
print "index 5 is ", (defined $fruit[5] ? "defined" : "undef"), "\n";
$#fruit = 2;
print "after \$#fruit = 2: @fruit\n";
my @cleared = (1 .. 3);
@cleared = ();
print "cleared count: ", scalar(@cleared), "\n";

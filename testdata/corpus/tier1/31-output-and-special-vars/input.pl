use strict;
use warnings;
use feature 'say';

print "print takes a list: ", "a", "b", "c", "\n";
print "no separator is inserted between them\n";
printf "printf: %s=%d\n", "x", 1;
say "say appends a newline";
my @list = ("x", "y", "z");
say "interpolated: @list";
say join(",", @list);
say for 1 .. 3;

print "--- \$, output field separator ---\n";
{
    local $, = ", ";
    print "one", "two", "three";
    print "\n";
}
print "one", "two", "three";
print "\n";

print "--- \$\\ output record separator ---\n";
{
    local $\ = "!\n";
    print "auto terminated";
    print "so is this";
}
print "back to manual\n";

print "--- \$\" list separator inside interpolation ---\n";
{
    local $" = " | ";
    say "@list";
}
say "@list";

print "--- explicit filehandles ---\n";
print STDOUT "explicit STDOUT\n";
print STDERR "this line goes to stderr and is not part of expected_stdout\n";
printf STDOUT "%s\n", "printf to an explicit handle";

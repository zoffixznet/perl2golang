use strict;
use warnings;

my @args = @ARGV;
print "checking ", scalar(@args), " argument(s)\n";
if (@args < 2) {
    print "usage: input.pl NUMBER NUMBER [NUMBER ...]\n";
    print "exiting with status 2\n";
    exit 2;
}
my $sum = 0;
for my $a (@args) {
    $sum += $a;
}
print "sum=$sum\n";
exit 0;

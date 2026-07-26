use strict;
use warnings;

my @n = (1 .. 5);
print "$_ " for @n;
print "\n";
my $sum = 0;
$sum += $_ foreach @n;
print "sum=$sum\n";
print "big\n"   if $sum > 10;
print "small\n" unless $sum > 10;
my $i = 3;
print "tick $i\n" while $i-- > 0;
print "i ended at $i\n";
my $j = 0;
$j++ until $j >= 4;
print "j=$j\n";
my @squares;
push @squares, $_ * $_ for 1 .. 5;
print "squares: @squares\n";
my $msg = "ok";
$msg = "empty" unless @n;
print "msg=$msg\n";
my %h;
$h{$_} = length $_ for qw(pear fig banana);
print join(", ", map { "$_=$h{$_}" } sort keys %h), "\n";
print "line $_\n" for reverse 1 .. 3;
my $label = "";
$label .= "a" for 1 .. 3;
print "label=$label\n";
for my $v (0, 1, 2) {
    print "v=$v is zero\n" if $v == 0;
    print "v=$v is one\n"  if $v == 1;
    print "v=$v is other\n" if $v > 1;
}
my @nonzero;
for my $v (0, 4, 0, 9) {
    push @nonzero, $v if $v;
}
print "nonzero: @nonzero\n";

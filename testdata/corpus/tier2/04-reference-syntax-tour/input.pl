#!/usr/bin/perl
use strict;
use warnings;

# Every dereference spelling Perl offers, plus ref() type checks. This is
# the kind of code that shows up in the middle of a data-munging script
# once the structures stop being flat.

my $num    = 42;
my @words  = qw(alpha beta gamma);
my %colour = (sky => 'blue', grass => 'green');
my $code   = sub { my ($x) = @_; return $x * $x };

my $sref = \$num;
my $aref = \@words;
my $href = \%colour;
my $cref = $code;

print "scalar: $$sref and ${$sref}\n";

print "array:  @$aref\n";
print "array:  @{$aref}\n";
print "elem0:  $$aref[0] / ${$aref}[0] / $aref->[0]\n";
print "count:  ", scalar(@$aref), " last index ", $#$aref, " / ", $#{$aref}, "\n";
print "slice:  @{$aref}[0,2]\n";

print "hash:   ", join(',', map { "$_=$href->{$_}" } sort keys %$href), "\n";
print "value:  $$href{sky} / ${$href}{sky} / $href->{sky}\n";
print "hslice: @{$href}{qw(sky grass)}\n";

print "code:   ", $cref->(7), " / ", &$cref(8), " / ", &{$cref}(9), "\n";

# Anonymous constructors and references to references.
my $anon_a = [ 10, 20, [ 30, 40 ] ];
my $anon_h = { name => 'ada', tags => [qw(math logic)] };
my $refref = \$aref;

print "nested: $anon_a->[2][1] and $anon_a->[2]->[0]\n";
print "mixed:  $anon_h->{tags}[1]\n";
print "refref: ", $$refref->[1], " / ", ${$$refref}[2], "\n";

for my $thing ($sref, $aref, $href, $cref, $refref, \$refref, $num) {
    my $t = ref $thing;
    printf "ref() => %s\n", $t eq '' ? '(not a reference)' : $t;
}

# A reference stored inside a plain array, retrieved and used.
my @registry = (\@words, \%colour, $code);
print "registry: ", scalar(@{ $registry[0] }), " words, ",
      scalar(keys %{ $registry[1] }), " colours, sq(5)=", $registry[2]->(5), "\n";

# Taking a reference to an anonymous copy vs the original.
my @copy = @$aref;
push @copy, 'delta';
print "orig=@words copy=@copy\n";

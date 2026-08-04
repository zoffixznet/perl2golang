#!/usr/bin/perl
# Four places where a mechanical translation quietly changes the answer:
# assigning a list, reading one where a list was expected, stepping a variable
# for its value, and taking something out of a hash.
use strict;
use warnings;

print "--- a list assignment copies ---\n";
my @original = ( 'apple', 'banana', 'cherry' );
my @copy     = @original;
$copy[0] = 'APPLE';
printf "original: %s\n", join( ',', @original );
printf "copy:     %s\n", join( ',', @copy );

my %prices = ( apple => 1, banana => 2 );
my %adjusted = %prices;
$adjusted{apple} = 99;
printf "prices:   apple=%d\n", $prices{apple};
printf "adjusted: apple=%d\n", $adjusted{apple};

print "--- reverse depends on what is wanted ---\n";
my @nums = ( 1, 2, 3, 4 );
my @backwards = reverse @nums;
printf "list:     %s\n", join( ',', @backwards );
my $runtogether = reverse @nums;
printf "one value: %s\n", $runtogether;
my @one = reverse('hello');
printf "one element list: %s\n", $one[0];
printf "one value again:  %s\n", scalar reverse('hello');

print "--- a step used for its value ---\n";
my $i = 3;
print "tick $i\n" while $i-- > 0;
printf "i ended at %d\n", $i;

my $n = 0;
my @taken;
push @taken, $n++ for 1 .. 4;
printf "taken:    %s\n", join( ',', @taken );
printf "n now:    %d\n", $n;

my $m = 5;
printf "pre-step: %d then %d\n", --$m, $m;

print "--- taking a pair out ---\n";
my %stock = ( widget => 7, gadget => 3, doodad => 11 );
my $gone = delete $stock{gadget};
printf "removed:  %d\n", $gone;
printf "left:     %s\n", join( ',', sort keys %stock );
my $missing = delete $stock{nothing};
printf "removing what is not there gives: %s\n", ( defined $missing ? $missing : 'nothing' );

print "--- a hash where a list was wanted ---\n";
my %pair = ( a => 1, b => 2 );
my @flat = %pair;
printf "flat length: %d\n", scalar @flat;
my %merged = ( %pair, c => 3 );
printf "merged keys: %s\n", join( ',', sort keys %merged );

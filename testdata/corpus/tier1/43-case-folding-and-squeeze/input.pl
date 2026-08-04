#!/usr/bin/perl
use strict;
use warnings;

# Two things a replacement can ask for that Go's regexp has no template for.

print "--- case folding inside a replacement ---\n";
my $mixed = 'gRACE hopper WROTE compilers';
my $title = join ' ', map { s/(\w+)/\u\L$1/r } split ' ', $mixed;
print "title case: $title\n";

my $shout = $mixed;
$shout =~ s/(\w+)/\U$1/g;
print "upper:      $shout\n";

my $one = 'hello world';
$one =~ s/(\w)/\u$1/;
print "first only: $one\n";

print "--- tr with the squeeze modifier ---\n";
my $noisy = 'aaabbbcccaaa';
( my $squeezed = $noisy ) =~ tr/a-c//s;
print "squeezed:   $squeezed\n";

my $spaced = "a   b\t\tc";
( my $tidy = $spaced ) =~ tr/ \t/ /s;
print "one space:  [$tidy]\n";

my $kept = $noisy;
my $count = ( $kept =~ tr/a// );
print "counted:    $count a(s), string unchanged: $kept\n";

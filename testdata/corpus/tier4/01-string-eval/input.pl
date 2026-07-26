#!/usr/bin/perl
# TRAP: string eval of a COMPUTED expression -- the code to run does not
# exist until runtime, so no ahead-of-time translation is possible.
use strict;
use warnings;

my @ops = ('+', '*', '-');
my $acc = 10;
for my $op (@ops) {
    my $code = "\$acc = \$acc $op 3;";
    eval $code;
    die $@ if $@;
    print "after '$op': $acc\n";
}

# The expression string can come from anywhere: config files, user input...
my $expr = join($ops[1], 2, 3, 4);    # builds the string "2*3*4"
print "computed: ", eval($expr), "\n";

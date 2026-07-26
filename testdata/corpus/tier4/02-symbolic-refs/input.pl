#!/usr/bin/perl
# TRAP: symbolic references -- variable names and sub names computed at
# runtime and looked up in the symbol table by string.
use strict;
use warnings;
no strict 'refs';

our $alpha = 1;
our $beta  = 2;
our $gamma = 3;

my $total = 0;
for my $name (qw(alpha beta gamma)) {
    $total += $$name;              # reads $alpha, $beta, $gamma by name
    ${ $name . "_seen" } = 1;      # CREATES $alpha_seen etc. at runtime
}
print "total=$total\n";

sub handler_add { return $_[0] + $_[1] }
sub handler_mul { return $_[0] * $_[1] }

for my $which (qw(add mul)) {
    print "$which: ", &{"handler_$which"}(6, 7), "\n";   # call sub by name
}

our $alpha_seen;
print "alpha_seen=$alpha_seen\n";

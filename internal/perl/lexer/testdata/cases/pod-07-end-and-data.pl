#!/usr/bin/perl
# CASE pod-07: `__END__` ends the compilable source. Everything after it is
# available through the DATA filehandle (in package main). `__END__` must be
# alone on its line, at column 0.
use strict; use warnings;

print "pod-07 before-end\n";

my @lines = <DATA>;
chomp @lines;
print "pod-07 data-lines: ", scalar(@lines), "\n";
print "pod-07 data: ", join(" | ", @lines), "\n";

# Text that merely LOOKS like __END__ inside a string does not end the file.
my $s = "__END__ in a string";
print "pod-07 string: $s\n";

__END__
first data line
second data line
=head1 this is just data
q{ unbalanced
"unterminated
__DATA__ nested marker is also just data

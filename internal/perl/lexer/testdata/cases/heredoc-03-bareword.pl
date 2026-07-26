#!/usr/bin/perl
# CASE heredoc-03: `<<EOT` -- bareword terminator, behaves like the double-quoted
# form (interpolating). Note `<< EOT` with a space is a SYNTAX ERROR since 5.28;
# only the quoted forms allow the space.
use strict; use warnings;

my $n = 7;
my $t = <<EOT;
value is $n
EOT
print "heredoc-03 interpolating: $t";

# Space before a bareword terminator is fatal in modern perl -- proven in a child.
my $out = `$^X -e 'my \$x = << EOT;\nbody\nEOT\nprint \$x;' 2>&1`;
$out =~ s/\s+\z//;
print "heredoc-03 space-bareword: ",
      ($out =~ /(?:syntax error|Use of bare|not allowed)/i ? "REJECTED" : "accepted [$out]"), "\n";

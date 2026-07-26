#!/usr/bin/perl
# CASE heredoc-02: `<<'EOT'` -- single-quoted heredoc, NO interpolation and no
# backslash escapes except \\ and \' are not even special: the body is literal.
use strict; use warnings;

my $who = "world";
my $t = <<'EOT';
hello $who
tab-ish: \t not a tab
at: @list backslash: \\
EOT
print "heredoc-02 literal:\n$t";
print "heredoc-02 has-dollar: ", ($t =~ /\$who/ ? "yes" : "no"), "\n";
print "heredoc-02 backslash-count: ", ($t =~ tr/\\//), "\n";

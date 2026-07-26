#!/usr/bin/perl
# CASE angle-03: `<>` is the empty-handle readline (ARGV/STDIN). Two characters,
# one token. `<<>>` (Perl 5.22+) is the safe double-diamond, four characters,
# one token -- and it must NOT be lexed as `<<` heredoc.
use strict; use warnings;

close STDIN;
open STDIN, '<', \"l1\nl2\nl3\n" or die;
@ARGV = ();

my $n = 0;
while (<>) { $n++ }
print "angle-03 diamond-lines: $n\n";

close STDIN;
open STDIN, '<', \"a\nb\n" or die;
@ARGV = ();
my $m = 0;
while (<<>>) { $m++ }
print "angle-03 double-diamond-lines: $m\n";

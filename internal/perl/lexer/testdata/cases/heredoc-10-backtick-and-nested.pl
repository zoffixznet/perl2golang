#!/usr/bin/perl
# CASE heredoc-10: `<<`CMD`` is a command heredoc (runs the body through the
# shell). Also: a heredoc opener inside a string-ish construct on the same line,
# and a heredoc terminator that looks like code.
use strict; use warnings;

my $out = <<`CMD`;
echo backtick-heredoc-ran
CMD
chomp $out;
print "heredoc-10 command: $out\n";

# A heredoc whose terminator is a string that contains punctuation.
my $t = <<"E-N-D";
punctuation terminator
E-N-D
print "heredoc-10 punct-terminator: $t";

# Heredoc opener appearing after a `?:` and inside a sub call on the same line.
my $flag = 1;
my $s = $flag ? <<"YES" : <<"NO";
took the yes branch
YES
took the no branch
NO
print "heredoc-10 ternary: $s";

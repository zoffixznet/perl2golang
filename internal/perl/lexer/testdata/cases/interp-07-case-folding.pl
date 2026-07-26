#!/usr/bin/perl
# CASE interp-07: `\U \L \Q` open a region ended by `\E` (or end of string);
# `\u \l` affect exactly the next character. These are NOT ordinary escapes:
# they are stateful, they nest, and they interact with interpolated values.
use strict; use warnings;

my $v = "mixed Case";

print "interp-07 upper: [\U$v\E] tail\n";
print "interp-07 lower: [\L$v\E] tail\n";
print "interp-07 ucfirst: [\u$v]\n";
print "interp-07 lcfirst: [\l\U$v\E]\n";
print "interp-07 no-E-runs-to-end: [\U$v]\n";
print "interp-07 quotemeta: [\Qa.b*c\E]\n";
print "interp-07 quotemeta-var: [\Q$v.\E]\n";
print "interp-07 nested: [\U outer \L inner \E outer \E]\n";
print "interp-07 u-then-var: [\u\L$v\E]\n";

# \Q inside a regex is the standard way to match a literal string.
my $needle = "a.b";
print "interp-07 regex-Q: ", ("xa.by" =~ /\Q$needle\E/ ? "literal-match" : "no"),
      " / ", ("xaXby" =~ /\Q$needle\E/ ? "BAD" : "correctly-no"), "\n";

# In a heredoc too.
my $hd = <<"EOT";
\U$v\E and \Q$needle\E
EOT
print "interp-07 heredoc: $hd";

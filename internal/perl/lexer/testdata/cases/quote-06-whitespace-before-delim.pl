#!/usr/bin/perl
# CASE quote-06: whitespace (and even comments/newlines) may sit between the
# quote-like operator and its opening delimiter -- EXCEPT that `#` cannot then be
# the delimiter, because after whitespace `#` starts a comment.
use strict; use warnings;

print "quote-06 q-space: ",  q (spaced paren), "\n";
print "quote-06 qq-space: ", qq  {spaced brace}, "\n";
print "quote-06 qw-space: ", join("|", qw  (a b c)), "\n";

my $s = "hay stack";
print "quote-06 m-space: ", ($s =~ m /stack/ ? "match" : "no"), "\n";

(my $t = $s) =~ s {hay}
                  {HAY};
print "quote-06 s-newline-between-parts: $t\n";

# A comment between the operator and the delimiter.
my $u = q     # comment here
  {after a comment};
print "quote-06 comment-before-delim: $u\n";

# After whitespace, `#` is a comment, so `q #foo#` does NOT make a string.
my $out = `$^X -e 'my \$x = q #foo#; print "got[\$x]";' 2>&1`;
$out =~ s/\s+\z//;
print "quote-06 q-space-hash: ", ($out =~ /error|Search pattern|terminator|EOF/i ? "REJECTED: $out" : "[$out]"), "\n";

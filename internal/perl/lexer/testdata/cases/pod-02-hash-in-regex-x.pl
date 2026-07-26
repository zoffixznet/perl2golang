#!/usr/bin/perl
# CASE pod-02: under /x the regex body has its OWN comment rule. A `#` there runs
# to the next newline INSIDE the pattern -- and if the pattern's closing delimiter
# is on that line after the `#`, it is swallowed by the comment.
use strict; use warnings;

my $re = qr{
    ^ \d+      # leading digits
    -
    \w+ $      # trailing word
}x;
print "pod-02 xmatch: ", ("12-ab" =~ $re ? "yes" : "no"), "\n";

# The trap: an unescaped `#` under /x comments out the rest of the LINE, so a
# closing delimiter after it would be eaten. Perl reports the failure.
my $out = `$^X -e 'my \$r = qr/a # b/x; print "built";' 2>&1`;
$out =~ s/\s+\z//;
print "pod-02 hash-then-delim-same-line: [$out]\n";

my $out2 = `$^X -e 'my \$r = qr/a # b/x; print "x" =~ \$r ? 1 : 0;' 2>&1`;
$out2 =~ s/\s+\z//;
print "pod-02 comment-eats-b: [$out2] (pattern is just /a/)\n";

# (?#...) is a regex comment that works with or without /x and has its own end.
print "pod-02 inline-comment: ", ("ab" =~ /a(?# a comment )b/ ? "match" : "no"), "\n";

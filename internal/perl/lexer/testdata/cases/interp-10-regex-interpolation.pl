#!/usr/bin/perl
# CASE interp-10: interpolation inside a REGEX. Same variable grammar as "" but
# `$` and `@` also have regex meanings, so the lexer must know that `$` at the end
# of the pattern (or before `)` or `|`) is the anchor, not a sigil.
use strict; use warnings;

my $word = "cat";
my @alts = ("dog","cat");
my %h = (k => "cat");

print "interp-10 scalar: ",  ("a cat" =~ /$word/       ? "match" : "no"), "\n";
print "interp-10 braced: ",  ("a cats" =~ /${word}s/   ? "match" : "no"), "\n";
print "interp-10 hash: ",    ("a cat" =~ /$h{k}/       ? "match" : "no"), "\n";
print "interp-10 array: ",   ("dog cat" =~ /@alts/     ? "match" : "no"), "\n";  # joined by $"
print "interp-10 expr: ",    ("abc6" =~ /abc@{[ 3*2 ]}/ ? "match" : "no"), "\n";

# `$` as an anchor in the positions where it cannot be a sigil.
print "interp-10 anchor-end: ",   ("xcat" =~ /cat$/    ? "match" : "no"), "\n";
print "interp-10 anchor-paren: ", ("xcat" =~ /(cat$)/  ? "match" : "no"), "\n";
print "interp-10 anchor-alt: ",   ("xcat" =~ /cat$|zz/ ? "match" : "no"), "\n";
print "interp-10 anchor-then-mod: ", ("a\ncat\n" =~ /^cat$/m ? "match" : "no"), "\n";

# `@` in a regex that is not a variable.
print "interp-10 literal-at: ", ('a@b' =~ /a\@b/ ? "match" : "no"), "\n";

# `$)` and `$|` are real punctuation variables, so in a pattern the lexer must
# decide anchor-vs-variable. Perl treats `$)` in a pattern as the anchor.
print "interp-10 dollar-rparen: ", ("cat" =~ /(cat$)/ ? "anchor" : "variable"), "\n";

# A single-quoted pattern does not interpolate at all.
print "interp-10 single-quoted: ", ('$word' =~ m'$word' ? "literal" : "no"), "\n";

# Interpolating a string that itself contains regex metacharacters.
my $meta = "a.c";
print "interp-10 meta-active: ", ("abc" =~ /$meta/    ? "dot matched b" : "no"), "\n";
print "interp-10 meta-quoted: ", ("abc" =~ /\Q$meta\E/ ? "BAD" : "correctly-no"), "\n";

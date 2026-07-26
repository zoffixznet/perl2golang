#!/usr/bin/perl
# CASE quote-05: non-bracketing punctuation delimiters. The SAME character closes
# the string; there is no nesting, only backslash escaping. `#` and `,` are the
# evil ones because they collide with comments and argument separators.
use strict; use warnings;

my $s = "a,b|c#d!e";

print "quote-05 m-pipe: ",  ($s =~ m|b\|c| ? "match" : "no"), "\n";
print "quote-05 m-bang: ",  ($s =~ m!d!    ? "match" : "no"), "\n";
print "quote-05 m-hash: ",  ($s =~ m#c\#d# ? "match" : "no"), "\n";
print "quote-05 m-comma: ", ($s =~ m,a\,b, ? "match" : "no"), "\n";
print "quote-05 m-slash: ", ($s =~ m/e$/   ? "match" : "no"), "\n";

# Substitution with each of them.
(my $a = $s) =~ s|b|B|;
(my $b = $s) =~ s!d!D!;
(my $c = $s) =~ s#c#C#;
(my $d = $s) =~ s,a,A,;
print "quote-05 subst: $a / $b / $c / $d\n";

# `q#...#` versus a real comment on the same line.
my $q = q#not a comment#;   # this IS a comment
print "quote-05 hash-delim: [$q]\n";

# `s,,,` where the delimiter is a comma and the statement has real commas after.
my @out = (do { (my $t = $s) =~ s,\|,PIPE,; $t }, "second");
print "quote-05 comma-delim: ", join(" + ", @out), "\n";

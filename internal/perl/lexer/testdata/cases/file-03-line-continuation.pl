#!/usr/bin/perl
# CASE file-03: Perl has NO line-continuation backslash outside strings --
# statements simply continue until `;`. Inside a double-quoted string a trailing
# backslash escapes the newline away; inside a single-quoted string it does not.
use strict; use warnings;

# A statement spanning several lines with no continuation marker.
my $sum =
    1
  + 2
  + 3;
print "file-03 multiline-expr: $sum\n";

# Literal newline inside a double-quoted string.
my $d = "first
second";
print "file-03 dq-embedded-newline-lines: ", scalar(split /\n/, $d), "\n";

# Backslash-newline inside a double-quoted string: the backslash escapes the
# newline character, which is not special, so the newline SURVIVES.
my $c = "a\
b";
print "file-03 dq-backslash-newline-len: ", length($c),
      " has-newline: ", ($c =~ /\n/ ? "yes" : "no"), "\n";

# Single-quoted: backslash-newline keeps both characters.
my $s = 'a\
b';
print "file-03 sq-backslash-newline-len: ", length($s), "\n";

# A backslash at the end of a shell-style comment continues nothing.
my $after = 1;   # trailing comment with a backslash \
print "file-03 comment-backslash-does-not-continue: $after\n";

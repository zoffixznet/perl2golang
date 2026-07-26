#!/usr/bin/perl
# CASE slash-16: `?PATTERN?` (the match-once operator) was REMOVED in Perl 5.22.
# In modern Perl `?` is only the ternary. This file proves the removal by
# compiling the construct under `perl -c` in a child process.
use strict; use warnings;

my $src = 'my $s = "ab"; if ($s =~ ?b?) { print "hit" }';
my $out = `$^X -e '$src' 2>&1`;
$out =~ s/\s+\z//;
print "slash-16 legacy-?pat? => ", ($out =~ /syntax error/ ? "SYNTAX ERROR (removed)" : "accepted: $out"), "\n";

# The ternary that shares the character.
local $_ = "ab";
print "slash-16 ternary: ", (/b/ ? "yes" : "no"), "\n";

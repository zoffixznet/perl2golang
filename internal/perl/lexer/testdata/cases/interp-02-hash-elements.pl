#!/usr/bin/perl
# CASE interp-02: `$hash{key}` inside a string. The subscript is lexed as CODE,
# so a bareword key is autoquoted, a quoted key keeps its quotes, and a complex
# key is a full expression -- including nested braces and function calls.
use strict; use warnings;

my %h = (
  simple      => "S",
  'with space'=> "WS",
  '5'         => "FIVE",
  'a}b'       => "BRACE",
);
my $k = "simple";
sub key { return "simple" }

print "interp-02 bareword: [$h{simple}]\n";
print "interp-02 single-quoted: [$h{'with space'}]\n";
print "interp-02 double-quoted: [$h{\"with space\"}]\n";
print "interp-02 variable-key: [$h{$k}]\n";
print "interp-02 numeric-key: [$h{5}]\n";
print "interp-02 call-key: [$h{ key() }]\n";
print "interp-02 expr-key: [$h{ 'sim' . 'ple' }]\n";
print "interp-02 brace-in-key: [$h{'a}b'}]\n";

# Nested structures.
my %n = (outer => { inner => "DEEP" });
print "interp-02 nested: [$n{outer}{inner}]\n";
print "interp-02 nested-arrow: [$n{outer}->{inner}]\n";

# A literal `{` after a variable that is NOT a subscript.
my $v = "V";
print "interp-02 literal-brace: [${v} {not a subscript}]\n";

# Hash slice in a string interpolates as a list.
my @slice = @h{qw(simple 5)};
print "interp-02 slice: [@h{qw(simple 5)}] same as [@slice]\n";

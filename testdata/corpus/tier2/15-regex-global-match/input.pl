#!/usr/bin/perl
use strict;
use warnings;

# The /g flag in both contexts, iterating with while(//g) and inspecting
# pos(), plus resetting the match position.

my $text = 'key1=alpha key2=beta key3=gamma key4=delta';

# /g in list context: all matches at once.
my @keys = $text =~ /(\w+)=/g;
print "keys: @keys\n";

# Multiple captures per match flatten into one list.
my @pairs = $text =~ /(\w+)=(\w+)/g;
print "pairs (flat): ", scalar @pairs, " items\n";
my %map = @pairs;
print join(' ', map { "$_->$map{$_}" } sort keys %map), "\n";

# /g in scalar context: iterate, one match at a time, tracking pos().
my $iterations = 0;
while ($text =~ /(\w+)=(\w+)/g) {
    my ($k, $v) = ($1, $2);
    printf "%-5s %-6s pos=%d\n", $k, $v, pos($text);
    $iterations++;
}
print "iterations=$iterations pos after loop=", (defined pos($text) ? pos($text) : 'undef'), "\n";

# Counting matches with the countof idiom.
my $vowel_runs = () = $text =~ /[aeiou]+/g;
print "vowel runs: $vowel_runs\n";

# Manually setting pos() to restart part-way through.
pos($text) = index($text, 'key3');
if ($text =~ /\G(\w+)=(\w+)/g) {
    print "resumed at key3: $1 -> $2\n";
}

# \G anchored tokeniser: a tiny lexer over an arithmetic expression.
my $expr = '12 + 34*(5 - 6) / 7';
my @tokens;
pos($expr) = 0;
while (pos($expr) < length $expr) {
    if    ($expr =~ /\G\s+/gc)        { next }
    elsif ($expr =~ /\G(\d+)/gc)      { push @tokens, "NUM($1)" }
    elsif ($expr =~ /\G([-+*\/])/gc)  { push @tokens, "OP($1)" }
    elsif ($expr =~ /\G([()])/gc)     { push @tokens, "PAREN($1)" }
    else                              { push @tokens, 'ERR'; last }
}
print "tokens: ", join(' ', @tokens), "\n";

# /g over many strings inside a loop, where each string has its own pos.
my @configs = ('a=1;b=2', 'x=9', 'p=1;q=2;r=3');
for my $cfg (@configs) {
    my $n = 0;
    $n++ while $cfg =~ /(\w)=(\d)/g;
    print "'$cfg' has $n settings\n";
}

# A match that fails resets pos() unless /c is used.
my $s = 'aaa bbb';
$s =~ /(a+)/g;
print "pos after success: ", pos($s), "\n";
$s =~ /zzz/g;
print "pos after failed /g: ", (defined pos($s) ? pos($s) : 'undef'), "\n";
$s =~ /(a+)/g;
$s =~ /zzz/gc;
print "pos after failed /gc: ", (defined pos($s) ? pos($s) : 'undef'), "\n";

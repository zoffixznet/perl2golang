#!/usr/bin/perl
use strict;
use warnings;

# What these operators and builtins hand back, which is often not what a first
# reading suggests.

print "--- && and || yield an operand ---\n";
my $name = 'ada';
my $empty = '';
print "'ada' && 'lovelace' = '", ( $name && 'lovelace' ), "'\n";
print "''    && 'lovelace' = '", ( $empty && 'lovelace' ), "'\n";
print "''    || 'anon'     = '", ( $empty || 'anon' ), "'\n";
print "0     || 'anon'     = '", ( 0 || 'anon' ), "'\n";
my $picked = $empty || $name || 'fallback';
print "first true of three: $picked\n";

print "--- push and unshift yield the new length ---\n";
my @queue = ( 1, 2, 3 );
my $after_push = push @queue, 4, 5;
print "push returned $after_push, queue is @queue\n";
my $after_unshift = unshift @queue, 0;
print "unshift returned $after_unshift, queue is @queue\n";

print "--- chop and chomp ---\n";
my $word = 'hello';
my $gone = chop $word;
print "chop removed '$gone' leaving '$word'\n";
chomp( my $line = "text\n" );
print "chomp inside a declaration: [$line]\n";
my $removed = chomp( my $copy = 'no newline' );
print "chomp removed $removed character(s) from [$copy]\n";

print "--- s/// yields a count, s///r yields the copy ---\n";
my $log = 'ERROR ERROR WARN ERROR';
my $hits = ( $log =~ s/ERROR/FATAL/g );
print "replaced $hits, now: $log\n";
my $once = 'a-b-c';
my $first = ( $once =~ s/-/+/ );
print "without /g replaced $first, now: $once\n";
my $orig = '  padded  ';
my $trimmed = $orig =~ s/^\s+|\s+$//gr;
print "orig=[$orig] trimmed=[$trimmed]\n";
my $miss = ( $orig =~ s/zzz/yyy/ );
print "no match yields '", ( $miss ? $miss : '' ), "'\n";

#!/usr/bin/perl
# TRAP: backreferences, lookahead, lookbehind, atomic groups -- all legal
# Perl, all inexpressible in RE2 (Go's regexp package).
use strict;
use warnings;

my $s = "abab cdcd effe";
my @doubled = $s =~ /\b(\w+)\1\b/g;    # backreference \1
print "doubled: @doubled\n";

my $pw = "hunter2X";
print "strong: ", ( $pw =~ /^(?=.*\d)(?=.*[A-Z]).{8,}$/ ? "yes" : "no" ), "\n";  # lookahead

my $text = "price: 42 USD, id: 99";
my ($price) = $text =~ /(?<=price: )(\d+)/;    # lookbehind
print "price: $price\n";

my $html = "<b>bold</b> and <i>italic</i>";
my @tags = $html =~ /<(\w+)>.*?<\/\1>/g;     # backref matches closing tag
print "tags: @tags\n";

my ($inner) = 'say "hi there" now' =~ /"((?>[^"]*))"/;    # atomic group
print "quoted: $inner\n";

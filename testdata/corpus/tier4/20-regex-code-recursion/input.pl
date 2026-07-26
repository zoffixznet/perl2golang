#!/usr/bin/perl
# TRAP: recursive subpatterns (?&name) and embedded code (?{...}) turn
# the regex engine into a full programming language.
use strict;
use warnings;

my $balanced = qr/^ (?<grp> \( (?: [^()]+ | (?&grp) )* \) ) $/x;
for my $t ( '(a(b)c)', '(a(b)c', '((()))' ) {
    print "$t: ", ( $t =~ $balanced ? "balanced" : "NOT balanced" ), "\n";
}

my $count = 0;
"aaa" =~ /a(?{ $count++ })a/;    # code runs DURING the match
print "code block ran $count time(s)\n";

my $sum = 0;
"10 20 30" =~ /\b(\d+)\b(?{ $sum += $1 })(*FAIL)/;   # match-all-and-fail trick
print "sum harvested by regex: $sum\n";

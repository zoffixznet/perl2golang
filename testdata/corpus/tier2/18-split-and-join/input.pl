#!/usr/bin/perl
use strict;
use warnings;

# Everything split does, which is more than it looks like: regex
# separators, capturing separators, the ' ' special case, the empty
# pattern, limits and trailing-empty-field behaviour.

my $record = "ada:x:1000:1000:Ada Lovelace,,,:/home/ada:/bin/bash";
my @fields = split /:/, $record;
printf "passwd fields: %d, user=%s shell=%s\n", scalar @fields, $fields[0], $fields[-1];

# Trailing empty fields are dropped unless a negative limit is given.
my $trailing = 'a,b,,c,,,';
my @dropped = split /,/, $trailing;
my @kept    = split /,/, $trailing, -1;
printf "dropped=%d kept=%d\n", scalar @dropped, scalar @kept;
print "kept: [", join('][', @kept), "]\n";

# A positive limit stops splitting and leaves the remainder intact.
my $kv = 'path=/var/log/nginx=rotated';
my ($key, $value) = split /=/, $kv, 2;
print "key=$key value=$value\n";

my @three = split /:/, $record, 3;
print "limit 3 last piece: $three[2]\n";

# split ' ' is magic: leading whitespace is stripped and runs collapse.
my $padded = "   alpha   beta\tgamma \n delta  ";
my @magic  = split ' ', $padded;
my @regex  = split / /, $padded;
printf "magic=%d regex=%d\n", scalar @magic, scalar @regex;
print "magic words: [", join('][', @magic), "]\n";

# split /\s+/ differs from split ' ' when the string starts with space.
my @ws = split /\s+/, $padded;
printf "split /\\s+/ gives %d fields, first is [%s]\n", scalar @ws, $ws[0];

# Capturing groups in the pattern put the separators into the output.
my $expr = '12+34-5*6';
my @with_ops = split /([-+*\/])/, $expr;
print "tokens: ", join(' ', @with_ops), "\n";
my @operands = grep { /^\d+$/ } @with_ops;
my @operators = grep { !/^\d+$/ } @with_ops;
print "operands=@operands operators=@operators\n";

# The empty pattern splits into characters.
my @chars = split //, 'perl';
print "chars: ", join('.', @chars), " (", scalar @chars, ")\n";
my @first_two = split //, 'perl', 3;
print "limited chars: [", join('][', @first_two), "]\n";

# Splitting on a multi-character and an alternation pattern.
my $csvish = 'one;;two;;three';
print "double-semicolon: ", join('|', split /;;/, $csvish), "\n";
my $mixed_sep = 'a1b2c3d';
print "alternation: ", join('|', split /[0-9]/, $mixed_sep), "\n";

# join is the inverse, and nests happily.
my %row = (name => 'ada', role => 'analyst', year => 1815);
my $encoded = join ';', map { "$_=$row{$_}" } sort keys %row;
print "encoded: $encoded\n";
my %decoded = map { split /=/, $_, 2 } split /;/, $encoded;
print "round trip ok: ", (join(',', map { $decoded{$_} } sort keys %decoded) eq
                          join(',', map { $row{$_} } sort keys %row) ? 'yes' : 'no'), "\n";

# split on a string that never matches returns the whole thing.
my @nomatch = split /ZZZ/, 'unchanged';
print "no separator: ", scalar @nomatch, " field(s): $nomatch[0]\n";

# Splitting an empty string yields an empty list.
my @empty = split /,/, '';
print "empty input -> ", scalar @empty, " fields\n";

# Chained: parse a query string.
my $qs = 'q=perl+regex&page=2&safe=&sort=date';
my @kvs = split /&/, $qs;
for my $pair (@kvs) {
    my ($k, $v) = split /=/, $pair, 2;
    $v = '' unless defined $v;
    $v =~ tr/+/ /;
    printf "%-5s => [%s]\n", $k, $v;
}

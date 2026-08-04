#!/usr/bin/perl
use strict;
use warnings;

# A match read on the right of a list assignment hands back its capture
# groups. The single-group case is the one that reads like a boolean and is
# not one.

my @lines = (
    'report.csv       4821 shipped',
    'notes.md          317 draft',
    'archive/old.csv  9002 shipped',
    'no-extension       12 draft',
);

print "--- one group ---\n";
my %by_ext;
for my $line (@lines) {
    my ($ext) = $line =~ /\.(\w+)\s/;
    $ext = '(none)' unless defined $ext && length $ext;
    push @{ $by_ext{$ext} }, (split ' ', $line)[0];
}
for my $ext (sort keys %by_ext) {
    printf "%-8s %d: %s\n", $ext, scalar @{ $by_ext{$ext} }, join(' ', @{ $by_ext{$ext} });
}

print "--- several groups ---\n";
for my $line (@lines) {
    my ($name, $size, $state) = $line =~ /^(\S+)\s+(\d+)\s+(\w+)$/;
    printf "%-16s %6d %s\n", $name, $size, $state;
}

print "--- a failed match yields nothing ---\n";
my ($missing) = 'plain text' =~ /\((\w+)\)/;
print "missing is ", (defined $missing ? "'$missing'" : 'undef'), "\n";
my ($a1, $a2) = 'plain text' =~ /\((\w+)\)(\d)/;
print "both undef: ", (defined $a1 ? 'no' : 'yes'), (defined $a2 ? ' no' : ' yes'), "\n";

print "--- captures into an array ---\n";
my @pair = 'width=1024' =~ /^(\w+)=(\d+)$/;
print "pair holds ", scalar @pair, ": @pair\n";
my @every = 'a1 b22 c333' =~ /([a-z])(\d+)/g;
print "every holds ", scalar @every, ": @every\n";

print "--- no groups at all ---\n";
my ($truth) = 'hello' =~ /ell/;
print "a match with no groups yields $truth\n";

print "--- the same match as a condition ---\n";
for my $line (@lines) {
    if (my ($state) = $line =~ /\s(\w+)$/) {
        print "state=$state\n" if $state eq 'draft';
    }
}

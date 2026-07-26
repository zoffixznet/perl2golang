#!/usr/bin/perl
use strict;
use warnings;

# Builds a directory tree out of flat path strings. The whole tree is grown
# by walking a reference downwards and letting each missing level
# autovivify -- there is no explicit "create this node" step anywhere.

my %tree;
my %size;

open my $fh, '<', 'files/paths.txt' or die "open: $!\n";
while (my $line = <$fh>) {
    chomp $line;
    next unless length $line;
    my ($path, $bytes) = split ' ', $line;
    my @parts = split m{/}, $path;
    my $leaf  = pop @parts;

    my $node = \%tree;
    for my $part (@parts) {
        $node = \%{ $node->{$part} };     # autovivifies $node->{$part} as a hashref
    }
    $node->{$leaf} = undef;               # undef marks a file
    $size{$path} = $bytes;
}
close $fh or die "close: $!\n";

sub render {
    my ($node, $prefix) = @_;
    return unless ref $node;
    for my $name (sort keys %$node) {
        my $child = $node->{$name};
        if (defined $child) {
            print "$prefix$name/\n";
            render($child, "$prefix  ");
        }
        else {
            print "$prefix$name\n";
        }
    }
    return;
}

print "-- tree --\n";
render(\%tree, '');

# Aggregate sizes back up the tree, again relying on autovivification of
# the accumulator hash.
my %dir_total;
for my $path (keys %size) {
    my @parts = split m{/}, $path;
    pop @parts;
    my $so_far = '';
    for my $part (@parts) {
        $so_far = length $so_far ? "$so_far/$part" : $part;
        $dir_total{$so_far} += $size{$path};
    }
    $dir_total{'(root)'} += $size{$path};
}

print "-- totals --\n";
for my $dir (sort keys %dir_total) {
    printf "%-20s %8d\n", $dir, $dir_total{$dir};
}

print "-- depths --\n";
my %depth_count;
$depth_count{ scalar(() = $_ =~ m{/}g) }++ for sort keys %size;
printf "depth %d: %d files\n", $_, $depth_count{$_} for sort { $a <=> $b } keys %depth_count;

print "top level dirs: ", join(' ', sort keys %tree), "\n";

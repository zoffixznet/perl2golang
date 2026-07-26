#!/usr/bin/perl
use strict;
use warnings;

# Directory walking with opendir/readdir plus the -e/-f/-d/-s/-r file
# tests. Everything is reported relative to the script's working
# directory so the output does not depend on where the tree lives.

my $root = 'files';

die "$0: $root is not a directory\n" unless -d $root;

opendir my $dh, $root or die "cannot opendir $root: $!\n";
my @entries = sort grep { $_ ne '.' && $_ ne '..' } readdir $dh;
closedir $dh or die "closedir: $!\n";

printf "%-12s %-5s %8s %s\n", 'NAME', 'TYPE', 'SIZE', 'NOTE';
for my $name (@entries) {
    my $p = "$root/$name";
    my $type = -d $p ? 'dir' : -f $p ? 'file' : 'other';
    my $size = -d $p ? 0 : -s $p;
    $size = 0 unless defined $size;
    my $note = -d $p            ? 'directory'
             : (-s $p ? 0 : 1)  ? 'empty file'
             :                    'has content';
    printf "%-12s %-5s %8d %s\n", $name, $type, $size, $note;
}

# Explicit tests on known paths, including ones that do not exist.
for my $p ("$root/report.csv", "$root/archive", "$root/empty.log", "$root/missing.txt") {
    printf "%-20s exists=%s file=%s dir=%s nonempty=%s readable=%s\n",
        $p,
        (-e $p ? 'y' : 'n'),
        (-f $p ? 'y' : 'n'),
        (-d $p ? 'y' : 'n'),
        (-s $p ? 'y' : 'n'),
        (-r $p ? 'y' : 'n');
}

# Recursive descent, collecting files by extension.
my %by_ext;
walk($root);

sub walk {
    my ($dir) = @_;
    opendir my $d, $dir or die "opendir $dir: $!\n";
    my @kids = sort grep { !/^\.\.?$/ } readdir $d;
    closedir $d or die "closedir $dir: $!\n";
    for my $kid (@kids) {
        my $path = "$dir/$kid";
        if (-d $path) {
            walk($path);
        }
        elsif (-f $path) {
            my ($ext) = $kid =~ /\.(\w+)$/;
            $ext = '(none)' unless defined $ext;
            push @{ $by_ext{$ext} }, $path;
        }
    }
    return;
}

for my $ext (sort keys %by_ext) {
    printf "%-8s %d: %s\n", $ext, scalar @{ $by_ext{$ext} }, join(' ', @{ $by_ext{$ext} });
}

# The stacked-test shorthand and the _ filehandle reuse.
my $target = "$root/report.csv";
if (-e $target && -f _ && -s _) {
    printf "%s is a non-empty regular file of %d bytes\n", $target, -s _;
}

# Total bytes under the tree.
my $total = 0;
for my $ext (keys %by_ext) {
    $total += -s $_ for @{ $by_ext{$ext} };
}
print "total bytes: $total\n";

#!/usr/bin/perl
use strict;
use warnings;
use File::Temp qw(tempdir tempfile);
use File::Path qw(make_path remove_tree);
use File::Spec;

# Building a scratch tree and taking it down again. Every path here is under a
# temporary directory whose name is different on every run, so nothing prints
# a path: what prints is how many things were made, whether they are there,
# and what is left when the tree comes down.

my $root = tempdir(CLEANUP => 1);
printf "root is a directory: %s\n", (-d $root ? 'yes' : 'no');

my $deep = File::Spec->catdir($root, 'build', 'obj', 'debug');
my @made = make_path($deep);
printf "directories created: %d\n", scalar @made;
printf "the deepest one exists: %s\n", (-d $deep ? 'yes' : 'no');

# Making the same tree twice creates nothing the second time.
my @again = make_path($deep);
printf "created on a second pass: %d\n", scalar @again;

for my $name (qw(one.o two.o three.o)) {
    my $path = File::Spec->catfile($deep, $name);
    open my $fh, '>', $path or die "cannot write $name: $!";
    print {$fh} "object\n";
    close $fh;
}

opendir my $dh, $deep or die "cannot read the build directory: $!";
my @objects = sort grep { !/^\.\.?$/ } readdir $dh;
closedir $dh;
printf "objects: %s\n", join(' ', @objects);

my ($fh, $temp) = tempfile(DIR => $root, SUFFIX => '.log');
print {$fh} "line one\nline two\n";
close $fh;
printf "the temporary file has a .log suffix: %s\n",
    ($temp =~ /\.log$/ ? 'yes' : 'no');
printf "and it is inside the root: %s\n",
    (index($temp, $root) == 0 ? 'yes' : 'no');

my $removed = remove_tree(File::Spec->catdir($root, 'build'));
printf "entries removed: %d\n", $removed;
printf "the build tree is gone: %s\n", (-d $deep ? 'no' : 'yes');
printf "the root survives: %s\n", (-d $root ? 'yes' : 'no');

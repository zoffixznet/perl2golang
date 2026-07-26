#!/usr/bin/perl
# CASE angle-09: `<...>` whose contents are NOT a bare identifier or simple scalar
# is a FILE GLOB, not readline. `<*.txt>` calls glob("*.txt").
use strict; use warnings;
use File::Temp qw(tempdir);

my $dir = tempdir(CLEANUP => 0);
for my $n (qw(a.txt b.txt c.dat)) {
  open my $fh, '>', "$dir/$n" or die; print $fh "x"; close $fh;
}

my @g = <$dir/*.txt>;                    # GLOB: contents are not a simple scalar
my @f = glob("$dir/*.txt");
print "angle-09 glob-count: ", scalar(@g), " matches-glob-fn: ",
      (join(",",@g) eq join(",",@f) ? "yes" : "no"), "\n";

# Contrast: `<$fh>` where the contents ARE a simple scalar -> readline.
open my $fh, '<', \"line\n" or die;
my $l = <$fh>;
chomp $l;
print "angle-09 readline: $l\n";

# `<{a,b}.txt>` brace expansion inside a glob -- braces inside angles.
my @b = <$dir/{a,b}.txt>;
print "angle-09 brace-glob: ", scalar(@b), "\n";

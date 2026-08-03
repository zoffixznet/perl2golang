#!/usr/bin/perl
# What a script asks about a file once it has found it: mode, size, ownership,
# links and timestamps. Perl answers most of it with stat and the file test
# operators; Go answers it with os.Stat and questions about the result, and
# the two disagree about what is cheap and what is possible.
use strict;
use warnings;
use File::Temp qw(tempdir);
use File::Spec;

my $root = tempdir( CLEANUP => 1 );
my $file = File::Spec->catfile( $root, 'sample.txt' );
open my $fh, '>', $file or die "cannot write: $!";
print {$fh} "one\ntwo\nthree\n";
close $fh;

# The mode, set and then read back.
chmod 0640, $file;
my @st = stat $file;
printf "stat gives %d fields\n", scalar @st;
printf "mode: %04o\n", $st[2] & 07777;
printf "size: %d and -s agrees: %s\n", $st[7], ( $st[7] == -s $file ? 'yes' : 'no' );
printf "links: %d\n", $st[3];
printf "is a plain file: %s\n", ( -f $file ? 'yes' : 'no' );
printf "readable, writable, executable: %s %s %s\n",
    ( -r $file ? 'yes' : 'no' ), ( -w $file ? 'yes' : 'no' ), ( -x $file ? 'yes' : 'no' );

chmod 0444, $file;
printf "after chmod 0444, writable: %s\n", ( -w $file ? 'yes' : 'no' );
chmod 0644, $file;

# A symbolic link, and the difference between following it and not.
my $link = File::Spec->catfile( $root, 'sample.link' );
symlink $file, $link or die "cannot link: $!";
printf "link is a link: %s\n", ( -l $link ? 'yes' : 'no' );
printf "link points at a file: %s\n", ( -f $link ? 'yes' : 'no' );
printf "readlink ends in sample.txt: %s\n",
    ( readlink($link) =~ /sample\.txt$/ ? 'yes' : 'no' );

# Timestamps, set to fixed values so the transcript does not move.
utime 1_700_000_000, 1_700_000_100, $file;
my ( $atime, $mtime ) = ( stat $file )[ 8, 9 ];
printf "atime: %d mtime: %d\n", $atime, $mtime;
printf "mtime is later than atime: %s\n", ( $mtime > $atime ? 'yes' : 'no' );

# Renaming and removing, which report success rather than raising.
my $moved = File::Spec->catfile( $root, 'renamed.txt' );
printf "rename worked: %s\n", ( rename( $file, $moved ) ? 'yes' : 'no' );
printf "old name gone: %s\n", ( -e $file ? 'no' : 'yes' );
printf "unlink removed %d\n", unlink($moved);
printf "unlink of a missing file removed %d\n", unlink($moved);

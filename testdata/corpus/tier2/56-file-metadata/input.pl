#!/usr/bin/perl
# Asking a file about itself, and changing what the answers are: mode, size,
# times, links and names. Everything is under a temporary directory, and the
# only numbers printed are ones the script set itself.
use strict;
use warnings;
use File::Temp qw(tempdir);
use File::Spec;
use Time::Local qw(timegm);

my $root = tempdir( CLEANUP => 1 );
my $file = File::Spec->catfile( $root, 'sample.txt' );
open my $fh, '>', $file or die "cannot write: $!";
print {$fh} "one\ntwo\nthree\n";
close $fh;

print "-- the status list --\n";
my @st = stat $file;
printf "fields: %d\n", scalar @st;
printf "size:   %d\n", $st[7];
printf "size agrees with -s: %s\n", ( $st[7] == -s $file ? 'yes' : 'no' );
printf "links:  %d\n", $st[3];
printf "is a plain file: %s\n", ( -f $file ? 'yes' : 'no' );

print "-- permissions --\n";
chmod 0640, $file;
printf "mode: %04o\n", ( stat $file )[2] & 07777;
chmod 0444, $file;
printf "after 0444, writable: %s\n", ( -w $file ? 'yes' : 'no' );
chmod 0644, $file;
printf "after 0644, writable: %s\n", ( -w $file ? 'yes' : 'no' );

print "-- times set to fixed values --\n";
my $when = timegm( 0, 0, 12, 25, 11, 2023 );
utime $when - 100, $when, $file;
my ( $atime, $mtime ) = ( stat $file )[ 8, 9 ];
printf "mtime: %d\n", $mtime;
printf "mtime is later than atime by %d\n", $mtime - $atime;
printf "mtime round trips: %s\n", ( $mtime == $when ? 'yes' : 'no' );

print "-- links --\n";
my $link = File::Spec->catfile( $root, 'sample.link' );
symlink $file, $link or die "cannot link: $!";
printf "the link is a link:  %s\n", ( -l $link ? 'yes' : 'no' );
printf "the file is not:     %s\n", ( -l $file ? 'yes' : 'no' );
printf "it points at a file: %s\n", ( -f $link ? 'yes' : 'no' );
printf "readlink names it:   %s\n", ( readlink($link) =~ /sample\.txt$/ ? 'yes' : 'no' );

print "-- moving and removing --\n";
my $moved = File::Spec->catfile( $root, 'renamed.txt' );
printf "rename worked:   %s\n", ( rename( $file, $moved ) ? 'yes' : 'no' );
printf "the old name is gone: %s\n", ( -e $file ? 'no' : 'yes' );
printf "unlink removed:  %d\n", unlink($moved);
printf "and again:       %d\n", unlink($moved);
printf "the link is still there: %s\n", ( -l $link ? 'yes' : 'no' );

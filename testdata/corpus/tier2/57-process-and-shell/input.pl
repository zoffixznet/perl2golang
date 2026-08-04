#!/usr/bin/perl
# Running another program, which is the last big thing a sysadmin script does
# that does not convert yet. The only external command is the interpreter
# itself, so the transcript is the same on any box that can run this file.
use strict;
use warnings;

my $perl = $^X;

print "-- capturing output --\n";
my $out = `"$perl" -e 'print qq{alpha\nbeta\n}'`;
printf "captured %d bytes and %d lines\n", length $out, scalar( () = $out =~ /\n/g );
my @lines = `"$perl" -le 'print for 1 .. 3'`;
chomp @lines;
printf "as a list: %s\n", join( ',', @lines );

print "-- exit status --\n";
my $rc = system( $perl, '-e', 'exit 0' );
printf "success: rc=%d decoded=%d\n", $rc, $rc >> 8;
system( $perl, '-e', 'exit 42' );
printf "failure decoded=%d signal=%d\n", $? >> 8, $? & 127;

print "-- the list form takes no shell --\n";
my $tricky = 'a b; echo oops';
my $shown = `"$perl" -e 'print "\$ARGV[0]\n"' "$tricky"`;
chomp $shown;
printf "argument arrived whole: %s\n", ( $shown eq $tricky ? 'yes' : 'no' );

print "-- reading through a pipe --\n";
open my $rd, '-|', $perl, '-e', 'print "$_\n" for 1 .. 4'
    or die "pipe open: $!\n";
my $total = 0;
while ( my $line = <$rd> ) {
    chomp $line;
    $total += $line;
}
close $rd;
printf "sum through the pipe: %d\n", $total;

print "-- writing through a pipe --\n";
open my $wr, '|-', $perl, '-e', 'my $n = 0; $n += length while <STDIN>; print "child saw $n\n"'
    or die "write pipe: $!\n";
print {$wr} "abcd\n" for 1 .. 3;
close $wr;

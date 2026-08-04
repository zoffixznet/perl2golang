#!/usr/bin/perl
use strict;
use warnings;

# The parts of running a program that are about the process rather than about
# the command line.

print "-- the status a pipe close leaves --\n";
open my $ok, '-|', 'sh', '-c', 'printf "x\n"; exit 0' or die "open: $!\n";
my @got = <$ok>;
close $ok;
printf "read %d line(s), close status %d\n", scalar @got, $? >> 8;

open my $bad, '-|', 'sh', '-c', 'exit 7' or die "open: $!\n";
my @none = <$bad>;
close $bad;
printf "read %d line(s), close status %d\n", scalar @none, $? >> 8;

print "-- keeping the two streams apart --\n";
my $merged = `sh -c 'printf "out\n"; printf "err\n" >&2' 2>&1`;
$merged =~ s/\n/|/g;
print "merged: $merged\n";

my $only_out = `sh -c 'printf "out\n"; printf "err\n" >&2' 2>/dev/null`;
chomp $only_out;
print "stdout only: $only_out\n";

print "-- forking --\n";
my $pid = fork();
die "fork: $!\n" unless defined $pid;
if ( $pid == 0 ) {
    exit 3;
}
waitpid $pid, 0;
printf "child exited with %d\n", $? >> 8;

print "-- the interpreter that is running this --\n";
printf "have an interpreter path: %s\n", ( length $^X ? 'yes' : 'no' );

#!/usr/bin/perl
use strict;
use warnings;

# Running another program. Everything here is a POSIX shell builtin, so the
# transcript is the same anywhere this file can run at all.

print "-- exit status --\n";
my $rc = system( 'sh', '-c', 'exit 0' );
printf "ok:   rc=%d decoded=%d\n", $rc, $rc >> 8;
system( 'sh', '-c', 'exit 42' );
printf "fail: decoded=%d signal=%d\n", $? >> 8, $? & 127;

print "-- capturing output --\n";
my $out = `printf 'alpha\nbeta\n'`;
printf "captured %d bytes, %d line(s)\n", length $out, scalar( () = $out =~ /\n/g );
print "text: $out";

my @lines = `printf '1\n2\n3\n'`;
chomp @lines;
printf "as a list: %d item(s): %s\n", scalar @lines, join( ',', @lines );

my $total = 0;
$total += $_ for @lines;
print "sum: $total\n";

print "-- the list form takes no shell --\n";
my $tricky = 'a b; echo oops';
system( 'sh', '-c', 'printf "%s\n" "$1"', 'sh', $tricky );

print "-- reading through a pipe --\n";
open my $rd, '-|', 'sh', '-c', 'printf "10\n20\n30\n"'
    or die "pipe open: $!\n";
my $sum = 0;
while ( my $line = <$rd> ) {
    chomp $line;
    $sum += $line;
}
close $rd;
print "sum through the pipe: $sum\n";

print "-- writing through a pipe --\n";
open my $wr, '|-', 'sh', '-c', 'cat > /dev/null'
    or die "write pipe: $!\n";
print {$wr} "line $_\n" for 1 .. 3;
close $wr;
print "wrote 3 lines to the child\n";

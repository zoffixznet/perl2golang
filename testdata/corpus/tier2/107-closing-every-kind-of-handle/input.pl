#!/usr/bin/perl
use strict;
use warnings;

# Closing is not one operation wearing four hats. Closing a file flushes what
# was written to it; closing a pipe waits for the program on the other end and
# leaves its exit status in $?; and a handle held in a list is closed through
# whatever is holding it. A script that gets any of these wrong loses data or
# loses a status, and neither failure announces itself.

my $path = 'closing-one.txt';

open my $out, '>', $path or die "open $path: $!\n";
print {$out} "one\n";
print {$out} "two\n";
close $out or die "close $path: $!\n";
printf "wrote %d bytes\n", -s $path;

open my $in, '<', $path or die "open $path: $!\n";
my @lines = <$in>;
close $in;
printf "read %d line(s) back\n", scalar @lines;
unlink $path;

# The status a pipe close leaves behind is the whole reason a script bothers
# to close a pipe rather than letting it fall out of scope.
open my $good, '-|', 'sh', '-c', 'printf "a\nb\nc\n"' or die "pipe: $!\n";
my @piped = <$good>;
close $good;
printf "pipe gave %d line(s), status %d\n", scalar @piped, $? >> 8;

open my $bad, '-|', 'sh', '-c', 'exit 4' or die "pipe: $!\n";
my @empty = <$bad>;
close $bad;
printf "failing pipe gave %d line(s), status %d\n", scalar @empty, $? >> 8;

# Handles kept in a list, closed by walking it.
my @open_files;
for my $n ( 1 .. 3 ) {
    my $name = "closing-batch-$n.txt";
    open my $fh, '>', $name or die "open $name: $!\n";
    print {$fh} "batch file $n\n";
    push @open_files, $fh;
}
close $_ for @open_files;

my $total = 0;
for my $n ( 1 .. 3 ) {
    my $name = "closing-batch-$n.txt";
    $total += -s $name;
    unlink $name;
}
printf "three batch files held %d bytes\n", $total;

my $left = grep { -e $_ } $path, map {"closing-batch-$_.txt"} 1 .. 3;
printf "files left behind: %d\n", $left;

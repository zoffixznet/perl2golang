#!/usr/bin/perl
use strict;
use warnings;

# The magic ARGV handle: <> reads every file named on the command line in
# turn, with $ARGV holding the current name. Resetting $. per file is the
# usual idiom for "line N of file F" reporting.

@ARGV or die "usage: $0 FILE...\n";

print "program: $0\n";
print "inputs:  @ARGV\n";

my %per_file;
my %status_count;
my $total_bytes = 0;

while (my $line = <>) {
    chomp $line;
    next unless $line =~ /\S/;

    my ($method, $path, $status, $bytes) = split ' ', $line;

    $per_file{$ARGV}{lines}++;
    $per_file{$ARGV}{bytes} += $bytes;
    $status_count{$status}++;
    $total_bytes += $bytes;

    printf "%-18s %3d %-6s %-18s %3s %6d\n",
        $ARGV, $., $method, $path, $status, $bytes;

    # Restart line numbering at the end of each file.
    close ARGV if eof;
}

print "-" x 60, "\n";
for my $file (sort keys %per_file) {
    printf "%-18s lines=%d bytes=%d\n",
        $file, $per_file{$file}{lines}, $per_file{$file}{bytes};
}

print "statuses: ", join(' ', map { "$_=$status_count{$_}" } sort keys %status_count), "\n";
print "total bytes: $total_bytes\n";
print "\@ARGV is now empty: ", (@ARGV ? 'no' : 'yes'), "\n";

my $errors = 0;
$errors += $status_count{$_} for grep { $_ >= 400 } keys %status_count;
printf "error responses: %d (%.1f%%)\n",
    $errors, 100 * $errors / (eval { my $t = 0; $t += $_ for values %status_count; $t });

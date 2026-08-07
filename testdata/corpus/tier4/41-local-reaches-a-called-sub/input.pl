#!/usr/bin/perl
# TRAP: local is dynamic scoping. The sub below is defined far from the
# block that localises $/, yet it sees the localised value, because local
# governs the rest of the CALL, not the rest of the block's text. A
# conversion that folds $/ into the reads it can see gives this sub the
# default newline and reads the wrong records.
use strict;
use warnings;

sub read_all_records {
    my ($path) = @_;
    open my $fh, '<', $path or die "open $path: $!\n";
    my @recs = <$fh>;    # what a "record" is depends on the CALLER here
    close $fh;
    chomp @recs;
    return @recs;
}

my @lines = read_all_records('files/data.txt');
printf "as lines: %d record(s), first is %s\n", scalar @lines, $lines[0];

{
    local $/ = '|';
    my @fields = read_all_records('files/data.txt');
    printf "as fields: %d record(s), first is %s\n", scalar @fields, $fields[0];
}

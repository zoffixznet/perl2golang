#!/usr/bin/perl
use strict;
use warnings;

# Slurping: whole-file reads with a localised $/, paragraph mode, and
# reading a file into a single scalar for whole-document regex work.

my $path = 'files/notes.txt';

# Whole file into one scalar. local restores $/ when the block exits.
my $whole = do {
    open my $fh, '<', $path or die "open: $!\n";
    local $/;
    <$fh>;
};
printf "slurped %d bytes, %d newlines\n", length($whole), ($whole =~ tr/\n//);

# Whole-document regex now that the text is one string.
my @titles = $whole =~ /^(Release \S+)$/mg;
print "releases: @titles\n";

# Paragraph mode: $/ = "" splits on blank-line runs.
my @paras;
{
    open my $fh, '<', $path or die "open: $!\n";
    local $/ = '';
    while (my $para = <$fh>) {
        chomp $para;
        push @paras, $para;
    }
    close $fh or die "close: $!\n";
}
printf "paragraph mode gave %d chunks\n", scalar @paras;

for my $p (@paras) {
    my @lines = split /\n/, $p;
    my $title = shift @lines;
    printf "%-12s %d note(s): %s\n", $title, scalar @lines,
        join('; ', map { substr($_, 0, 20) } @lines);
}

# A custom record separator.
my $csv_ish = "alpha::beta::gamma::";
my @recs;
{
    open my $fh, '<', \$csv_ish or die "open scalar: $!\n";
    local $/ = '::';
    while (my $rec = <$fh>) {
        chomp $rec;
        push @recs, $rec if length $rec;
    }
    close $fh or die "close: $!\n";
}
print "custom \$/ records: ", join('|', @recs), "\n";

# $/ is restored outside the block, so line reads work normally again.
open my $fh, '<', $path or die "open: $!\n";
my $first = <$fh>;
close $fh or die "close: $!\n";
chomp $first;
print "line mode restored, first line: $first\n";

# Slurp into a list of lines with chomp applied in one go.
my @lines = do {
    open my $h, '<', $path or die "open: $!\n";
    my @l = <$h>;
    close $h or die "close: $!\n";
    chomp @l;
    @l;
};
printf "line list: %d entries, %d of them blank\n",
    scalar @lines, scalar grep { $_ eq '' } @lines;

# Whole-file substitution done on the slurped copy.
(my $bumped = $whole) =~ s/^Release (\d+)\.(\d+)$/"Release " . ($1 + 10) . ".$2"/mge;
my @bumped_titles = $bumped =~ /^(Release \S+)$/mg;
print "bumped: @bumped_titles\n";

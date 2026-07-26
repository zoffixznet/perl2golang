#!/usr/bin/perl
# log-tail -- incremental log follower with rotation detection.
#
# Cron runs this every minute; it remembers where it stopped in a state
# file and only alerts on lines it has not seen.  Rotation is detected by
# checksumming the first `siglen` bytes of the file: if the header bytes
# changed, this is a brand-new file and we start from 0.  (We used to
# compare inodes, but that broke on the NFS-mounted log share, 2016.)
#
# This corpus copy prints the would-be new state to stdout instead of
# rewriting the state file, so runs are side-effect free.
use strict;
use warnings;
use Digest::MD5 qw(md5_hex);

my ($state_file, $pattern) = @ARGV;
die "usage: $0 <state-file> [pattern]\n" unless defined $state_file;
$pattern = defined $pattern ? qr/$pattern/ : qr/\b(?:ERROR|FATAL)\b/;

# ---- load state (missing state file is fine: first run) ----
my %state = (offset => 0, sig => '', siglen => 64, runs => 0);
if (open my $sf, '<', $state_file) {
    while (<$sf>) {
        next if /^\s*#/;
        chomp;
        $state{$1} = $2 if /^(\w+)=(.*)$/;
    }
    close $sf;
}
my $log = $state{file}
    or die "state file has no file= entry and no default configured\n";

# ---- open log, decide where to start ----
open my $lf, '<', $log or die "open $log: $!\n";
binmode $lf;

my $size = -s $log;
read $lf, my $head, $state{siglen};
my $sig = md5_hex($head // '');

my ($start, $why);
if ($state{sig} and $sig ne $state{sig}) {
    ($start, $why) = (0, 'rotation detected (header changed)');
} elsif ($state{offset} > $size) {
    # truncation without rotation: copytruncate-style logrotate
    ($start, $why) = (0, sprintf('truncation detected (offset %d > size %d)',
        $state{offset}, $size));
} elsif ($state{offset} == $size) {
    ($start, $why) = ($state{offset}, 'no growth');
} else {
    ($start, $why) = ($state{offset}, "resuming at byte $state{offset}");
}

print "log-tail: $log ($size bytes): $why\n";

# ---- scan new region ----
seek $lf, $start, 0 or die "seek: $!\n";
my ($scanned, $matched, $consumed) = (0, 0, $start);
my @alerts;
while (my $line = <$lf>) {
    # a partial last line (no newline yet) is left for the next run --
    # writers append atomically per line but we may catch them mid-write
    last unless $line =~ /\n\z/;
    $consumed += length $line;
    $scanned++;
    chomp $line;
    if ($line =~ $pattern) {
        $matched++;
        push @alerts, $line;
    }
}
close $lf;

if (@alerts) {
    print "new matches ($matched of $scanned new lines):\n";
    print "  $_\n" for @alerts;
} else {
    print "no new matches in $scanned new lines\n";
}

# ---- emit updated state ----
print "--- new state ---\n";
my %next = (
    file   => $log,
    offset => $consumed,
    sig    => $sig,
    siglen => $state{siglen},
    runs   => $state{runs} + 1,
);
# fixed key order: this block gets diffed by the deploy pipeline
print "$_=$next{$_}\n" for qw(file offset sig siglen runs);
exit($matched ? 1 : 0);

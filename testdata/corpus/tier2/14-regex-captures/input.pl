#!/usr/bin/perl
use strict;
use warnings;

# Syslog scraper. Numbered captures, named captures, alternation, anchors,
# character classes and non-greedy quantifiers, all on realistic input.

my $re = qr/
    ^(?<mon>[A-Z][a-z]{2})\s+
     (?<day>\d{1,2})\s+
     (?<time>\d{2}:\d{2}:\d{2})\s+
     (?<host>\S+)\s+
     (?<prog>[\w\-\/]+)
     (?:\[(?<pid>\d+)\])?
     :\s+
     (?<msg>.*)$
/x;

open my $fh, '<', 'files/syslog.txt' or die "open: $!\n";
my @records;
my $skipped = 0;
while (my $line = <$fh>) {
    chomp $line;
    unless ($line =~ $re) {
        $skipped++;
        next;
    }
    push @records, { %+ };
}
close $fh or die "close: $!\n";

printf "parsed=%d skipped=%d\n", scalar @records, $skipped;
for my $r (@records) {
    printf "%s %-9s %-9s pid=%-5s %s\n",
        $r->{time}, $r->{host}, $r->{prog},
        (defined $r->{pid} ? $r->{pid} : '-'),
        substr($r->{msg}, 0, 30);
}

# Numbered captures with $1..$3 and alternation.
for my $r (@records) {
    if ($r->{msg} =~ /^(Accepted|Failed)\s+(\S+)\s+for\s+(?:invalid user\s+)?(\w+)\s+from\s+([\d.]+)/) {
        printf "auth %-8s method=%-9s user=%-5s ip=%s\n", $1, $2, $3, $4;
    }
}

# Non-greedy vs greedy on the same string.
my $tag = '<b>bold</b> and <i>italic</i>';
my ($lazy)   = $tag =~ /<(.+?)>/;
my ($greedy) = $tag =~ /<(.+)>/;
print "lazy=[$lazy]\n";
print "greedy=[$greedy]\n";

# Anchors and character classes.
my @candidates = qw(ada_lovelace 9lives x A-1 __init__ tab	sep);
for my $c (@candidates) {
    my $ident = $c =~ /^[A-Za-z_]\w*$/         ? 'identifier' : '-';
    my $digit = $c =~ /^\d/                    ? 'leading-digit' : '-';
    my $space = $c =~ /[[:space:]]/            ? 'has-space' : '-';
    printf "%-14s %-11s %-14s %s\n", $c, $ident, $digit, $space;
}

# A capture that may not participate, plus the //-defined-or guard.
for my $s ('duration: 1523.442 ms', 'duration: 12 ms', 'no duration here') {
    if ($s =~ /duration:\s+(\d+)(?:\.(\d+))?\s*ms/) {
        my $whole = $1;
        my $frac  = defined $2 ? $2 : '000';
        print "duration $whole.$frac\n";
    }
    else {
        print "no duration in '$s'\n";
    }
}

# Capture into a list directly.
my ($h, $m, $sec) = '22:17:00' =~ /^(\d+):(\d+):(\d+)$/;
printf "seconds since midnight: %d\n", $h * 3600 + $m * 60 + $sec;

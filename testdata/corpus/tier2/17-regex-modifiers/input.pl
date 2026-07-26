#!/usr/bin/perl
use strict;
use warnings;

# The modifier flags that change what a pattern means: /x for readable
# patterns, /i for case, /m for line anchors, /s for dot-matches-newline.

my $log = "2023-11-14 ERROR db timeout\n2023-11-15 warn disk low\n2023-11-16 Error retry failed\n";

# /m makes ^ and $ match at internal line boundaries.
my @starts = $log =~ /^(\d{4}-\d{2}-\d{2})/mg;
print "line starts: @starts\n";

my $ends = () = $log =~ /failed$/mg;
print "lines ending in 'failed': $ends\n";

# Without /m, ^ only matches at the very beginning.
my $nom = () = $log =~ /^\d{4}/g;
print "without /m: $nom\n";

# /i for case-insensitive matching.
my @errors = $log =~ /^\S+\s+(error)\b/gim;
print "error tokens (as written): @errors\n";
printf "case-insensitive error count: %d\n", scalar @errors;

# /s makes . cross newlines.
my $blob = "start\nmiddle\nend";
my ($without_s) = $blob =~ /start(.*)end/;
my ($with_s)    = $blob =~ /start(.*)end/s;
printf "without /s: %s\n", defined $without_s ? "matched [$without_s]" : 'no match';
printf "with /s:    %s\n", defined $with_s ? 'matched ' . length($with_s) . ' chars' : 'no match';

# /x: whitespace and # comments inside the pattern are ignored.
my $ipv4 = qr/
    ^
    (?: 25[0-5] | 2[0-4]\d | 1\d\d | [1-9]?\d )   # first octet
    (?:
        \.
        (?: 25[0-5] | 2[0-4]\d | 1\d\d | [1-9]?\d )
    ){3}                                          # three more octets
    $
/x;

for my $ip ('10.0.0.1', '255.255.255.255', '256.1.1.1', '1.2.3', '192.168.001.1') {
    printf "%-16s %s\n", $ip, ($ip =~ $ipv4 ? 'valid' : 'invalid');
}

# Combining flags, and a pattern stored in a variable then reused.
my $word_re = qr/\b(\w{5,})\b/;
my $prose = "The Quick Brown Fox\nJumps over the LAZY dog";
my @long = $prose =~ /$word_re/g;
print "long words: @long\n";

my @lines_with_caps = grep { /^[A-Z]/ } split /\n/, $prose;
print "lines starting uppercase: ", scalar @lines_with_caps, "\n";

# /x plus /i plus named captures in one pattern.
my $duration_re = qr/
    (?<value> \d+ (?: \. \d+ )? )   # number, optional fraction
    \s*
    (?<unit>  ms | s | m | h )      # unit
    \b
/xi;

for my $s ('took 1500ms', 'ran for 2.5S', 'waited 3 H', 'nothing here') {
    if ($s =~ $duration_re) {
        printf "%-14s value=%-6s unit=%s\n", $s, $+{value}, lc $+{unit};
    }
    else {
        printf "%-14s no duration\n", $s;
    }
}

# /a-style class behaviour: \d vs an explicit class, and \b at edges.
my $mixed = 'abc123def456';
my @nums = $mixed =~ /(\d+)/g;
my @alph = $mixed =~ /([a-z]+)/g;
print "nums=@nums alpha=@alph\n";

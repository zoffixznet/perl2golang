#!/usr/bin/perl
# urlnorm -- normalise a list of URLs and find cache-key duplicates.
#
# Feed it the raw URL column from the CDN logs; it prints the canonical
# form for each and then groups URLs that collapse to the same cache
# key.  Unparseable lines are reported, not fatal -- the log always
# contains garbage (browser prefetch bugs, port scanners, that one
# monitoring probe from 2021 nobody can turn off).
use strict;
use warnings;
use FindBin;
use lib $FindBin::Bin;
use TinyURL;

my $file = shift @ARGV or die "usage: $0 <urls.txt>\n";
open my $fh, '<', $file or die "open $file: $!\n";

my (%by_canon, @rejects, @lines);
my $n = 0;

while (my $raw = <$fh>) {
    chomp $raw;
    next unless $raw =~ /\S/;
    $n++;

    my $url = eval { TinyURL->parse($raw) };
    if (!$url) {
        my $err = $@;
        chomp $err;
        push @rejects, [$n, $err];
        printf "%2d REJECT  %s\n", $n, $raw;
        next;
    }

    my $canon = $url->normalize->canon;
    printf "%2d %-7s %s\n", $n, $url->scheme, $canon;
    push @{ $by_canon{$canon} }, $n;
}
close $fh;

# ---- duplicate report ----
my @dupes = sort grep { @{ $by_canon{$_} } > 1 } keys %by_canon;
print "\ncache-key duplicates: ", scalar @dupes, "\n";
for my $canon (@dupes) {
    print "  $canon\n    lines: @{ $by_canon{$canon} }\n";
}

print "\nrejected: ", scalar @rejects, "\n";
printf "  line %2d: %s\n", @$_ for @rejects;

printf "\n%d urls -> %d distinct cache keys\n", $n - @rejects, scalar keys %by_canon;
exit(@rejects ? 1 : 0);

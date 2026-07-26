#!/usr/bin/perl
# access-report -- combined-format access log summary.
#
# Reads any number of log files via <> (we pass the rotated file first so
# the timeline is contiguous).  Started life in 2014 as a 20-line awk
# replacement; the named-capture regex replaced a split-on-space horror
# after a user agent with an embedded quote broke everything.
use strict;
use warnings;

my $TOP = 8;

# Combined Log Format, tolerantly.  Bytes can be '-' (HEAD requests).
my $LINE_RE = qr{
    ^
    (?<ip>\S+) \s+
    (?<ident>\S+) \s+
    (?<user>\S+) \s+
    \[ (?<ts>[^\]]+) \] \s+
    " (?<method>[A-Z]+) \s+ (?<target>\S+) \s+ HTTP/(?<httpver>[\d.]+) " \s+
    (?<status>\d{3}) \s+
    (?<bytes>\d+|-) \s+
    " (?<referer>[^"]*) " \s+
    " (?<agent>.*) "        # trailing junk after UA has been seen; eat it
    \s* $
}x;

my (%agg, %status_count, %method_count, %hour_count);
my ($lines, $parsed, $bytes_total) = (0, 0, 0);
my @bad;

while (my $line = <>) {
    $lines++;
    chomp $line;
    unless ($line =~ $LINE_RE) {
        push @bad, [$ARGV, $., substr($line, 0, 40)];
        # $. does not reset between <> files unless you close ARGV;
        # historical decision: we keep the raw running count.
        next;
    }
    $parsed++;
    my ($status, $method, $target) = @+{qw(status method target)};
    my $bytes = $+{bytes} eq '-' ? 0 : $+{bytes};
    my $path  = normalize_path($target);

    # three-level rollup: path -> status -> metric
    $agg{$path}{$status}{hits}  += 1;
    $agg{$path}{$status}{bytes} += $bytes;

    $status_count{$status}++;
    $method_count{$method}++;
    $bytes_total += $bytes;

    if ($+{ts} =~ m{^\d+/\w+/\d+:(\d{2}):}) {
        $hour_count{$1}++;
    }
}

# ---------------- report ----------------
printf "parsed %d/%d lines, %s transferred\n\n", $parsed, $lines, commify($bytes_total);

print "by status:\n";
for my $st (sort keys %status_count) {
    printf "  %s %-12s %6d  %5.1f%%\n",
        $st, status_label($st), $status_count{$st},
        100 * $status_count{$st} / $parsed;
}

print "\nby method: ",
    join(', ', map { "$_=$method_count{$_}" } sort keys %method_count), "\n";

# flatten path/status rollup into per-path totals for the top table
my %path_totals;
for my $path (keys %agg) {
    for my $st (keys %{ $agg{$path} }) {
        $path_totals{$path}{hits}  += $agg{$path}{$st}{hits};
        $path_totals{$path}{bytes} += $agg{$path}{$st}{bytes};
        $path_totals{$path}{errors} += $agg{$path}{$st}{hits} if $st >= 400;
    }
}

print "\ntop paths:\n";
printf "  %-28s %5s %6s %10s  %s\n", 'PATH', 'HITS', 'ERR', 'BYTES', 'STATUSES';
my @paths = sort {
    $path_totals{$b}{hits} <=> $path_totals{$a}{hits}
        or $a cmp $b
} keys %path_totals;
splice @paths, $TOP if @paths > $TOP;
for my $path (@paths) {
    my $t = $path_totals{$path};
    my $statuses = join ',',
        map  { "$_:$agg{$path}{$_}{hits}" }
        sort keys %{ $agg{$path} };
    printf "  %-28s %5d %6d %10s  %s\n",
        $path, $t->{hits}, $t->{errors} || 0, commify($t->{bytes}), $statuses;
}

print "\nrequests by hour (UTC):\n";
for my $h (sort keys %hour_count) {
    printf "  %s:00 %-40s %d\n", $h, '#' x $hour_count{$h}, $hour_count{$h};
}

if (@bad) {
    print "\nskipped lines:\n";
    printf "  %s line %d: %s...\n", @$_ for @bad;
}

my $err = 0;
$err += $status_count{$_} || 0 for grep { $_ >= 500 } keys %status_count;
printf "\nserver error rate: %.2f%%\n", 100 * $err / $parsed;
exit($err ? 1 : 0);

# ---------------- helpers ----------------
sub normalize_path {
    my ($t) = @_;
    $t =~ s/\?.*//;                          # strip query string
    $t =~ s{^https?://[^/]+}{};              # absolute-form requests (proxies)
    $t = '/' if $t eq '';
    # collapse numeric ids so /products/9912 and /products/123 group together
    $t =~ s{/\d+(?=/|$)}{/:id}g;
    return $t;
}

sub status_label {
    my ($s) = @_;
    my %names = (
        200 => 'OK',        201 => 'Created',  204 => 'No Content',
        302 => 'Found',     304 => 'Not Mod',
        403 => 'Forbidden', 404 => 'Not Found',
        500 => 'Server Err',
    );
    return $names{$s} // ($s >= 500 ? '5xx' : $s >= 400 ? '4xx' : $s >= 300 ? '3xx' : '2xx');
}

sub commify {
    # classic perlfaq5 commify
    my $n = reverse shift;
    $n =~ s/(\d{3})(?=\d)/$1,/g;
    return scalar reverse $n;
}

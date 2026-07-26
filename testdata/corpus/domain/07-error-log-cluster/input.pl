#!/usr/bin/perl
# error-cluster -- group "similar" error log lines into signatures.
#
# The normaliser turns volatile bits (numbers, hex addrs, paths, quoted
# strings) into placeholders so that the same logical error clusters
# together no matter which order id or worker hit it.  Ordering of the
# substitutions matters and was tuned by trial and error -- see comments.
use strict;
use warnings;

my $file      = shift @ARGV or die "usage: $0 <error-log> [min-count]\n";
my $min_count = shift @ARGV // 1;

open my $fh, '<', $file or die "open $file: $!\n";

my %cluster;   # signature -> { count, level, first_line, last_line, example }
my %level_seen;

while (my $line = <$fh>) {
    chomp $line;
    my ($ts, $level, $msg) = $line =~ /^(\S+ \S+) \[(\w+)\] (.*)$/
        or next;
    $level_seen{$level}++;
    next if $level eq 'info';   # info is never worth clustering

    my $sig = normalize($msg);
    my $c = $cluster{$sig} ||= {
        count      => 0,
        level      => $level,
        first_line => $.,
        example    => $msg,
    };
    $c->{count}++;
    $c->{last_line} = $.;
    # a signature seen at both warn and error gets promoted to error
    $c->{level} = 'error' if $level eq 'error';
}
close $fh;

my @sigs = grep { $cluster{$_}{count} >= $min_count } keys %cluster;

# report: biggest first, ties broken by first appearance then signature
@sigs = sort {
    $cluster{$b}{count} <=> $cluster{$a}{count}
        or $cluster{$a}{first_line} <=> $cluster{$b}{first_line}
        or $a cmp $b
} @sigs;

my $total = 0;
$total += $cluster{$_}{count} for @sigs;

print "levels: ", join(' ', map { "$_=$level_seen{$_}" } sort keys %level_seen), "\n";
print "clusters: ", scalar @sigs, " (covering $total lines)\n\n";

for my $sig (@sigs) {
    my $c = $cluster{$sig};
    printf "%3dx [%-5s] %s\n", $c->{count}, $c->{level}, $sig;
    printf "     lines %d..%d, e.g.: %s\n", $c->{first_line}, $c->{last_line},
        elide($c->{example}, 68);
}
exit 0;

# --------------------------------------------------------------
sub normalize {
    my ($msg) = @_;

    # 1. quoted strings first, else the path rule below eats them
    $msg =~ s/'[^']*'/'…'/g;

    # 2. hex addresses before generic numbers (0x7f... would otherwise
    #    become 0xN with a dangling prefix)
    $msg =~ s/\b0x[0-9a-fA-F]+\b/HEX/g;

    # 3. absolute paths -- greedy segment match, keeps the mount point
    #    out of the signature entirely
    $msg =~ s{(?:/[\w.-]+){2,}}{PATH}g;

    # 4. sizes with units, THEN bare numbers
    $msg =~ s/\b\d+(?:\.\d+)?\s*(?:ms|s|MB|GB|KB|%)\b/NUM_UNIT/g;
    $msg =~ s/\b\d+\b/N/g;

    # 5. collapse host-style names like db-03 -> db-N (the \b\d\b rule
    #    above misses digits glued to hyphens)
    $msg =~ s/\b([a-z]+)-N\b/$1-N/g;   # normalise already-replaced ones
    $msg =~ s/\b([a-z]+)-\d+\b/$1-N/g;

    $msg =~ s/\s+/ /g;
    return $msg;
}

sub elide {
    my ($s, $max) = @_;
    return $s if length($s) <= $max;
    return substr($s, 0, $max - 3) . '...';
}

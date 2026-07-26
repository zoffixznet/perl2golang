#!/usr/bin/perl
# proc-table-summary -- summarise a `ps aux` snapshot per user.
#
# Fed from a fixture here; in production the snapshot comes over ssh from
# hosts we cannot install agents on.  The RSS column is occasionally
# corrupted by a kernel bug on the old 3.10 hosts, hence the defensive
# numeric checks (see OPS-2291).
use strict;
use warnings;

my $file  = shift @ARGV or die "usage: $0 <ps-aux-output> [top-n]\n";
my $top_n = shift @ARGV // 3;

open my $fh, '<', $file or die "open $file: $!\n";
my $hdr = <$fh>;
die "not ps aux output\n" unless defined $hdr and $hdr =~ /^USER\s+PID/;

my (%by_user, @zombies, @badlines, %state_count);
my $total = 0;

while (my $line = <$fh>) {
    chomp $line;
    next unless $line =~ /\S/;

    # USER PID %CPU %MEM VSZ RSS TTY STAT START TIME COMMAND...
    my @f = split ' ', $line, 11;
    if (@f < 11) { push @badlines, [$., 'short line']; next }
    my ($user, $pid, $cpu, $mem, $vsz, $rss, $tty, $stat, $start, $time, $cmd) = @f;

    unless ($pid =~ /^\d+$/ and $cpu =~ /^\d+(?:\.\d+)?$/
        and $mem =~ /^\d+(?:\.\d+)?$/ and $rss =~ /^\d+$/) {
        push @badlines, [$., 'non-numeric field'];
        next;
    }

    $total++;
    my $primary_state = substr $stat, 0, 1;
    $state_count{$primary_state}++;
    push @zombies, { pid => $pid, user => $user, cmd => short_cmd($cmd) }
        if $primary_state eq 'Z';

    # autovivification does the bookkeeping for first-seen users
    my $u = $by_user{$user} ||= { procs => 0, cpu => 0, mem => 0, rss_kb => 0, cmds => {} };
    $u->{procs}++;
    $u->{cpu}    += $cpu;
    $u->{mem}    += $mem;
    $u->{rss_kb} += $rss;
    $u->{cmds}{ short_cmd($cmd) }++;
}
close $fh;

# ---------------- report ----------------
printf "processes: %d (skipped %d unparseable)\n", $total, scalar @badlines;
print  "states:    ", join(' ', map { "$_=$state_count{$_}" } sort keys %state_count), "\n\n";

printf "%-10s %5s %7s %7s %9s  %s\n",
    'USER', 'PROCS', 'CPU%', 'MEM%', 'RSS', 'TOP COMMANDS';
for my $user (sort { $by_user{$b}{cpu} <=> $by_user{$a}{cpu} or $a cmp $b } keys %by_user) {
    my $u = $by_user{$user};
    my @top = sort { $u->{cmds}{$b} <=> $u->{cmds}{$a} or $a cmp $b } keys %{ $u->{cmds} };
    splice @top, $top_n if @top > $top_n;
    printf "%-10s %5d %7.1f %7.1f %9s  %s\n",
        $user, $u->{procs}, $u->{cpu}, $u->{mem}, human_kb($u->{rss_kb}),
        join(', ', map { "$_($u->{cmds}{$_})" } @top);
}

if (@zombies) {
    print "\nzombies (", scalar @zombies, "):\n";
    for my $z (sort { $a->{pid} <=> $b->{pid} } @zombies) {
        printf "  pid %-6d %-10s %s\n", $z->{pid}, $z->{user}, $z->{cmd};
    }
}
if (@badlines) {
    print "\nunparseable lines:\n";
    printf "  line %d: %s\n", @$_ for @badlines;
}

# memory hogs: anything over 5% MEM in a single process gets called out.
# (Threshold argued about at length in 2021; 5%% won because the DB hosts
# have 8G and postgres legitimately sits at ~4%%.)
my @hogs;
for my $user (sort keys %by_user) {
    my $u = $by_user{$user};
    push @hogs, [$user, $u->{mem}] if $u->{mem} > 5;
}
if (@hogs) {
    print "\nmemory hogs (>5% aggregate):\n";
    printf "  %-10s %.1f%%\n", @$_ for sort { $b->[1] <=> $a->[1] } @hogs;
}
exit(@zombies ? 1 : 0);

# ---------------- helpers ----------------
sub short_cmd {
    my ($cmd) = @_;
    # kernel threads keep their brackets; everything else is basename'd
    return $1 if $cmd =~ /^(\[[^\]]+\])/;
    my ($word) = split ' ', $cmd;
    # 'postgres: walwriter' style titles: keep the first two words
    if ($word =~ /:$/) {
        my @w = split ' ', $cmd;
        return join ' ', @w[0 .. ($#w < 1 ? $#w : 1)];
    }
    $word =~ s{.*/}{};
    $word =~ s/^-//;    # login shells show as -bash
    return $word;
}

sub human_kb {
    my ($kb) = @_;
    return $kb . 'K' if $kb < 1024;
    return sprintf '%.1fM', $kb / 1024 if $kb < 1024 * 1024;
    return sprintf '%.2fG', $kb / (1024 * 1024);
}

#!/usr/bin/perl
# Access-log analyzer: Getopt::Long CLI, regex parsing, and a report whose
# column widths are computed at runtime (sprintf '%*d').
use strict;
use warnings;
use Getopt::Long;
use List::Util qw(max sum0);

my %opt = ( top => 3, 'min-bytes' => 0 );
GetOptions( \%opt, 'top=i', 'min-bytes=i', 'errors-only!' )
  or die "bad options\n";
my $logfile = shift @ARGV // 'files/access.log';

# Threshold for "slow"-style flagging comes from the environment with a
# fixed default so runs are reproducible.
my $big = $ENV{LOG_BIG_BYTES} // 4096;

my $line_re = qr{
    ^(\S+)\s+\S+\s+(\S+)\s+   # ip, user ('-' when anonymous)
    \[([^\]]+)\]\s+           # timestamp
    "(\w+)\s+(\S+)[^"]*"\s+   # method, path
    (\d{3})\s+(\d+)$          # status, bytes
}x;

my ( %by_path, %by_status, %by_user );
my ( $malformed, $total_bytes ) = ( 0, 0 );
my @big_hits;

open my $fh, '<', $logfile or die "open $logfile: $!\n";
while (<$fh>) {
    chomp;
    my ( $ip, $user, $ts, $method, $path, $status, $bytes ) = /$line_re/
      or ++$malformed, next;
    next if $bytes < $opt{'min-bytes'};
    next if $opt{'errors-only'} && $status < 400;

    my $class =
        $status >= 500 ? 'server-error'
      : $status >= 400 ? 'client-error'
      : $status >= 300 ? 'redirect'
      :                  'ok';    # chained ternary ladder

    $by_path{$path}{count}++;
    $by_path{$path}{bytes} += $bytes;
    $by_status{$class}++;
    $by_user{ $user eq '-' ? '(anon)' : $user }++;
    $total_bytes += $bytes;
    push @big_hits, [ $path, $bytes ] if $bytes >= $big;
}
close $fh;

# ---- report ------------------------------------------------------------
my @paths = sort {
    $by_path{$b}{count} <=> $by_path{$a}{count}
      or $a cmp $b
} keys %by_path;
splice @paths, $opt{top} if @paths > $opt{top};

# Runtime-computed column widths, applied through %-*s / %*d.
my $pathw  = max( 4, map { length } @paths );
my $countw = max( 5, map { length $by_path{$_}{count} } @paths );

printf "TOP %d PATHS (of %d)\n", scalar @paths, scalar keys %by_path;
printf "%-*s %*s %10s\n", $pathw, 'PATH', $countw, 'COUNT', 'BYTES';
for my $p (@paths) {
    printf "%-*s %*d %10d\n", $pathw, $p, $countw, $by_path{$p}{count},
      $by_path{$p}{bytes};
}

print "\nSTATUS CLASSES\n";
printf "  %-12s %d\n", $_, $by_status{$_} for sort keys %by_status;

print "\nUSERS\n";
for my $u ( sort { $by_user{$b} <=> $by_user{$a} or $a cmp $b } keys %by_user )
{
    printf "  %-8s %d\n", $u, $by_user{$u};
}

print "\nBIG TRANSFERS (>= $big bytes)\n";
printf "  %s (%d)\n", @$_ for sort { $b->[1] <=> $a->[1] } @big_hits;

printf "\nmalformed lines: %d\n", $malformed;
printf "total bytes: %d (avg %.1f)\n", $total_bytes,
  $total_bytes / sum0( values %by_user );

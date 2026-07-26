#!/usr/bin/perl
# disk-usage-report -- parse `df -k` output and report against thresholds.
#
# Runs from cron on ~200 hosts; output is mailed, so formatting matters.
# The wrapped-line handling exists because device-mapper and NFS paths
# are longer than df's 20-column device field (bit us in 2017).
use strict;
use warnings;
use Getopt::Long;

my %opt = (warn => 80, crit => 90, 'skip-pseudo' => 1);
GetOptions(\%opt, 'warn=i', 'crit=i', 'skip-pseudo!', 'sort=s')
    or die "bad options\n";
$opt{sort} ||= 'pct';
die "warn must be < crit\n" if $opt{warn} >= $opt{crit};

my $df_file = shift @ARGV or die "usage: $0 [options] <df-output>\n";

# Pseudo filesystems we do not page about.  overlay added for the k8s hosts.
my %PSEUDO = map { $_ => 1 } qw(devtmpfs tmpfs overlay squashfs);

my @fs = parse_df($df_file);

# filter
@fs = grep { !$PSEUDO{ $_->{fstype} } } @fs if $opt{'skip-pseudo'};

# classify each filesystem
for my $f (@fs) {
    if    ($f->{pct} >= $opt{crit}) { $f->{level} = 'CRIT' }
    elsif ($f->{pct} >= $opt{warn}) { $f->{level} = 'WARN' }
    else                            { $f->{level} = 'ok'   }
}

# sort order: worst first, then by mount for stability
my %SORTERS = (
    pct   => sub { $b->{pct} <=> $a->{pct} or $a->{mount} cmp $b->{mount} },
    mount => sub { $a->{mount} cmp $b->{mount} },
    avail => sub { $a->{avail} <=> $b->{avail} or $a->{mount} cmp $b->{mount} },
);
my $sorter = $SORTERS{ $opt{sort} }
    or die "unknown sort '$opt{sort}' (want: " . join('|', sort keys %SORTERS) . ")\n";
@fs = sort $sorter @fs;

printf "%-4s %5s %10s %10s %10s  %s\n",
    'LVL', 'USE%', 'SIZE', 'USED', 'AVAIL', 'MOUNT';
my %tally;
for my $f (@fs) {
    $tally{ $f->{level} }++;
    printf "%-4s %4d%% %10s %10s %10s  %s\n",
        $f->{level}, $f->{pct},
        human_k($f->{size}), human_k($f->{used}), human_k($f->{avail}),
        $f->{mount};
}
my $total_used = 0;
$total_used += $_->{used} for @fs;
printf "\n%d filesystems, %s used total; crit=%d warn=%d ok=%d\n",
    scalar @fs, human_k($total_used),
    map { $tally{$_} || 0 } qw(CRIT WARN ok);

# Nagios-flavoured exit codes: 0 ok, 1 warning, 2 critical.
exit(($tally{CRIT} ? 2 : 0) || ($tally{WARN} ? 1 : 0));

# --------------------------------------------------------------------
sub parse_df {
    my ($path) = @_;
    open my $fh, '<', $path or die "open $path: $!\n";
    my $header = <$fh>;                      # discard, but sanity-check it
    die "does not look like df -k output\n"
        unless defined $header and $header =~ /1K-blocks/;

    my (@out, $pending_dev);
    while (my $line = <$fh>) {
        chomp $line;
        next unless length $line;

        # A line with only a device name means df wrapped: remember it and
        # glue the numbers from the following line onto it.
        if ($line =~ /^(\S+)\s*$/) {
            $pending_dev = $1;
            next;
        }

        my ($dev, @rest);
        if (defined $pending_dev) {
            $dev = $pending_dev;
            undef $pending_dev;
            $line =~ s/^\s+//;
            @rest = split ' ', $line;
        } else {
            ($dev, @rest) = split ' ', $line;
        }
        # mount point may contain spaces in theory; df doesn't quote, so we
        # take fields positionally and re-join the tail.
        my ($size, $used, $avail, $pcts) = splice @rest, 0, 4;
        my $mount = join ' ', @rest;
        (my $pct = $pcts) =~ s/%$//;

        unless ($pct =~ /^\d+$/ and $size =~ /^\d+$/) {
            warn_line("unparseable df line: $line");
            next;
        }
        push @out, {
            dev    => $dev,
            fstype => guess_fstype($dev),
            size   => $size,
            used   => $used,
            avail  => $avail,
            pct    => $pct,
            mount  => $mount,
        };
    }
    close $fh;
    return @out;
}

sub guess_fstype {
    my ($dev) = @_;
    return $dev            if $dev =~ /^(?:devtmpfs|tmpfs|overlay|squashfs)$/;
    return 'nfs'           if $dev =~ /^[\w.-]+:\//;      # host:/export
    return 'device-mapper' if $dev =~ m{^/dev/mapper/};
    return 'disk'          if $dev =~ m{^/dev/};
    return 'other';
}

sub human_k {
    # input is KiB (df -k); keep one decimal like `df -h` roughly does
    my ($k) = @_;
    return "$k" . 'K' if $k < 1024;
    my @units = qw(M G T);
    my $v = $k / 1024;
    my $u = shift @units;
    while ($v >= 1024 and @units) { $v /= 1024; $u = shift @units }
    my $s = sprintf '%.1f', $v;
    $s =~ s/\.0$//;
    return $s . $u;
}

sub warn_line {
    # We used to warn() to stderr but cron mails made that useless;
    # everything goes to stdout now so the report is one artifact.
    my ($msg) = @_;
    print "NOTE  $msg\n";
}

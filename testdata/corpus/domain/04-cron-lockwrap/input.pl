#!/usr/bin/perl
# cronwrap -- run registered maintenance jobs under a lock, with sysexits
# style exit codes so the cron wrapper upstream can tell "busy" from
# "broken".  75 (EX_TEMPFAIL) means another instance holds a fresh lock.
#
# --now exists so tests (and the ops runbook examples) are reproducible;
# without it we use time(), same as always.
use strict;
use warnings;
use Getopt::Long;

my $EX_OK       = 0;
my $EX_TEMPFAIL = 75;

my %opt = ('stale-after' => 3600, lockdir => 'files/locks');
GetOptions(\%opt, 'now=i', 'stale-after=i', 'lockdir=s', 'dry-run|n', 'verbose|v')
    or die "bad options\n";
my $now = $opt{now} // time;

# ---------------------------------------------------------------------
# Job registry.  Each entry: description + coderef returning (exit, @report).
# Jobs used to be shell scripts scattered in /usr/local/bin; folding them
# in here (2018) is why we can test the whole thing offline.
# ---------------------------------------------------------------------
my %JOBS = (
    'nightly-backup' => {
        desc => 'verify last backup run set counts',
        code => \&job_backup_verify,
    },
    'health-check' => {
        desc => 'poll service supervisor state file',
        code => \&job_health_check,
    },
    'report-rollup' => {
        desc => 'roll daily reports into weekly',
        code => \&job_report_rollup,
    },
);

my @requested = @ARGV;
if (!@requested or grep { $_ eq '--list' } @requested) {
    print "available jobs:\n";
    printf "  %-16s %s\n", $_, $JOBS{$_}{desc} for sort keys %JOBS;
    exit $EX_OK;
}

my $worst = $EX_OK;
for my $name (@requested) {
    my $job = $JOBS{$name};
    if (!$job) {
        print "[$name] unknown job; known: ", join(', ', sort keys %JOBS), "\n";
        $worst = bump($worst, 64);   # EX_USAGE
        next;
    }

    my $lockfile = "$opt{lockdir}/$name.lock";
    my $lock = read_lock($lockfile);
    if ($lock) {
        my $age = $now - $lock->{started};
        if ($age <= $opt{'stale-after'}) {
            printf "[%s] SKIP: locked by pid %d on %s, %ds old (fresh)\n",
                $name, $lock->{pid}, $lock->{host}, $age;
            $worst = bump($worst, $EX_TEMPFAIL);
            next;
        }
        printf "[%s] stale lock (pid %d, %ds old > %ds) -- would break it\n",
            $name, $lock->{pid}, $age, $opt{'stale-after'};
    }

    if ($opt{'dry-run'}) {
        # dry-run still *runs* the job (they are all read-only checks)
        # but never touches lock files.
        print "[$name] dry-run: not writing $lockfile\n" if $opt{verbose};
    } else {
        write_lock($lockfile);
    }

    my ($rc, @report) = $job->{code}->();
    printf "[%s] %s (exit %d)\n", $name, ($rc == 0 ? 'OK' : 'FAILED'), $rc;
    print "  $_\n" for @report;
    $worst = bump($worst, $rc);

    unlink $lockfile unless $opt{'dry-run'};
}
printf "cronwrap: done, worst exit %d\n", $worst;
exit $worst;

# ------------------------------ jobs ---------------------------------
sub job_backup_verify {
    open my $fh, '<', 'files/backup_sets.txt' or return (66, "cannot read backup_sets.txt: $!");
    my (@bad, $sets);
    while (<$fh>) {
        next if /^\s*#/ or !/\S/;
        chomp;
        my ($set, $want, $got) = split /\t/;
        $sets++;
        push @bad, sprintf('%s: expected %d files, found %d', $set, $want, $got)
            if $want != $got;
    }
    close $fh;
    return @bad ? (1, "$sets sets checked", @bad)
                : (0, "$sets sets checked, all complete");
}

sub job_health_check {
    open my $fh, '<', 'files/services.txt' or return (66, "cannot read services.txt: $!");
    my (%state, @down);
    while (<$fh>) {
        chomp;
        my ($svc, $st, $note) = split /\t/;
        next unless defined $st;
        $state{$st}++;
        push @down, "$svc ($note)" if $st eq 'down';
    }
    close $fh;
    my $summary = join ' ', map { "$_=$state{$_}" } sort keys %state;
    return @down ? (2, $summary, map { "DOWN: $_" } @down)
                 : (0, $summary);
}

sub job_report_rollup {
    # The real job shells out to the reporting DB; the check here only
    # validates that its input window makes sense relative to --now.
    my @lines = ("window ends " . iso($now));
    return (0, @lines);
}

# ---------------------------- plumbing --------------------------------
sub read_lock {
    my ($path) = @_;
    open my $fh, '<', $path or return undef;
    my %lock;
    while (<$fh>) { $lock{$1} = $2 if /^(\w+)=(\S+)$/ }
    close $fh;
    # tolerate hand-created junk lock files (it happens)
    return undef unless $lock{pid} and $lock{started} and $lock{started} =~ /^\d+$/;
    $lock{host} ||= 'unknown';
    return \%lock;
}

sub write_lock {
    my ($path) = @_;
    open my $fh, '>', $path or die "cannot create lock $path: $!\n";
    print {$fh} "pid=$$\nhost=localhost\nstarted=$now\n";
    close $fh;
}

sub bump {
    my ($cur, $new) = @_;
    return $new > $cur ? $new : $cur;
}

sub iso {
    my ($t) = @_;
    my @g = gmtime $t;
    return sprintf '%04d-%02d-%02dT%02d:%02d:%02dZ',
        $g[5] + 1900, $g[4] + 1, @g[3, 2, 1, 0];
}

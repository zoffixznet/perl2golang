#!/usr/bin/perl
# passwd-audit -- offline audit of a passwd-format dump.
#
# Part of the quarterly access review.  Reads a copied passwd file (we
# never audit the live one from this tool -- lesson learned) plus the
# /etc/shells equivalent, and emits findings graded CRIT/WARN/INFO.
use strict;
use warnings;

my ($passwd_file, $shells_file) = @ARGV;
die "usage: $0 <passwd-file> <shells-file>\n" unless $shells_file;

my $UID_HUMAN_MIN = 1000;    # site policy: humans start at 1000
my $UID_HUMAN_MAX = 60000;

# ---- load valid shells ----
my %valid_shell;
open my $sh, '<', $shells_file or die "open $shells_file: $!\n";
while (<$sh>) { chomp; $valid_shell{$_} = 1 if m{^/} }
close $sh;

# ---- parse users ----
my (@users, @findings);
open my $fh, '<', $passwd_file or die "open $passwd_file: $!\n";
while (my $line = <$fh>) {
    chomp $line;
    next unless length $line;
    my @f = split /:/, $line, -1;    # -1: keep trailing empty fields!
    if (@f != 7) {
        push @findings, ['CRIT', "(line $.)", 'malformed line: ' . abbrev($line)];
        next;
    }
    my ($name, $pw, $uid, $gid, $gecos, $home, $shell) = @f;
    push @users, {
        name  => $name,  pw   => $pw,   uid   => $uid, gid => $gid,
        gecos => $gecos, home => $home, shell => $shell, line => $.,
    };
}
close $fh;

# ---- audits ----
my (%by_uid, %by_name);
for my $u (@users) {
    push @{ $by_uid{ $u->{uid} } }, $u->{name};
    $by_name{ $u->{name} }++;
}

for my $u (@users) {
    my $who = $u->{name};

    # empty password field means "no password required" on old systems
    if ($u->{pw} eq '') {
        push @findings, ['CRIT', $who, 'empty password field'];
    } elsif ($u->{pw} ne 'x' and $u->{pw} ne '*') {
        push @findings, ['WARN', $who, 'password hash stored in passwd file'];
    }

    if ($u->{uid} eq '0' and $who ne 'root') {
        push @findings, ['CRIT', $who, 'UID 0 account that is not root'];
    }

    if (!$valid_shell{ $u->{shell} }) {
        # interactive-looking shells that are not whitelisted are worse
        # than obviously-custom ones; both get flagged.
        my $sev = $u->{shell} =~ m{/(?:bash|sh|zsh|ksh|csh)$} ? 'CRIT' : 'WARN';
        push @findings, [$sev, $who, "shell not in shells file: $u->{shell}"];
    }

    if ($u->{uid} =~ /^\d+$/) {
        if ($u->{uid} >= $UID_HUMAN_MIN and $u->{uid} <= $UID_HUMAN_MAX) {
            # humans should live under /home -- service accts are exempt
            # if they follow the svc_/legacy naming convention (grandfathered)
            if ($u->{home} !~ m{^/home/} and $who !~ /^(?:svc_|legacy)/) {
                push @findings, ['WARN', $who, "human-range UID but home is $u->{home}"];
            }
            if ($u->{name} ne lc $u->{name}) {
                push @findings, ['INFO', $who, 'mixed-case username'];
            }
        }
    } else {
        push @findings, ['CRIT', $who, "non-numeric UID '$u->{uid}'"];
    }
}

for my $uid (sort { numeric_or_string($a, $b) } keys %by_uid) {
    my @names = @{ $by_uid{$uid} };
    next unless @names > 1;
    next if $uid eq '0' and join(',', sort @names) eq 'root,toor'; # already CRIT'd above
    push @findings, ['WARN', join('+', sort @names), "shared UID $uid"];
}

# ---- report ----
my %sev_rank = (CRIT => 0, WARN => 1, INFO => 2);
my %count;
print "passwd-audit: ", scalar @users, " accounts loaded\n\n";
for my $f (sort {
        $sev_rank{ $a->[0] } <=> $sev_rank{ $b->[0] }
        or $a->[1] cmp $b->[1]
        or $a->[2] cmp $b->[2]
    } @findings)
{
    $count{ $f->[0] }++;
    printf "%-4s %-22s %s\n", @$f;
}
print "\n";
printf "%s: %d\n", $_, $count{$_} // 0 for qw(CRIT WARN INFO);

# stale human accounts summary (naming-convention heuristic, best effort)
my @stale = sort map { $_->{name} } grep {
    $_->{uid} =~ /^\d+$/ and $_->{uid} >= $UID_HUMAN_MIN
        and $_->{gecos} =~ /\b(?:intern|temp|contractor)\b/i
} @users;
print "review-for-removal: @stale\n" if @stale;

exit(($count{CRIT} // 0) ? 1 : 0);

# ---- helpers ----
sub abbrev {
    my ($s) = @_;
    return length $s > 30 ? substr($s, 0, 27) . '...' : $s;
}

# UIDs are strings that are usually numbers; sort numerically when we
# can, fall back to string compare for the garbage ones.
sub numeric_or_string {
    my ($x, $y) = @_;
    return $x <=> $y if $x =~ /^\d+$/ and $y =~ /^\d+$/;
    return $x cmp $y;
}

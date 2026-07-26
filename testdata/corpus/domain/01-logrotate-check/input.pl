#!/usr/bin/perl
# logrotate-check -- sanity-check a logrotate config against a log snapshot.
#
# Grew out of a one-liner after the March 2020 incident where a typo'd
# stanza silently stopped rotation for six months.  Now it parses the
# whole config, validates each stanza, and cross-checks against a
# collector snapshot of actual log sizes/mtimes.
use strict;
use warnings;

my %FREQ_SECONDS = (
    daily   => 86400,
    weekly  => 7 * 86400,
    monthly => 31 * 86400,   # pessimistic; we only warn, never page
    yearly  => 366 * 86400,
);

my %KNOWN_SIMPLE = map { $_ => 1 } qw(
    compress nocompress delaycompress missingok nomissingok
    notifempty ifempty sharedscripts copytruncate
);

my ($conf_path, $state_path, $now) = @ARGV;
die "usage: $0 <logrotate.conf> <state.txt> <now-epoch>\n"
    unless defined $now and $now =~ /^\d+$/;

# ---- pass 1: parse config into global defaults + stanzas ----------------
my (%global, @stanzas, @parse_errors);
{
    open my $fh, '<', $conf_path or die "open $conf_path: $!\n";
    my @lines = <$fh>;
    close $fh;
    chomp @lines;

    my $i = 0;
    while ($i < @lines) {
        my $line = strip($lines[$i]);
        if ($line eq '') { $i++; next }

        if ($line =~ /\{\s*$/) {
            # stanza header: one or more paths then '{'
            (my $paths = $line) =~ s/\s*\{\s*$//;
            my @paths = split ' ', $paths;
            my ($stanza, $consumed);
            # Each stanza is parsed under eval so one broken block does
            # not take out the whole report (that was the 2020 bug).
            eval {
                ($stanza, $consumed) = parse_stanza(\@lines, $i + 1);
                1;
            } or do {
                my $err = $@;
                chomp $err;
                push @parse_errors, sprintf('stanza at line %d (%s): %s',
                    $i + 1, $paths[0] // '?', $err);
                # skip forward to the closing brace so we can resume
                $consumed = 0;
                $consumed++ while $i + 1 + $consumed < @lines
                    and strip($lines[$i + 1 + $consumed]) ne '}';
                $consumed++;   # the brace itself
            };
            if ($stanza) {
                $stanza->{paths} = \@paths;
                $stanza->{line}  = $i + 1;
                push @stanzas, $stanza;
            }
            $i += 1 + $consumed;
        }
        else {
            my ($k, $v) = parse_directive($line);
            if (defined $k) { $global{$k} = $v }
            else { push @parse_errors, "global line " . ($i + 1) . ": unparseable '$line'" }
            $i++;
        }
    }
}

# ---- pass 2: validate stanzas -------------------------------------------
my @findings;
for my $st (@stanzas) {
    my $where = $st->{paths}[0];
    my @freqs = grep { exists $st->{directives}{$_} } sort keys %FREQ_SECONDS;
    if (@freqs > 1) {
        push @findings, ['ERROR', $where,
            'conflicting frequencies: ' . join(', ', @freqs)];
    }
    if (exists $st->{directives}{rotate}) {
        my $n = $st->{directives}{rotate};
        push @findings, ['ERROR', $where, "rotate count not numeric: '$n'"]
            unless $n =~ /^\d+$/;
    }
    for my $bad (sort keys %{ $st->{unknown} || {} }) {
        push @findings, ['WARN', $where, "unknown directive '$bad'"];
    }
}
push @findings, ['ERROR', 'config', $_] for @parse_errors;

# ---- pass 3: cross-check against snapshot -------------------------------
my %managed;   # path -> stanza
for my $st (@stanzas) { $managed{$_} = $st for @{ $st->{paths} } }

open my $sfh, '<', $state_path or die "open $state_path: $!\n";
while (my $line = <$sfh>) {
    next if $line =~ /^\s*(?:#|$)/;
    chomp $line;
    my ($path, $size, $mtime) = split /\t/, $line;
    my $st = $managed{$path};
    if (!$st) {
        push @findings, ['WARN', $path, 'log present but not managed by any stanza'];
        next;
    }
    my ($freq) = grep { exists $st->{directives}{$_} } sort keys %FREQ_SECONDS;
    $freq ||= (grep { exists $global{$_} } sort keys %FREQ_SECONDS)[0] || 'weekly';
    my $age = $now - $mtime;
    if ($age > 2 * $FREQ_SECONDS{$freq}) {
        push @findings, ['ERROR', $path,
            sprintf('not rotated: mtime %dd old but schedule is %s',
                int($age / 86400), $freq)];
    }
    if (exists $st->{directives}{size}) {
        my $limit = parse_size($st->{directives}{size});
        if (defined $limit and $size > $limit) {
            push @findings, ['WARN', $path,
                sprintf('size %d exceeds size directive %s',
                    $size, $st->{directives}{size})];
        }
    }
}
close $sfh;

# ---- report -------------------------------------------------------------
printf "logrotate-check: %d stanzas, %d global directives\n",
    scalar @stanzas, scalar keys %global;
my $errors = 0;
for my $f (sort { $a->[0] cmp $b->[0] or $a->[1] cmp $b->[1] or $a->[2] cmp $b->[2] } @findings) {
    printf "%-5s %-28s %s\n", @$f;
    $errors++ if $f->[0] eq 'ERROR';
}
print "clean: no findings\n" unless @findings;
printf "summary: %d error(s), %d warning(s)\n", $errors, scalar(@findings) - $errors;
exit($errors ? 1 : 0);

# ---- helpers ------------------------------------------------------------
sub parse_stanza {
    my ($lines, $start) = @_;
    my %st = (directives => {}, unknown => {});
    my $i = $start;
    my $in_script = 0;
    while ($i < @$lines) {
        my $line = strip($lines->[$i]);
        if ($in_script) {
            $in_script = 0 if $line eq 'endscript';
            $i++; next;
        }
        if ($line eq '}') { return (\%st, $i - $start + 1) }
        if ($line =~ /^(?:pre|post)rotate$/) { $in_script = 1; $i++; next }
        if ($line ne '' and $line !~ /^#/) {
            my ($k, $v) = parse_directive($line);
            if (!defined $k) { $st{unknown}{$line} = 1 }
            elsif ($KNOWN_SIMPLE{$k} or exists $FREQ_SECONDS{$k}
                   or $k =~ /^(?:rotate|size|create|olddir|maxage)$/) {
                $st{directives}{$k} = $v;
            }
            else { $st{unknown}{$k} = 1 }
        }
        $i++;
    }
    die "missing closing brace\n";
}

# A directive is a bare word, or word + value(s).  Written with /x
# because the original unreadable version got misedited twice.
sub parse_directive {
    my ($line) = @_;
    if ($line =~ m{
            ^ ( [a-z][a-z0-9_]* )      # directive name
            (?: \s+ (.+?) )?           # optional argument blob
            \s* $
        }x) {
        my ($k, $v) = ($1, $2);
        return ($k, defined $v ? $v : 1);
    }
    return;
}

sub parse_size {
    my ($s) = @_;
    return unless $s =~ /^(\d+)([kMG]?)$/;
    my %mult = ('' => 1, k => 1024, M => 1024**2, G => 1024**3);
    return $1 * $mult{$2};
}

sub strip { my $s = shift; $s =~ s/^\s+|\s+$//g; $s =~ s/^#.*//; $s }

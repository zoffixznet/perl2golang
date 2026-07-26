#!/usr/bin/perl
use strict;
use warnings;

# die/eval as Perl's try/catch. Warnings are routed to stdout through
# $SIG{__WARN__} so the whole transcript stays in one stream, which is a
# common thing to do in a script that writes its own log.

local $SIG{__WARN__} = sub { print "WARN: $_[0]" };

sub parse_int {
    my ($name, $raw) = @_;
    die "missing value for $name\n"            unless defined $raw && length $raw;
    die "$name must be an integer, got '$raw'\n" unless $raw =~ /^-?\d+$/;
    return $raw + 0;
}

sub load_config {
    my ($path) = @_;
    open my $fh, '<', $path or die "cannot read $path: $!\n";
    my %cfg;
    while (my $line = <$fh>) {
        chomp $line;
        next unless $line =~ /\S/;
        my ($k, $v) = $line =~ /^\s*(\w+)\s*=\s*(.*?)\s*$/
            or die "malformed line $.: $line\n";
        $cfg{$k} = $v;
    }
    close $fh or die "close $path: $!\n";
    return \%cfg;
}

my $cfg = load_config('files/config.ini');
print "loaded ", scalar(keys %$cfg), " settings: ", join(',', sort keys %$cfg), "\n";

# Basic eval/$@ pair, run over both good and bad input.
for my $key (qw(workers timeout retries port)) {
    my $value = eval { parse_int($key, $cfg->{$key}) };
    if ($@) {
        my $err = $@;
        chomp $err;
        printf "%-8s FAILED: %s\n", $key, $err;
    }
    else {
        printf "%-8s = %d\n", $key, $value;
    }
}

# eval returning a value, with the ternary "or default" idiom.
my $workers = eval { parse_int('workers', $cfg->{workers}) } || 1;
my $timeout = eval { parse_int('timeout', $cfg->{timeout}) } || 30;
print "effective: workers=$workers timeout=$timeout\n";

# Runtime errors, not just explicit die: division by zero is trapped too.
for my $divisor (4, 0, 2) {
    my $r = eval { 100 / $divisor };
    if (my $err = $@) {
        $err =~ s/ at .*//s;
        printf "100/%d -> error: %s\n", $divisor, $err;
    }
    else {
        printf "100/%d -> %s\n", $divisor, $r;
    }
}

# Nested eval: the inner failure is caught, annotated and rethrown.
sub outer {
    my ($n) = @_;
    my $inner = eval { inner($n) };
    if ($@) {
        my $e = $@;
        chomp $e;
        die "outer(): while handling $n: $e\n";
    }
    return $inner;
}
sub inner {
    my ($n) = @_;
    die "inner() refuses negatives\n" if $n < 0;
    return sqrt $n;
}

for my $n (9, -1) {
    my $v = eval { outer($n) };
    if ($@) {
        my $e = $@; chomp $e;
        print "caught: $e\n";
    }
    else {
        printf "outer(%d) = %.1f\n", $n, $v;
    }
}

# warn does not stop anything; it just reports.
sub checked_sqrt {
    my ($n) = @_;
    if ($n < 0) {
        warn "checked_sqrt: clamping $n to 0\n";
        $n = 0;
    }
    return sqrt $n;
}
printf "sqrt: %.2f %.2f\n", checked_sqrt(16), checked_sqrt(-5);

# local $@ so an inner cleanup cannot clobber an error being handled.
sub cleanup_that_dies {
    local $@;
    eval { die "cleanup exploded\n" };
    return 'cleaned';
}
eval { die "original failure\n" };
my $saved = $@;
my $note  = cleanup_that_dies();
printf "after %s, \$@ still: %s", $note, $saved;

# $@ is cleared by a successful eval, so it must be captured immediately.
eval { die "first\n" };
eval { 1 };
printf "after a successful eval, \$@ is %s\n", ($@ eq '' ? '(empty)' : $@);

print "done\n";

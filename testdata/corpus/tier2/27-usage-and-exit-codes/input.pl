#!/usr/bin/perl
use strict;
use warnings;

# Argument validation with distinct exit codes per failure mode, the way a
# script meant to be driven from cron or a Makefile is written. Diagnostics
# go to stdout here so the transcript stays in one stream.

use constant {
    EX_OK       => 0,
    EX_USAGE    => 2,
    EX_NOINPUT  => 66,
    EX_DATAERR  => 65,
};

sub usage {
    my ($msg) = @_;
    print "error: $msg\n" if defined $msg;
    print "usage: $0 --mode=MODE COUNT [COUNT...]\n";
    print "  --mode=sum|max|avg   how to combine the counts\n";
    print "  --strict             reject non-numeric input\n";
    print "exit codes: 0 ok, 2 usage, 65 data error, 66 no input\n";
    return;
}

my $mode   = 'sum';
my $strict = 0;
my @counts;

for my $arg (@ARGV) {
    if    ($arg =~ /^--mode=(\w+)$/) { $mode = $1 }
    elsif ($arg eq '--strict')       { $strict = 1 }
    elsif ($arg eq '--help')         { usage(); exit EX_OK }
    elsif ($arg =~ /^-/)             { usage("unknown option $arg"); exit EX_USAGE }
    else                             { push @counts, $arg }
}

my %modes = (
    sum => sub { my $t = 0; $t += $_ for @_; return $t },
    max => sub { my $m = $_[0]; for (@_) { $m = $_ if $_ > $m } return $m },
    avg => sub { my $t = 0; $t += $_ for @_; return $t / @_ },
);

unless (exists $modes{$mode}) {
    usage("unknown mode '$mode'");
    exit EX_USAGE;
}

unless (@counts) {
    usage('no counts supplied');
    exit EX_NOINPUT;
}

print "program: $0\n";
print "mode:    $mode\n";
print "strict:  $strict\n";
print "inputs:  @counts\n";

my @bad = grep { !/^-?\d+(?:\.\d+)?$/ } @counts;
if (@bad) {
    print "non-numeric input: @bad\n";
    if ($strict) {
        print "strict mode: refusing to continue\n";
        exit EX_DATAERR;
    }
    print "dropping non-numeric values\n";
    @counts = grep { /^-?\d+(?:\.\d+)?$/ } @counts;
}

unless (@counts) {
    print "nothing numeric left\n";
    exit EX_DATAERR;
}

printf "%s of %d value(s) = %.2f\n", $mode, scalar @counts, $modes{$mode}->(@counts);
print "done\n";
exit EX_DATAERR if $mode eq 'avg' && $modes{avg}->(@counts) == 0;
exit EX_OK;

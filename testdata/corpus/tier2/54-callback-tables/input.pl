#!/usr/bin/perl
# Callbacks kept in collections: the dispatch table, the pipeline, and the
# closure that remembers something. All three put functions in a slot, which
# is where Perl's freedom about what a sub takes and returns meets Go's one
# type per slot.
use strict;
use warnings;

my @audit;

my %actions = (
    upper   => sub { uc $_[0] },
    trim    => sub { my ($s) = @_; $s =~ s/^\s+|\s+$//g; $s },
    tag     => sub { my ( $s, $t ) = @_; "[$t] $s" },
    note    => sub { push @audit, $_[0]; return },
    measure => sub { length $_[0] },
);

print "-- one at a time --\n";
printf "upper:   %s\n", $actions{upper}->('quiet');
printf "trim:    [%s]\n", $actions{trim}->('  padded  ');
printf "tag:     %s\n", $actions{tag}->( 'body', 'warn' );
printf "measure: %s\n", $actions{measure}->('four');

$actions{note}->('first');
$actions{note}->('second');
printf "audit:   %s\n", join( ' ', @audit );

print "-- chosen by name --\n";
for my $name ( sort keys %actions ) {
    next if $name eq 'note' or $name eq 'tag';
    printf "%-8s -> %s\n", $name, $actions{$name}->('  Mixed Case  ');
}

print "-- a pipeline in an array --\n";
my @pipeline = (
    sub { my ($s) = @_; $s =~ s/\s+/ /g; $s },
    sub { my ($s) = @_; lc $s },
    sub { my ($s) = @_; "<$s>" },
);
my $text = "  Some   Spaced   Text  ";
for my $stage (@pipeline) {
    $text = $stage->($text);
}
printf "piped:   %s\n", $text;

print "-- closing over a counter --\n";
my %counters;
for my $kind (qw(hit miss)) {
    my $n = 0;
    $counters{$kind} = sub { my ($by) = @_; $n += $by; $n };
}
$counters{hit}->(1);
$counters{hit}->(3);
$counters{miss}->(1);
printf "hit=%s miss=%s\n", $counters{hit}->(0), $counters{miss}->(0);

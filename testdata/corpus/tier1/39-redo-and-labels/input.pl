#!/usr/bin/perl
# redo, and what it does to the other two loop keywords. redo re-runs the body
# without advancing, so a loop that uses it needs its next and last to still
# mean the loop rather than the retry.
use strict;
use warnings;

print "--- a retry that gives up ---\n";
my %attempts;
for my $job (qw(alpha beta gamma)) {
    $attempts{$job}++;
    if ( $job eq 'beta' && $attempts{$job} < 3 ) {
        redo;
    }
    printf "%-6s succeeded on attempt %d\n", $job, $attempts{$job};
}

print "--- redo beside next and last ---\n";
my $seen = 0;
my @log;
for my $n ( 1 .. 6 ) {
    $seen++;
    if ( $n == 2 && $seen < 5 ) {
        push @log, "retry $n";
        redo;
    }
    if ( $n % 2 == 0 ) {
        push @log, "skip $n";
        next;
    }
    if ( $n == 5 ) {
        push @log, "stop at $n";
        last;
    }
    push @log, "take $n";
}
print "  $_\n" for @log;
printf "body ran %d times\n", $seen;

print "--- redo in a while loop ---\n";
my $i     = 0;
my $fixes = 0;
while ( $i < 4 ) {
    if ( $i == 2 && $fixes < 2 ) {
        $fixes++;
        print "  patching pass $fixes\n";
        redo;
    }
    print "  handled $i\n";
    $i++;
}
printf "fixes: %d\n", $fixes;

print "--- a labelled loop is unaffected ---\n";
OUTER: for my $i ( 1 .. 3 ) {
    for my $j ( 1 .. 3 ) {
        next OUTER if $j == 2;
        last OUTER if $i == 3;
        print "  $i.$j\n";
    }
}

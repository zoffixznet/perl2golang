#!/usr/bin/perl
# TRAP: the fork process model. The child continues from the fork with its
# own copy of every variable and its own exit status; the parent reaps it
# and reads the status out of $?. Go's runtime is threaded and cannot
# survive a bare fork, so no translation exists without deciding between a
# goroutine and an exec'd child, and that is a decision, not a conversion.
use strict;
use warnings;

my $shared = 'parent value';

my $pid = fork();
die "fork: $!\n" unless defined $pid;

if ( $pid == 0 ) {
    # Only the child runs this, on its own copy of the world.
    $shared = 'child value';
    exit 7;
}

waitpid $pid, 0;
printf "child exited %d\n",        $? >> 8;
printf "parent still sees '%s'\n", $shared;

# A second fork whose child never exits on its own: the block falls off the
# end, so the child would continue into the parent's code below the if.
my $pid2 = fork();
die "fork: $!\n" unless defined $pid2;
if ( $pid2 == 0 ) {
    exit 0;
}
waitpid $pid2, 0;
print "both children reaped\n";

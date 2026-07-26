#!/usr/bin/perl
# TRAP: goto &sub -- a tail-call that REPLACES the current stack frame.
# caller() never sees the trampoline; @_ passes through invisibly.
use strict;
use warnings;

sub real_work {
    my $up = ( caller(1) )[3] // "top-level";
    print "real_work(@_): frame above me: $up\n";
    return "ok";
}

sub trampoline {
    print "trampoline ran\n";
    goto &real_work;               # this frame VANISHES from the stack
}

sub normal_call {
    return real_work(@_);          # ordinary call: adds a frame
}

print trampoline( 1, 2 ), "\n";
print normal_call( 1, 2 ), "\n";

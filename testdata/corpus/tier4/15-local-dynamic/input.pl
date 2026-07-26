#!/usr/bin/perl
# TRAP: local = DYNAMIC scoping. A callee sees the caller's temporary
# value of a global, decided by the call chain at runtime, not by the
# source text. Restored even when the scope exits via die.
use strict;
use warnings;

our $mode = "normal";
sub describe { return "mode=$mode" }    # WHICH $mode? depends on caller!
sub deeper   { return describe() }

print describe(), "\n";

sub with_debug {
    local $mode = "debug";
    return deeper();                    # two frames down, still sees "debug"
}
print with_debug(), "\n";
print describe(), "\n";                 # restored at scope exit

sub might_die {
    local $mode = "panic";
    die "boom\n";
}
eval { might_die() };
print describe(), " (restored even after die)\n";

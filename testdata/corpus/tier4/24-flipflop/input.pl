#!/usr/bin/perl
# TRAP: the scalar '..' flip-flop keeps HIDDEN PER-OPERATOR state across
# loop iterations: a stateful toggle, not a range. Its value is a
# sequence number, and the last hit is marked with "E0".
use strict;
use warnings;

my @lines = (
    "intro",
    "BEGIN",  "  line one", "  line two", "END",
    "outro",
    "BEGIN",  "  second block", "END",
    "tail",
);

for my $l (@lines) {
    if ( $l =~ /^BEGIN/ .. $l =~ /^END/ ) {    # toggles on BEGIN, off after END
        print "in: $l\n";
    }
}

for my $l (@lines) {
    my $state = ( $l =~ /^BEGIN/ .. $l =~ /^END/ );
    print "state=$state $l\n" if $state;       # 1, 2, 3, "4E0", 1, 2, "3E0"
}

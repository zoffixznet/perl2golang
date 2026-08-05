#!/usr/bin/perl
# TRAP: the corners of the pack template language. A checksum code folds the
# data instead of returning fields, a BER integer has no fixed width at all,
# and a byte-order modifier rewrites the meaning of the code before it. None
# of these are in the interpreter a converted program carries, and the tool
# has to say so at conversion time, naming the code.
use strict;
use warnings;

my $data = "session\x00counters";

# %16C* folds every byte into a 16-bit checksum rather than listing them.
my $ck = unpack '%16C*', $data;
printf "checksum: %d\n", $ck;

# w is a BER compressed integer: one to many bytes, decided by the value.
my $ber = pack 'w', 300;
printf "ber bytes: %d\n", length $ber;

# l> flips a native-order code to big-endian with a modifier.
my ($flip) = unpack 'l>', pack( 'l>', -7 );
printf "flipped: %d\n", $flip;

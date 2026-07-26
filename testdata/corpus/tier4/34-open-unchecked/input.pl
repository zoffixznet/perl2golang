#!/usr/bin/perl
# TRAP: open() failure is a false RETURN VALUE, not an exception. The
# script keeps running: reads from the never-opened handle return
# nothing, and the exit status is still 0 ("success").
use strict;
use warnings;
no warnings qw(unopened closed uninitialized);

my $ok = open( my $fh, '<', '/nonexistent/config.txt' );
print "open ok? ", ( $ok ? "yes" : "no ($!)" ), "\n";

my @lines = <$fh>;                       # no error; just an empty list
print "lines read: ", scalar(@lines), "\n";

my $first = <$fh>;                       # scalar read: undef
print "first line: ", ( $first // "undef" ), "\n";

print "script keeps running and exits 0\n";

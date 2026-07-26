#!/usr/bin/perl -w -l
# CASE file-01: the `#!` line. Perl RE-PARSES its own shebang line for switches
# even when the script was not started via the shebang, so `-w`, `-l`, `-s`,
# `-T` there change how the program behaves.
use strict;

# -l was requested on the shebang line, so print appends $\ = "\n".
print "file-01 output-record-separator-set: " . (defined $\ ? "yes" : "no");
print "file-01 warnings-on: " . ($^W ? "yes" : "no");
print "file-01 shebang-line: " . do {
    open my $fh, '<', $0 or die;
    my $l = <$fh>;
    close $fh;
    $l =~ s/\s+\z//r;
};
# Note the missing "\n" on every print above: -l supplied it.

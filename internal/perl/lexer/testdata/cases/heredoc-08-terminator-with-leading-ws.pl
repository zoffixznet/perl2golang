#!/usr/bin/perl
# CASE heredoc-08: for a NON-indented heredoc the terminator must be at column 0
# and be the whole line. A line that merely CONTAINS the terminator, or has
# leading whitespace, does NOT end the body.
use strict; use warnings;

my $t = <<"EOT";
one
  EOT
EOTx
xEOT
EOT
print "heredoc-08 body-lines: ", scalar(split /\n/, $t), "\n";
print "heredoc-08 body: ", ($t =~ s/\n/|/gr), "\n";

# The indented form DOES accept leading whitespace on the terminator.
my $i = <<~"EOT";
    a
      EOT-not-a-terminator
    EOT
print "heredoc-08 indented-body: ", ($i =~ s/\n/|/gr), "\n";

# Trailing whitespace after the terminator also disqualifies it: the line below
# that reads "W" followed by a single space does NOT end the body.
my $w = <<'W';
p
W 
W
print "heredoc-08 trailing-ws-lines: ", scalar(split /\n/, $w), "\n";
print "heredoc-08 trailing-ws-body: ", ($w =~ s/\n/|/gr), "\n";

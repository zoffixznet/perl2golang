#!/usr/bin/perl
# CASE heredoc-11: an UNTERMINATED heredoc (EOF reached before the terminator) is
# a fatal compile error. Also: a heredoc opened on the last line of the file.
# Both are checked in child processes so this file itself stays valid.
use strict; use warnings;

my $bad = <<'PERL';
my $x = <<"EOT";
never closed
PERL
open my $fh, '>', "heredoc-11-child.pl" or die;
print $fh $bad;
close $fh;
my $out = `$^X -c heredoc-11-child.pl 2>&1`;
$out =~ s/\s+\z//;
print "heredoc-11 unterminated: ",
      ($out =~ /Can't find string terminator/ ? "FATAL: $out" : "unexpected [$out]"), "\n";

# A heredoc whose body is the very end of file WITH the terminator present is fine.
my $good = "my \$x = <<'EOT';\nbody\nEOT\nprint qq{ok:\$x};\n";
open my $g, '>', "heredoc-11-child2.pl" or die;
print $g $good;
close $g;
my $out2 = `$^X heredoc-11-child2.pl 2>&1`;
$out2 =~ s/\s+\z//;
print "heredoc-11 terminated-at-eof: $out2\n";

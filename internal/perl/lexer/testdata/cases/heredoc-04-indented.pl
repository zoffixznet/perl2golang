#!/usr/bin/perl
# CASE heredoc-04: `<<~EOT` -- indented heredoc (5.26+). The terminator may be
# indented, and that indentation is stripped from every body line.
use strict; use warnings;

sub build {
    my $name = shift;
    my $t = <<~"EOT";
        Dear $name,
          indented further
        Bye.
        EOT
    return $t;
}
my $s = build("Ada");
print "heredoc-04 dedented:\n$s";
print "heredoc-04 first-char-is-D: ", (substr($s,0,1) eq "D" ? "yes" : "no"), "\n";
print "heredoc-04 second-line-indent: ",
      ($s =~ /\n(\s*)indented/ ? length($1) : -1), "\n";

my $lit = <<~'EOT';
    no $interp here
    EOT
print "heredoc-04 indented-single: $lit";

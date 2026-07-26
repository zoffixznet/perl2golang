#!/usr/bin/perl
# CASE stmt-06: `format NAME =` ... `.` is a whole sub-language with its own
# line-oriented lexer, terminated by a line containing only `.`. Also variable
# attributes (`my $x :shared`).
use strict; use warnings;

our ($name, $qty) = ("widget", 3);

format REPORT =
@<<<<<<<<<< @>>>
$name,      $qty
.

open(my $fh, '>', \my $buf) or die;
{
    # Select the in-memory handle and give it the REPORT format.
    my $old = select($fh);
    local $~ = "REPORT";
    local $= = 1_000_000;      # huge page length: never triggers a top-of-page
    write;
    select($old);
}
close $fh;
$buf =~ s/\s+\z//;
print "stmt-06 format-output: [$buf]\n";

# A `.` on its own line ONLY terminates a format; elsewhere it is concatenation.
my $cat = "a" . "b";
print "stmt-06 concat-dot: $cat\n";

# Variable attributes: `my $x :attr` is lexed as declaration + attribute list.
# An unknown attribute is a COMPILE error raised after the lexer accepted it.
my $out = `$^X -e 'my \$x :nosuchattr = 1; print "ok";' 2>&1`;
$out =~ s/\s+/ /g; $out =~ s/\s+\z//;
print "stmt-06 var-attribute: ", ($out =~ /Invalid SCALAR attribute/ ? "lexed, then rejected: $out" : "[$out]"), "\n";

# A `.` on a line by itself outside a format is just an operator/syntax error.
my $out2 = `$^X -e 'my \$x = 1;
.
print \$x;' 2>&1`;
$out2 =~ s/\s+/ /g; $out2 =~ s/\s+\z//;
print "stmt-06 lone-dot-outside-format: ", ($out2 =~ /syntax error/ ? "SYNTAX ERROR" : "[$out2]"), "\n";

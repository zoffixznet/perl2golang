#!/usr/bin/perl
# CASE bare-02: a bareword used as a FILEHANDLE. `print FH LIST` has no comma
# after the handle -- the lexer must know that the first bareword after `print`
# may be a handle, and that `print FH` differs from `print FH()`.
use strict; use warnings;

my $buf = "";
open(OUT, '>', \$buf) or die;
print OUT "written via bareword handle\n";
printf OUT "%s\n", "printf too";
close OUT;
print "bare-02 captured: ", ($buf =~ s/\n/|/gr), "\n";

# STDOUT / STDERR are just barewords in the same slot.
print STDOUT "bare-02 to-stdout\n";

# A lexical handle in the same slot needs no braces.
open(my $fh, '>', \my $b2) or die;
print $fh "lexical\n";
close $fh;
print "bare-02 lexical: ", ($b2 =~ s/\n//r), "\n";

# An expression as a handle needs braces (see stmt-05).
my %handles;
open($handles{log}, '>', \my $b3) or die;
print { $handles{log} } "from a hash element\n";
close $handles{log};
print "bare-02 braced-expr: ", ($b3 =~ s/\n//r), "\n";

# `print FOO "x"` vs `print FOO, "x"`: with a comma FOO is a bareword string.
{
  no strict 'subs';
  my @l = (FOO, "x");
  print "bare-02 with-comma: [", join("|", @l), "]\n";
}

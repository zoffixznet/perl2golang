#!/usr/bin/perl
# CASE stmt-03: `print {EXPR} LIST` -- the braces hold a filehandle EXPRESSION.
# The `{` here is neither a block nor a hashref, and there is no comma after it.
use strict; use warnings;

my %out;
open($out{a}, '>', \my $ba) or die;
open($out{b}, '>', \my $bb) or die;

print { $out{a} } "to a\n";
print { $out{b} } "to b\n";
printf { $out{a} } "%s\n", "printf to a";
close $out{$_} for qw(a b);
print "stmt-03 a: ", ($ba =~ s/\n/|/gr), "\n";
print "stmt-03 b: ", ($bb =~ s/\n/|/gr), "\n";

# The same with an array element and a ternary inside the braces.
my @fhs;
open($fhs[0], '>', \my $bc) or die;
print { $fhs[0] } "array element handle\n";
my $pick = 1;
print { $pick ? $fhs[0] : \*STDOUT } "ternary handle\n";
close $fhs[0];
print "stmt-03 c: ", ($bc =~ s/\n/|/gr), "\n";

# Without braces, `print $out{a} "x"` is a syntax error.
my $err = `$^X -e 'my %o; open(\$o{a}, ">", \\my \$b); print \$o{a} "x";' 2>&1`;
$err =~ s/\s+/ /g; $err =~ s/\s+\z//;
print "stmt-03 no-braces: ", ($err =~ /syntax error|String found/ ? "SYNTAX ERROR" : "[$err]"), "\n";

# `print {$fh} @list` vs `print {a=>1}` (a hashref argument to print).
print "stmt-03 hashref-arg: ", (sub { ref($_[0]) }->({a=>1})), "\n";

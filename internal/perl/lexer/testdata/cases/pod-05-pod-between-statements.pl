#!/usr/bin/perl
# CASE pod-05: POD is only recognised where a STATEMENT could begin. It works
# between statements, inside a sub body and inside a loop body -- but a line
# starting with `=word` in the middle of an open expression (inside parens, inside
# a list literal) is a SYNTAX ERROR, not POD.
use strict; use warnings;

sub add {
    my ($a, $b) = @_;

=head2 add

Documentation in the middle of a sub body.

=cut

    return $a + $b;
}

print "pod-05 sub-with-pod: ", add(2,3), "\n";

my @out;
for my $i (1..3) {

=item loop doc

POD inside a loop body.

=cut

    push @out, $i * 2;
}
print "pod-05 loop: @out\n";

# POD inside an OPEN expression: rejected. Proven in a child process.
my $src = "my \@x = (1,\n=pod\ntext\n=cut\n2);\nprint \"\@x\";\n";
open my $fh, '>', "pod-05-child.pl" or die;
print $fh $src;
close $fh;
my $out = `$^X pod-05-child.pl 2>&1`;
$out =~ s/\s+\z//;
print "pod-05 pod-inside-parens: ", ($out =~ /syntax error/ ? "SYNTAX ERROR: $out" : "[$out]"), "\n";

# Same for a POD block between elements of a hash literal.
my $src2 = "my %h = (\n  a => 1,\n\n=for comment\nx\n\n=cut\n\n  b => 2,\n);\nprint join(q{,}, sort keys %h);\n";
open my $fh2, '>', "pod-05-child2.pl" or die;
print $fh2 $src2;
close $fh2;
my $out2 = `$^X pod-05-child2.pl 2>&1`;
$out2 =~ s/\s+\z//;
print "pod-05 pod-inside-hash-literal: ", ($out2 =~ /syntax error/ ? "SYNTAX ERROR" : "[$out2]"), "\n";

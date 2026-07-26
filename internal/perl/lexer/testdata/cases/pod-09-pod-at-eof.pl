#!/usr/bin/perl
# CASE pod-09: a POD block that runs to END OF FILE with no `=cut` is legal.
# The lexer must treat EOF as an implicit terminator, not raise an error.
use strict; use warnings;

print "pod-09 code-ran\n";

my $self = do { local $/; open my $fh, '<', $0 or die; <$fh> };
print "pod-09 has-cut-after-final-pod: ",
      ($self =~ /=head1 TRAILING.*=cut/s ? "yes" : "no"), "\n";
print "pod-09 file-ends-in-pod: ",
      ($self =~ /=head1 TRAILING[^=]*\z/s ? "yes" : "no"), "\n";

=head1 TRAILING POD

This POD block has no =cut. The file simply ends here, and perl accepts it.

Code-shaped text that must never be lexed:

    my $x = q{ unbalanced
    <<'HEREDOC_NEVER_OPENED'

#!/usr/bin/perl
# CASE pod-04: a POD block starts at a line whose FIRST character is `=` followed
# by an identifier, in a position where a statement could begin, and runs to a
# line starting with `=cut` (or EOF).
use strict; use warnings;

print "pod-04 before\n";

=pod

This is POD. It contains code-looking text that must be ignored:

    my $x = <<'NOT_A_HEREDOC';
    q{ unbalanced brace
    "unterminated string

=cut

print "pod-04 after-pod\n";

=head1 ANOTHER BLOCK

=head2 Nested heading

Body text.

=over 4

=item one

=back

=cut

print "pod-04 after-second\n";

=anything_at_all

Any =word starts POD, even an unknown command.

=cut

print "pod-04 end\n";

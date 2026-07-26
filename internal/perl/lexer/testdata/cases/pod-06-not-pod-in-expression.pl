#!/usr/bin/perl
# CASE pod-06: `=` at the start of a line is POD ONLY when a statement could begin
# there. Inside a string, inside a heredoc body, and as a continuation of an
# expression it is ordinary text or an operator.
use strict; use warnings;

# `=head1` inside a heredoc body: NOT pod, just text.
my $t = <<'EOT';
=head1 not really pod
=cut
EOT
print "pod-06 heredoc-body-lines: ", scalar(split /\n/, $t), "\n";

# `=` at column 0 as a continuation of a multi-line expression: this IS treated
# as POD by perl's lexer, which is why it is a classic bug. Proven in a child.
my $out = `$^X -e 'my \$x
= 5;
print "got \$x";' 2>&1`;
$out =~ s/\s+\z//;
print "pod-06 leading-equals-continuation: [$out]\n";

# `= 5` indented by one space is fine (POD requires column 0).
my $y
 = 5;
print "pod-06 indented-continuation: $y\n";

# `=` inside a string at the start of a line.
my $s = "line one
=head1 inside a string
end";
print "pod-06 string-lines: ", scalar(split /\n/, $s), "\n";

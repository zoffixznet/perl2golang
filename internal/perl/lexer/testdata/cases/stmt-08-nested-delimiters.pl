#!/usr/bin/perl
# CASE stmt-08: brace/paren/bracket matching must be done by the LEXER, not by a
# naive counter, because delimiters inside strings, regexes, comments, POD and
# heredoc bodies are data. This file has every kind of unbalanced-looking text.
use strict; use warnings;

my %h;
$h{'}'}      = "close-brace key";
$h{'{'}      = "open-brace key";
$h{"a}b{c"}  = "both";
print "stmt-08 brace-keys: ", join(",", map { $h{$_} } sort keys %h), "\n";

# Unbalanced braces inside strings, regexes and comments.
my $s = "{{{ unbalanced";                  # } <- inside a comment
my $t = q{balanced {inner} outer};
my $re = qr/\{ [^}]* \}/x;
print "stmt-08 string: [", length($s), "] q: [$t]\n";
print "stmt-08 regex: ", ("x{yz}" =~ $re ? "match" : "no"), "\n";

# A block whose body contains a string full of closing braces.
my $out = do {
    my $inner = "}}}}}";
    length($inner);
};
print "stmt-08 do-block: $out\n";

# Heredoc body full of unbalanced delimiters.
my $hd = <<'EOT';
} ) ] } ) ]
sub not_a_sub {
EOT
print "stmt-08 heredoc-lines: ", scalar(split /\n/, $hd), "\n";

# Parens inside a character class and inside a quoted key.
my %p = ( '(' => 1, ')' => 2 );
print "stmt-08 paren-keys: ", scalar(keys %p), " regex: ",
      ("a(b" =~ /a[(]b/ ? "match" : "no"), "\n";

=head1 POD WITH BRACES

    } } } } sub {  q{  "

=cut

print "stmt-08 after-pod: ok\n";
__END__
} ) ] unbalanced trailer that must be ignored

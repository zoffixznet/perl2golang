#!/usr/bin/perl
# CASE sigil-06: `@{[ EXPR ]}` and `${\ EXPR }` -- the two idioms for putting an
# arbitrary expression inside an interpolating string. Inside the braces the
# lexer must switch back to full CODE lexing, recursively.
use strict; use warnings;

my @n = (3,4,5);
sub total { my $s = 0; $s += $_ for @_; return $s }

print "sigil-06 list-trick: @{[ map { $_*2 } @n ]}\n";
print "sigil-06 scalar-trick: ${\ total(@n) }\n";
print "sigil-06 call: @{[ total(@n) ]}\n";
print "sigil-06 nested-quotes: @{[ join(q{-}, map { qq{<$_>} } @n) ]}\n";
print "sigil-06 nested-braces: @{[ do { my %h=(a=>1); join q{}, sort keys %h } ]}\n";
print "sigil-06 ternary: @{[ @n > 2 ? q{many} : q{few} ]}\n";

# A string containing a `}` inside the embedded code.
print "sigil-06 inner-brace: @{[ ( q<}> ) ]}\n";

# The same trick inside a regex and inside a heredoc.
print "sigil-06 in-regex: ", ("6" =~ /^@{[ 3*2 ]}$/ ? "match" : "no"), "\n";
my $hd = <<"EOT";
in heredoc: @{[ total(@n) ]}
EOT
print "sigil-06 $hd";

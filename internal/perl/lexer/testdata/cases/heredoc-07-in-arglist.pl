#!/usr/bin/perl
# CASE heredoc-07: a heredoc inside a function call argument list, including a
# nested call, and one used as a hash value. The `)` `,` `;` after the opener are
# ordinary tokens on the SAME line as `<<`.
use strict; use warnings;

sub tag { my ($n, $b) = @_; return "<$n>" . $b . "</$n>\n" }

print tag("p", <<"BODY"), "after\n";
paragraph text
BODY

my %h = (
    greeting => <<"G",
hi there
G
    farewell => <<"F",
bye now
F
);
print "heredoc-07 hash: ", $h{greeting} =~ s/\n//r, "/", $h{farewell} =~ s/\n//r, "\n";

# Nested calls, heredoc in the inner one.
print "heredoc-07 nested: ", uc(tag("b", <<"I")), "";
inner
I

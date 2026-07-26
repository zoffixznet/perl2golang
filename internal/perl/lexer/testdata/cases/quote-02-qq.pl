#!/usr/bin/perl
# CASE quote-02: `qq` interpolates like "". Same delimiter rules as q.
use strict; use warnings;

my $n = "N";
my @a = (1,2);
print "quote-02 paren: ", qq(interp $n and @a), "\n";
print "quote-02 brace: ", qq{brace $n}, "\n";
print "quote-02 bang: ",  qq!bang $n!, "\n";
print "quote-02 slash: ", qq/slash $n/, "\n";
print "quote-02 hash: ",  qq#hash $n#, "\n";

# Escapes work in qq regardless of delimiter; the DELIMITER itself can be escaped.
print "quote-02 escaped-delim: ", qq{a\}b}, "\n";
print "quote-02 escapes: ", qq{tab[\t] hex[\x41] nl-len[${\ length(qq{\n}) }]}, "\n";

# Nested balanced braces need no escaping.
print "quote-02 nested: ", qq{outer {inner} outer}, "\n";

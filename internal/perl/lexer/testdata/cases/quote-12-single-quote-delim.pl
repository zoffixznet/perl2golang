#!/usr/bin/perl
# CASE quote-12: when the delimiter is `'`, the construct does NOT interpolate.
# This applies to m'', s''', qr'', and heredoc <<''. The delimiter changes the
# interpolation rule, so the lexer must record it.
use strict; use warnings;

my $v = "VAL";

print "quote-12 m-single: ", ('$v' =~ m'$v' ? "literal-match" : "no"), "\n";
print "quote-12 m-slash:  ", ('VAL' =~ m/$v/ ? "interp-match"  : "no"), "\n";

(my $a = 'a$v b') =~ s'$v'X';
print "quote-12 s-single: $a\n";

(my $b = 'aVALb') =~ s/$v/Y/;
print "quote-12 s-interp: $b\n";

# The replacement half of s''' is also literal.
(my $c = "z") =~ s/z/$v/;
(my $d = "z") =~ s'z'$v';
print "quote-12 replacement: interp=[$c] literal=[$d]\n";

# In s{...}'...' the FIRST part follows brace rules (interpolating) and the
# SECOND part is single-quoted (literal).
(my $e = "aVALb") =~ s{$v}'$v';
print "quote-12 mixed-halves: $e\n";

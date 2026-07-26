#!/usr/bin/perl
# CASE quote-07: three-part operators (s, tr, y). With a BRACKETING delimiter the
# two parts are independently delimited and may differ; with a non-bracketing
# delimiter the middle delimiter is shared: s/a/b/.
use strict; use warnings;

my $base = "aXbXc";

(my $a = $base) =~ s{X}{-};                # brace/brace
(my $b = $base) =~ s(X)(=);                # paren/paren
(my $c = $base) =~ s[X][+];                # bracket/bracket
(my $d = $base) =~ s<X><~>;                # angle/angle
print "quote-07 same-brackets: $a $b $c $d\n";

# MIXED: bracketing first part, different delimiter for the replacement.
(my $e = $base) =~ s{X}/./;
(my $f = $base) =~ s{X}!Q!;
(my $g = $base) =~ s(X)[R];
(my $g2 = $base) =~ s{X}'';                # single-quoted replacement, no interp
print "quote-07 mixed: $e $f $g $g2\n";

# `s{X}!!!` (an extra delimiter char) is a SYNTAX ERROR -- maximal munch does not
# rescue a malformed three-part form.
my $err = `$^X -e 'my \$s="aXb"; \$s =~ s{X}!!!; print \$s;' 2>&1`;
$err =~ s/\s+\z//;
print "quote-07 malformed: ", ($err =~ /syntax error/ ? "SYNTAX ERROR" : "[$err]"), "\n";

# Shared middle delimiter (non-bracketing).
(my $h = $base) =~ s/X/./g;
print "quote-07 shared: $h\n";

# tr and y with bracketing and mixed delimiters.
(my $i = $base) =~ tr{abc}{ABC};
(my $j = $base) =~ y{X}{x};
(my $k = $base) =~ tr(a)(A);
print "quote-07 tr: $i $j $k\n";

# Whitespace and a newline between the two bracketed parts.
(my $l = $base) =~ s{X}
                    {NL};
print "quote-07 split-lines: $l\n";

#!/usr/bin/perl
# CASE file-06: constructs where the source text is not known until RUNTIME, and
# where the file the lexer sees is not the file on disk. These are the true
# out-of-scope cases for any static lexer.
use strict; use warnings;

# 1. String eval: the lexed text is a runtime value.
my $piece = "2 + 3";
my $r = eval "$piece";
print "file-06 string-eval: $r\n";

my $built = join("", "pri", "nt ", '"assembled at runtime"');
eval $built;
print "\n";

# 2. s///ee: the replacement is evaluated, then the RESULT is evaluated again.
my $x = 21;
(my $s = "PLACEHOLDER") =~ s/PLACEHOLDER/'$x * 2'/ee;
print "file-06 double-eval-subst: $s\n";

# 3. A source filter rewrites the file before perl lexes it.
open my $fh, '>', "Filtered.pm" or die;
print $fh <<'MODULE';
package Filtered;
use Filter::Util::Call;
sub import {
    filter_add(sub {
        my $status = filter_read();
        s/GREETING/"hello from a source filter"/g if $status > 0;
        return $status;
    });
}
1;
MODULE
close $fh;

open my $fh2, '>', "file-06-user.pl" or die;
print $fh2 "use lib q{.};\nuse Filtered;\nprint GREETING;\n";
close $fh2;
my $out = `$^X file-06-user.pl 2>&1`;
$out =~ s/\s+\z//;
print "file-06 source-filter: [$out]\n";
print "file-06 filter-note: the text 'print GREETING;' on disk is never what perl lexes\n";

# 4. BEGIN blocks can define subs (and prototypes) that change later parsing.
my $out2 = `$^X -e 'BEGIN { *main::mylen = sub (\$) { length \$_[0] } } print mylen "abcd", "!";' 2>&1`;
$out2 =~ s/\s+/ /g; $out2 =~ s/\s+\z//;
print "file-06 begin-defined-prototype: [$out2]\n";

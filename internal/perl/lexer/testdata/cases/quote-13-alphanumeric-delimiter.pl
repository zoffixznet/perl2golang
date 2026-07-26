#!/usr/bin/perl
# CASE quote-13: a WORD character can never be a quote-like delimiter, and
# whitespace-then-word is not one either. `qq` followed by a letter is part of a
# longer identifier, not the operator plus a delimiter.
use strict; use warnings;

sub qfooq { return "a sub called qfooq" }
print "quote-13 identifier: ", qfooq(), "\n";

my $qq_var = "not the qq operator";
print "quote-13 varname: $qq_var\n";

sub try {
    my $src = shift;
    my $o = `$^X -e '$src' 2>&1`;
    $o =~ s/\s+/ /g; $o =~ s/\s+\z//;
    return $o;
}

# `q` with a letter delimiter: the whole thing is one BAREWORD.
print "quote-13 letter-delim: [", try('my $x = qzfooz; print $x;'), "] (bareword, not q//)\n";
# `q` with a digit delimiter: `q1foo1` is not a legal identifier either.
print "quote-13 digit-delim:  [", try('my $x = q1foo1; print $x;'), "]\n";
# `q` then whitespace then a letter: the letter is not accepted as a delimiter.
print "quote-13 space-letter: [", try('print q foo;'), "]\n";
# `m` with a letter delimiter: `mxfoox` is a bareword, so it is always true.
print "quote-13 m-letter:     [", try('my $x = mxfoox; print $x;'), "] (bareword)\n";

# `_` is a word character too.
print "quote-13 underscore-delim: [", try('my $x = q_foo_; print $x;'), "]\n";

# But a NON-word, non-space character always works, including backslash.
print "quote-13 backslash-delim: [", q\foo\, "]\n";
print "quote-13 quote-delim: [", q'foo', "] [", q"bar", "]\n";

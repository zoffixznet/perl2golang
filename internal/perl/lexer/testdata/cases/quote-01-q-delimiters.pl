#!/usr/bin/perl
# CASE quote-01: `q` with every delimiter family. Matched pairs () {} [] <> nest;
# any other non-word, non-space character is a plain (non-nesting) delimiter.
use strict; use warnings;

my @s = (
  q(paren),
  q{brace},
  q[bracket],
  q<angle>,
  q!bang!,
  q|pipe|,
  q#hash#,
  q,comma,,
  q/slash/,
  q'single',
  q"double",
  q^caret^,
  q:colon:,
  q.dot.,
  q-dash-,
  q+plus+,
);
print "quote-01 count: ", scalar(@s), "\n";
print "quote-01 values: ", join("|", @s), "\n";

# `q` with a backslash delimiter and with a digit-adjacent delimiter.
print "quote-01 misc: ", q&amp&, " ", q%pct%, " ", q~tilde~, "\n";

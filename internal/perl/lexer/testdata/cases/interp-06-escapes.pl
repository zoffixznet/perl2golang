#!/usr/bin/perl
# CASE interp-06: backslash escapes inside "" and qq{}. `\x41`, `\x{263A}`,
# `\0`, `\c[`, `\e`, `\N{U+...}` all have their own sub-grammar with different
# lengths, so the escape scanner is not a simple two-character rule.
use strict; use warnings;
binmode(STDOUT, ':encoding(UTF-8)');

my @pairs = (
  ['\n',        "\n"],
  ['\t',        "\t"],
  ['\r',        "\r"],
  ['\f',        "\f"],
  ['\b',        "\b"],
  ['\a',        "\a"],
  ['\e',        "\e"],
  ['\0',        "\0"],
  ['\x41',      "\x41"],
  ['\x{263A}',  "\x{263A}"],
  ['\x{1F600}', "\x{1F600}"],
  ['\101',      "\101"],
  ['\o{101}',   "\o{101}"],
  ['\c[',       "\c["],
  ['\cA',       "\cA"],
  ['\\\\',      "\\"],
  ['\$',        "\$"],
  ['\@',        "\@"],
  ['\"',        "\""],
  ['\N{U+0041}',"\N{U+0041}"],
);
for my $p (@pairs) {
  printf "interp-06 %-12s -> ord=%s len=%d\n",
         $p->[0], join(",", map { ord } split //, $p->[1]), length($p->[1]);
}

# An unknown escape is just the character itself (with a warning under -w for some).
print "interp-06 unknown-escape: [", "\q" eq "q" ? "backslash dropped" : "kept", "]\n";

# Escapes do NOT happen in single quotes, except \\ and \'.
print "interp-06 single: [", 'a\nb', "] len=", length('a\nb'), "\n";
print "interp-06 single-bs: [", 'a\\b', "] len=", length('a\\b'), "\n";

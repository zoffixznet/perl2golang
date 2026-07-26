#!/usr/bin/perl
# CASE file-05: non-ASCII source with and without `use utf8`. The pragma changes
# how the LEXER decodes the source bytes, so identifier names, string lengths and
# regex semantics all shift. `use utf8` is lexically scoped.
use strict; use warnings;
binmode(STDOUT, ':encoding(UTF-8)');

sub try {
    my ($name, $bytes) = @_;
    open my $fh, '>:raw', $name or die;
    print $fh $bytes;
    close $fh;
    my $o = `$^X $name 2>&1`;
    $o =~ s/\s+\z//;
    return $o;
}

my $eacute = "\xC3\xA9";           # UTF-8 bytes for U+00E9

# Without `use utf8` the two bytes are two characters.
print "file-05 no-pragma: [",
      try("file-05-a.pl", qq{my \$s = "caf$eacute"; print length(\$s);}),
      "] (bytes)\n";

# With `use utf8` they are one character.
print "file-05 with-pragma: [",
      try("file-05-b.pl", qq{use utf8; my \$s = "caf$eacute"; print length(\$s);}),
      "] (characters)\n";

# Non-ASCII IDENTIFIERS require `use utf8`.
my $ident = "\xC3\xA9v";           # "\xE9v" as UTF-8
print "file-05 utf8-identifier: [",
      try("file-05-c.pl", qq{use utf8; my \$$ident = 7; print \$$ident;}),
      "]\n";
print "file-05 non-utf8-identifier: [",
      try("file-05-d.pl", qq{my \$$ident = 7; print \$$ident;}),
      "]\n";

# `use utf8` is lexically scoped: outside the block the bytes are bytes again.
print "file-05 scoped: [",
      try("file-05-e.pl",
          qq{my \$a; { use utf8; \$a = length("$eacute"); } my \$b = length("$eacute"); print "\$a/\$b";}),
      "] (inside/outside)\n";

# A regex character class over a non-ASCII literal.
print "file-05 regex: [",
      try("file-05-f.pl", qq{use utf8; print "caf$eacute" =~ /caf[$eacute]/ ? "match" : "no";}),
      "]\n";

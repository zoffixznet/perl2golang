#!/usr/bin/perl
# CASE file-04: CRLF line endings, a file with no trailing newline, and a UTF-8
# BOM. All three are byte-level facts the lexer meets before any token.
use strict; use warnings;

sub try {
    my ($name, $bytes) = @_;
    open my $fh, '>:raw', $name or die;
    print $fh $bytes;
    close $fh;
    my $o = `$^X $name 2>&1`;
    $o =~ s/\s+/ /g; $o =~ s/\s+\z//;
    return $o;
}

# 1. CRLF endings throughout.
my $crlf = join("", map { "$_\r\n" } (
    'my $x = 1;',
    'my $y = 2;',
    'print "crlf-ok:", $x+$y;',
));
print "file-04 crlf: [", try("file-04-crlf.pl", $crlf), "]\n";

# 2. CRLF inside a heredoc body and inside a multi-line string. Perl NORMALISES
# CRLF to LF while reading the source, so the \r never reaches the string.
my $crlf_hd = join("", map { "$_\r\n" } (
    'my $t = <<"EOT";',
    'body',
    'EOT',
    'print "hd-ords:", join(",", map { ord } split //, $t);',
));
print "file-04 crlf-heredoc: [", try("file-04-crlfhd.pl", $crlf_hd),
      "] (10 = LF, no 13)\n";

my $crlf_str = "my \$s = \"ab\r\ncd\";\r\nprint \"str-ords:\", join(\",\", map { ord } split //, \$s);\r\n";
print "file-04 crlf-in-string: [", try("file-04-crlfstr.pl", $crlf_str), "]\n";

# 3. No trailing newline at end of file.
print "file-04 no-trailing-newline: [",
      try("file-04-nonl.pl", 'print "ends-without-newline";'), "]\n";

# 4. A statement split so the file ends mid-token.
print "file-04 truncated: [",
      try("file-04-trunc.pl", 'print "unterminated'), "]\n";

# 5. UTF-8 BOM at the very start of the file.
print "file-04 bom: [",
      try("file-04-bom.pl", "\xEF\xBB\xBF" . 'print "bom-ok";'), "]\n";

# 6. A lone CR (old Mac) line ending.
print "file-04 cr-only: [",
      try("file-04-cr.pl", "my \$x = 1;\rprint \"cr-ok\";"), "]\n";

#!/usr/bin/perl
# HTML-ish tag stripper with the classic hazards handled in the right
# order: comments, script/style blocks, tags whose attributes contain '>',
# entity decoding, then whitespace normalisation. Also harvests links.
use strict;
use warnings;

my $file = shift @ARGV // 'files/page.html';

my $html = do {
    open my $fh, '<', $file or die "open $file: $!\n";
    local $/;
    <$fh>;
};

# ---- harvest links BEFORE stripping ------------------------------------
my @links;
while (
    $html =~ m{
        <a \s [^>]*? href \s* = \s*
        ( " ([^"]*) " | ' ([^']*) ' )    # double- or single-quoted
        [^>]* > ( .*? ) </a>
    }gsxi
  )
{
    my $url  = defined $2 ? $2 : $3;
    my $text = $4;
    $text =~ s/<[^>]+>//g;               # inner markup like <b>
    $text =~ s/\s+/ /g;
    push @links, [ $text, $url ];
}

# ---- strip, order matters ----------------------------------------------
my $text = $html;

# 1. comments (multi-line, non-greedy)
$text =~ s/<!--.*?-->//gs;

# 2. script/style INCLUDING content ('<' inside them must not confuse us)
$text =~ s{<(script|style)\b[^>]*>.*?</\1\s*>}{}gsi;    # backreference \1

# 3. tags -- attribute values may contain '>' inside quotes, so a naive
#    <[^>]+> would cut data-x="a > b" short. Alternation eats quoted
#    chunks first.
$text =~ s{
    < \s* /? \s* \w+          # open or close tag name
    (?: [^>"'] | "[^"]*" | '[^']*' )*   # attrs: quoted strings may hold '>'
    /? >
}{}gsx;

# 4. entities, named + numeric
my %entity = (
    amp  => '&', lt   => '<', gt    => '>', quot => '"',
    nbsp => ' ', mdash => '--', copy => '(c)',
);
$text =~ s/&#(\d+);/chr $1/ge;                    # numeric, /e evaluates
$text =~ s/&(\w+);/exists $entity{$1} ? $entity{$1} : "&$1;"/ge;

# 5. whitespace normalisation into tidy lines
my @out;
for my $l ( split /\n/, $text ) {
    $l =~ s/^\s+|\s+$//g;
    $l =~ s/\s+/ /g;
    push @out, $l if length $l;
}

print "--- text ---\n";
print "$_\n" for @out;

print "--- links (", scalar @links, ") ---\n";
printf "%-18s => %s\n", @$_ for @links;

# quick integrity checks a converter must preserve
print "--- checks ---\n";
printf "kept angle text: %s\n",
  ( grep { /<special>/ } @out ) ? 'yes' : 'no';
printf "script leaked: %s\n",
  ( grep { /ignore me|document\.write/ } @out ) ? 'yes' : 'no';
printf "comment leaked: %s\n",
  ( grep { /navigation|footer/ } @out ) ? 'yes' : 'no';
printf "attr value 'a > b' leaked: %s\n",
  ( grep { /a > b/ } @out ) ? 'yes' : 'no';

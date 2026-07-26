#!/usr/bin/perl
use strict;
use warnings;

# Substitution in all its usual forms, including /e and the /r
# non-destructive flag, plus tr/// used for translation and counting.

my $path = '/var/log/nginx/access.log.1';

(my $base = $path) =~ s{.*/}{};
print "basename: $base\n";

my $noext = $base;
my $chops = ($noext =~ s/\.\w+$//);
print "stripped '$noext' ($chops substitution)\n";

# s/// without /g replaces once; with /g replaces everywhere.
my $csv = 'a,b,,c,,d';
(my $once = $csv) =~ s/,/;/;
(my $all  = $csv) =~ s/,/;/g;
print "once=$once all=$all\n";

# Return value of s/// is the number of replacements.
my $line = 'ERROR ERROR WARN ERROR';
my $n = ($line =~ s/ERROR/FATAL/g);
print "replaced $n occurrences -> $line\n";

# Backreferences in the replacement.
my $date = '2023-11-14';
(my $us = $date) =~ s/^(\d{4})-(\d{2})-(\d{2})$/$3\/$2\/$1/;
print "reformatted: $us\n";

# /e evaluates the replacement as Perl code.
my $sizes = 'disk=2048 mem=512 swap=1024';
(my $mb = $sizes) =~ s/(\d+)/sprintf('%.1fMiB', $1 \/ 1024)/ge;
print "converted: $mb\n";

my $template = 'Hello {name}, you have {count} messages in {box}.';
my %fields = (name => 'Ada', count => 3, box => 'inbox');
(my $filled = $template) =~ s/\{(\w+)\}/exists $fields{$1} ? $fields{$1} : "<$1?>"/ge;
print "filled: $filled\n";

(my $missing = 'Hi {who}, see {nothing}.') =~ s/\{(\w+)\}/exists $fields{$1} ? $fields{$1} : "<$1?>"/ge;
print "missing: $missing\n";

# /r returns the modified copy and leaves the original alone.
my $orig = '  padded  ';
my $trimmed = $orig =~ s/^\s+|\s+$//gr;
print "orig=[$orig] trimmed=[$trimmed]\n";

# Case folding in a substitution using \u and \U.
my $title = join ' ', map { s/(\w+)/\u\L$1/r } split ' ', 'gRACE hopper WROTE compilers';
print "title: $title\n";

# tr/// translating characters.
my $dna = 'GATTACA';
(my $comp = $dna) =~ tr/ACGT/TGCA/;
print "complement of $dna is $comp\n";

# tr/// counting without modifying (empty replacement list).
my $sentence = 'The quick brown fox jumps over the lazy dog';
my $vowels = ($sentence =~ tr/aeiouAEIOU//);
my $spaces = ($sentence =~ tr/ //);
my $alpha  = ($sentence =~ tr/a-zA-Z//);
print "vowels=$vowels spaces=$spaces letters=$alpha\n";

# tr with /d to delete and /s to squeeze runs.
(my $digits_only = 'a1b22c333') =~ tr/0-9//cd;
print "digits only: $digits_only\n";
(my $squeezed = 'aaabbbcccaaa') =~ tr/a-z//s;
print "squeezed: $squeezed\n";
(my $upper = 'mixed Case Text') =~ tr/a-z/A-Z/;
print "upper: $upper\n";

# Chained substitutions in a normalising sub.
sub slug {
    my ($s) = @_;
    $s = lc $s;
    $s =~ s/[^a-z0-9]+/-/g;
    $s =~ s/^-+|-+$//g;
    $s =~ s/-{2,}/-/g;
    return $s;
}
print "slug: ", slug('  Hello, World!! -- Again  '), "\n";
print "slug: ", slug('perl2go: A Converter (v2)'), "\n";

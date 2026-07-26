#!/usr/bin/perl
# CASE interp-09: `"user@example.com"` -- the `@example` is an array
# interpolation. Under `use strict` it is a COMPILE ERROR; without strict it is
# an empty string plus a warning. This is the single most common real-world
# string-lexing surprise.
use strict; use warnings;

sub child {
    my ($name, $src) = @_;
    open my $fh, '>', $name or die;
    print $fh $src;
    close $fh;
    my $o = `$^X $name 2>&1`;
    $o =~ s/\s+/ /g; $o =~ s/\s+\z//;
    return $o;
}

print "interp-09 strict: [",
  child("interp-09-a.pl", qq{use strict; use warnings;\nprint "you\@example.com";\n}),
  "]\n";

print "interp-09 no-strict: [",
  child("interp-09-b.pl", qq{use warnings;\nprint "you\@example.com";\n}),
  "]\n";

# The correct spellings.
print "interp-09 escaped: [", "you\@example.com", "]\n";
print "interp-09 single-quoted: [", 'you@example.com', "]\n";
print "interp-09 concatenated: [", "you" . '@' . "example.com", "]\n";

# `@` followed by something that cannot start a name is literal even in "".
print "interp-09 at-digit: [", "10@5", "]\n";
print "interp-09 at-space: [", "a @ b", "]\n";
print "interp-09 at-punct: [", "a @. b", "]\n";

#!/usr/bin/perl
# CASE sigil-02: `$$x[0]` means `${$x}[0]` (subscript binds to the INNER deref),
# which is the same as `$x->[0]`. `${$x[0]}` is a completely different thing.
use strict; use warnings;

my @arr = ("A","B");
my $aref = \@arr;
my @of_refs = (\"zero", \"one");

print "sigil-02 dollardollar: ", $$aref[0], "\n";
print "sigil-02 braced:       ", ${$aref}[0], "\n";
print "sigil-02 arrow:        ", $aref->[0], "\n";
print "sigil-02 all-equal: ",
      ($$aref[0] eq ${$aref}[0] && ${$aref}[0] eq $aref->[0] ? "yes" : "no"), "\n";

# ${$x[0]} derefs the ELEMENT, not the array ref.
print "sigil-02 deref-element: ", ${$of_refs[0]}, "\n";

my %h = (k => "vee");
my $href = \%h;
print "sigil-02 hash: ", $$href{k}, " / ", ${$href}{k}, " / ", $href->{k}, "\n";

# Chained subscripts: the arrow is optional BETWEEN subscripts.
my $deep = { a => [ { b => "found" } ] };
print "sigil-02 chained: ", $deep->{a}[0]{b}, " / ", $$deep{a}[0]{b}, "\n";

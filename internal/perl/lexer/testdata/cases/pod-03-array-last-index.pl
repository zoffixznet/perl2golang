#!/usr/bin/perl
# CASE pod-03: `$#` is a SIGIL, not a comment. `$#array`, `$#{$ref}`, `$#$ref`,
# and `$#` alone (the deprecated output-format variable, removed) all start with
# the same two characters that would otherwise begin a comment.
use strict; use warnings;

my @a = (10,20,30);
my $r = \@a;

print "pod-03 lastindex: $#a\n";
print "pod-03 deref-brace: ", $#{$r}, "\n";
print "pod-03 deref-bare: ",  $#$r,  "\n";
print "pod-03 postfix: ",     $r->$#*, "\n";

# In a string, $#a interpolates.
print "pod-03 interp: [$#a] and [@{[ $#{$r} ]}]\n";

# Assigning to $#a truncates the array.
$#a = 1;
print "pod-03 truncated: @a\n";

# Package-qualified form.
@Foo::list = (1,2,3,4);
print "pod-03 qualified: $#Foo::list\n";

# A real comment starting right after a $# expression.
my $n = $#a;   # this trailing text is a comment
print "pod-03 after-comment: $n\n";

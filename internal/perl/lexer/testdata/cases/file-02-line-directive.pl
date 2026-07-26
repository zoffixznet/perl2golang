#!/usr/bin/perl
# CASE file-02: `# line NNN "FILE"` is a COMPILER DIRECTIVE disguised as a
# comment. It rewrites __LINE__/__FILE__ and warning/error locations from the
# next line on. It must be at column 0 and match a specific shape.
use strict; use warnings;

print "file-02 before: line=", __LINE__, " file=", (__FILE__ =~ m{([^/]+)\z})[0], "\n";
# line 42 "virtual.pl"
print "file-02 after: line=", __LINE__, " file=", __FILE__, "\n";
#line 100 "second.pl"
print "file-02 no-space-form: line=", __LINE__, " file=", __FILE__, "\n";

# An ordinary comment that merely mentions the word line changes nothing.
# this line 999 "nope.pl" is just prose
print "file-02 ordinary-comment: line=", __LINE__, " file=", __FILE__, "\n";

# Indented, so NOT a directive.
   # line 777 "indented.pl"
print "file-02 indented-not-directive: line=", __LINE__, " file=", __FILE__, "\n";

# The directive also relocates warnings.
my $w = "";
{
  local $SIG{__WARN__} = sub { $w = $_[0] };
# line 500 "warned.pl"
  my $u; my $x = $u + 1;
}
$w =~ s/\s+\z//;
print "file-02 warning-location: [$w]\n";

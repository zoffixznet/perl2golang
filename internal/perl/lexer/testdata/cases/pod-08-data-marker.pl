#!/usr/bin/perl
# CASE pod-08: `__DATA__` behaves like `__END__` but attaches the DATA handle to
# the CURRENTLY COMPILING package, which is why the handle below is Widget::DATA
# and not main::DATA.
use strict; use warnings;
no warnings 'once';

package Widget;

print "pod-08 main-DATA-open: ", (defined(fileno(*main::DATA)) ? "yes" : "no"), "\n";
print "pod-08 widget-DATA-open: ", (defined(fileno(*Widget::DATA)) ? "yes" : "no"), "\n";

my @l = <Widget::DATA>;
chomp @l;
print "pod-08 lines: ", scalar(@l), " [", join("|", @l), "]\n";

__DATA__
widget-one
widget-two

#!/usr/bin/perl
# CASE pod-10: a STRAY `=cut` (no POD block open) is not a no-op -- it OPENS a POD
# skip that runs to the next `=cut` or to EOF. Code between two stray `=cut`s
# silently disappears. Also `=` followed by a non-identifier is not POD.
use strict; use warnings;

print "pod-10 start\n";

# Everything from a stray `=cut` to the NEXT `=cut` is swallowed.
sub write_child {
    my ($name, $src) = @_;
    open my $fh, '>', $name or die;
    print $fh $src;
    close $fh;
    my $o = `$^X $name 2>&1`;
    $o =~ s/\s+/ /g; $o =~ s/\s+\z//;
    return $o;
}

print "pod-10 stray-cut-pair: [",
      write_child("pod-10-child1.pl", qq{print "a\\n";\n\n=cut\n\nprint "b\\n";\n\n=cut\n\nprint "c\\n";\n}),
      "] (b is swallowed)\n";

print "pod-10 stray-cut-to-eof: [",
      write_child("pod-10-child2.pl", qq{print "a\\n";\n=cut\nprint "b\\n";\n}),
      "] (b is swallowed)\n";

print "pod-10 equals-digit: [",
      write_child("pod-10-child3.pl", qq{print "a";\n=1 not pod\nprint "b";\n}),
      "]\n";

print "pod-10 equals-space: [",
      write_child("pod-10-child4.pl", qq{my \$x\n= 5;\nprint "got \$x";\n}),
      "] (a leading `= ` is an operator, not POD)\n";

print "pod-10 end\n";

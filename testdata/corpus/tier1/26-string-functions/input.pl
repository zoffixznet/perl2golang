use strict;
use warnings;

my $s = "Hello, World";
print "length:   ", length($s), "\n";
print "uc:       ", uc($s), "\n";
print "lc:       ", lc($s), "\n";
print "ucfirst:  ", ucfirst(lc $s), "\n";
print "lcfirst:  ", lcfirst($s), "\n";

print "--- index and rindex ---\n";
print "index 'o':        ", index($s, "o"), "\n";
print "index 'o' from 5: ", index($s, "o", 5), "\n";
print "rindex 'o':       ", rindex($s, "o"), "\n";
print "rindex 'o' to 5:  ", rindex($s, "o", 5), "\n";
print "index 'zz':       ", index($s, "zz"), "\n";
print "index '':         ", index($s, ""), "\n";
print "starts with Hello: ", (index($s, "Hello") == 0 ? "yes" : "no"), "\n";
print "contains World:    ", (index($s, "World") >= 0 ? "yes" : "no"), "\n";

print "--- ord and chr ---\n";
print "ord('A')=", ord("A"), " chr(66)=", chr(66), " ord('')=", ord(""), "\n";

print "--- repeat, reverse, split, join ---\n";
print "repeat:   ", "ab" x 3, "\n";
print "reverse:  ", scalar reverse($s), "\n";
print "chars:    ", join("|", split(//, "abc")), "\n";
print "rejoined: ", join("", split(//, "abc")), "\n";

print "--- padding without sprintf ---\n";
my $label = "id";
print "[", $label . " " x (8 - length $label), "]\n";
print "[", " " x (8 - length $label) . $label, "]\n";

print "--- case-insensitive compare ---\n";
for my $pair ("Apple:apple", "Apple:Banana", "b:A") {
    my ($x, $y) = split /:/, $pair;
    printf "%-14s cmp=%2d  lc cmp=%2d\n", $pair, ($x cmp $y), (lc $x cmp lc $y);
}

print "--- lc/uc are not involutive on already-cased text ---\n";
print uc(lc("MiXeD")), " ", lc(uc("MiXeD")), "\n";

use strict;
use warnings;

my $s = "The quick brown fox";
print "from 4:        '", substr($s, 4), "'\n";
print "4 for 5:       '", substr($s, 4, 5), "'\n";
print "last 3:        '", substr($s, -3), "'\n";
print "-9 for 5:      '", substr($s, -9, 5), "'\n";
print "0 to -4:       '", substr($s, 0, -4), "'\n";
print "beyond end:    '", substr($s, 4, 100), "'\n";
print "zero length:   '", substr($s, 4, 0), "'\n";

print "--- 4-argument replacement ---\n";
my $u = $s;
my $old = substr($u, 4, 5, "lazy");
print "replaced '$old' with 'lazy' -> $u\n";

my $v = $s;
substr($v, 0, 0, ">> ");
print "prefixed:      $v\n";
substr($v, length($v), 0, " <<");
print "suffixed:      $v\n";

print "--- lvalue substr ---\n";
my $t = $s;
substr($t, 4, 5) = "slow!";
print "lvalue assign: $t\n";
substr($t, -3) = "cat";
print "tail assign:   $t\n";

print "--- fixed width field extraction ---\n";
my $rec = "20240115ACME     0042";
printf "date=%s-%s-%s name=%s qty=%d\n",
    substr($rec, 0, 4), substr($rec, 4, 2), substr($rec, 6, 2),
    substr($rec, 8, 8), substr($rec, 16, 4);

print "--- building fixed width output ---\n";
for my $pair ("ab:7", "abcdefgh:1234") {
    my ($name, $qty) = split /:/, $pair;
    my $field = $name . (" " x 10);
    print "[", substr($field, 0, 10), "][", substr("00000" . $qty, -5), "]\n";
}

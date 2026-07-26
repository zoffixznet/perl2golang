use strict;
use warnings;

my $i = 5;
my $post = $i++;
print "post-inc returned $post, i is now $i\n";
my $pre = ++$i;
print "pre-inc returned $pre, i is now $i\n";
my $postdec = $i--;
print "post-dec returned $postdec, i is now $i\n";
my $predec = --$i;
print "pre-dec returned $predec, i is now $i\n";

print "--- magic string autoincrement ---\n";
my $s = "aa";
for (1 .. 3) { $s++ }
print "aa incremented 3 times: $s\n";
for my $start (qw(Az zz a9 Zz9 ID001 aa9)) {
    my $t = $start;
    $t++;
    print "$start ++ -> $t\n";
}
my $id = "ID001";
$id++ for 1 .. 5;
print "ID001 after 5 increments: $id\n";

print "--- compound assignment ---\n";
my $n = 9;
$n += 1;  print "+= 1  -> $n\n";
$n -= 3;  print "-= 3  -> $n\n";
$n *= 4;  print "*= 4  -> $n\n";
$n /= 2;  print "/= 2  -> $n\n";
$n **= 2; print "**= 2 -> $n\n";
$n %= 7;  print "%= 7  -> $n\n";
my $str = "ab";
$str .= "cd";
print "concat assign: $str\n";
$str x= 2;
print "repeat assign: $str\n";

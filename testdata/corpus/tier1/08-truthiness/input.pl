use strict;
use warnings;

print "--- strings ---\n";
for my $v ("0", "", "0.0", "00", "0E0", " ", "false", "0 but true", "\n") {
    my $shown = $v;
    $shown =~ tr/\n/N/;
    printf "%-14s => %s\n", "'$shown'", ($v ? "true" : "false");
}

print "--- numbers ---\n";
for my $n (0, 1, -1, 0.0, 0.1, -0.0) {
    printf "%-14s => %s\n", $n, ($n ? "true" : "false");
}

print "--- containers ---\n";
my @empty = ();
my @falsy = (0);
print "empty array          => ", (@empty ? "true" : "false"), "\n";
print "array holding a 0    => ", (@falsy ? "true" : "false"), "\n";
my %eh;
my %fh = (k => 0);
print "empty hash           => ", (%eh ? "true" : "false"), "\n";
print "hash with one key    => ", (%fh ? "true" : "false"), "\n";
my $u;
print "undef                => ", ($u ? "true" : "false"), "\n";

print "--- what a comparison actually returns ---\n";
my $t = (1 == 1);
my $f = (1 == 2);
printf "true is  '%s' (length %d)\n", $t, length($t);
printf "false is '%s' (length %d)\n", $f, length($f);
print "false + 1 = ", $f + 1, "\n";

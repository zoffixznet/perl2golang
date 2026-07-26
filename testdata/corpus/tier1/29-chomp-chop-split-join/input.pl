use strict;
use warnings;

print "--- chomp and chop ---\n";
my $withnl = "hello\n";
chomp(my $clean = $withnl);
print "chomp: [$clean] length ", length($clean), "\n";
my $nonl = "hello";
my $removed = chomp(my $copy = $nonl);
print "chomp on a line with no newline removed $removed chars\n";
my $t = "hello";
my $ch = chop $t;
print "chop removed '$ch' leaving '$t'\n";
my @lines = ("a\n", "b\n", "c");
chomp @lines;
print "chomped list: @lines\n";

print "--- split shapes ---\n";
print join("|", split(/,/, "a,b,c")), "\n";
my @trailing = split(/,/, "a,b,,c,,");
print "trailing empties dropped: ", scalar(@trailing), " -> ", join("|", @trailing), "\n";
my @kept = split(/,/, "a,b,,c,,", -1);
print "limit -1 keeps them: ", scalar(@kept), " -> ", join("|", @kept), "\n";
print "limit 2:  ", join("|", split(/ /, "one two three", 2)), "\n";
print "empty pattern: ", join("|", split(//, "xyz")), "\n";
print "regex \\s+: [", join("][", split(/\s+/, "  lead and trail  ")), "]\n";
print "literal ' ': [", join("][", split(' ', "  lead and trail  ")), "]\n";

print "--- join ---\n";
my @f = qw(2024 01 15);
print join("-", @f), "\n";
print join("", @f), "\n";
print join(", ", "solo"), "\n";
print "[", join(",", ()), "]\n";

print "--- round trip over STDIN ---\n";
my $lineno = 0;
while (my $line = <STDIN>) {
    chomp $line;
    $lineno++;
    my @field = split /,/, $line, -1;
    printf "%d: %d fields -> %s\n", $lineno, scalar(@field), join(" / ", @field);
}
print "read $lineno lines\n";

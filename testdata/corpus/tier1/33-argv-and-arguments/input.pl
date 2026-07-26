use strict;
use warnings;

print "argc: ", scalar(@ARGV), "\n";
for my $i (0 .. $#ARGV) {
    printf "argv[%d] = [%s] (%d chars)\n", $i, $ARGV[$i], length($ARGV[$i]);
}

print "--- \$0 is the script name, not an argument ---\n";
print "script name is a string: ", (length($0) > 0 ? "yes" : "no"), "\n";

print "--- consuming arguments ---\n";
my @rest = @ARGV;
my $cmd = shift @rest;
print "command: $cmd\n";
print "remaining: ", join(",", @rest), "\n";

print "--- a flag-and-value scan ---\n";
my %opt = (verbose => 0, name => "default");
my @positional;
my @scan = @ARGV;
while (@scan) {
    my $arg = shift @scan;
    if    ($arg eq "-v")     { $opt{verbose} = 1 }
    elsif ($arg eq "--name") { $opt{name} = shift(@scan) // "" }
    else                     { push @positional, $arg }
}
for my $k (sort keys %opt) { print "opt $k = $opt{$k}\n" }
print "positional: ", join("|", @positional), "\n";

print "--- numeric use of an argument ---\n";
my ($n) = grep { /^[0-9]+$/ } @ARGV;
$n = 0 unless defined $n;
print "first all-digit argument as a number: ", $n + 0, "\n";
print "doubled: ", $n * 2, "\n";

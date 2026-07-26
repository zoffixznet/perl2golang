use strict;
use warnings;

my $i = 0;
while ($i < 5) {
    print "while $i\n";
    $i++;
}

print "--- until ---\n";
my $j = 5;
until ($j <= 0) {
    print "until $j\n";
    $j -= 2;
}

print "--- do-while runs the body first ---\n";
my $k = 0;
do {
    print "do-while $k\n";
    $k++;
} while ($k < 3);
my $never = 0;
do {
    print "body ran once despite a false condition\n";
    $never++;
} while (0);
print "never=$never\n";

print "--- draining with an explicit defined test ---\n";
my @queue = (3, 0, 7);
while (defined(my $item = shift @queue)) {
    print "item $item\n";
}

print "--- the trap that defined() avoids ---\n";
my @q2 = (3, 0, 7);
my @taken;
while (my $item = shift @q2) {
    push @taken, $item;
}
print "truthiness-driven loop took @taken and left @q2\n";

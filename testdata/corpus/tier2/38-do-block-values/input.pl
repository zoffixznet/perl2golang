#!/usr/bin/perl
# do BLOCK is a term, not a statement: the block runs and hands back the
# value of the last thing it evaluated. Every shape a real script uses it in
# is here, including the ones where the block must not run unconditionally.
use strict;
use warnings;

# ---- setup then value --------------------------------------------------
my $config = do {
    my %defaults = (retries => 3, timeout => 30);
    $defaults{timeout} *= 2;
    join ',', map { "$_=$defaults{$_}" } sort keys %defaults;
};
print "config: $config\n";

# ---- conditional value, the if/elsif/else chain as a term --------------
for my $n (5, 50, 500) {
    my $bucket = do {
        if    ($n < 10)  { 'tiny' }
        elsif ($n < 100) { 'medium' }
        else             { 'huge' }
    };
    print "n=$n bucket=$bucket\n";
}

# ---- a block whose value is a list -------------------------------------
my @evens = do {
    my @out;
    for my $i (1 .. 10) {
        push @out, $i if $i % 2 == 0;
    }
    @out;
};
print "evens: @evens\n";

# ---- `or do` as the failure arm; the block must not run when the left
#      side is true, which is the whole point of the idiom ---------------
my %seen;
my @words = qw(alpha beta alpha gamma beta alpha);
my @firsts;
for my $w (@words) {
    $seen{$w}++ or do {
        push @firsts, $w;
        next;
    };
}
print "firsts: @firsts\n";
print "counts: ", join(' ', map { "$_=$seen{$_}" } sort keys %seen), "\n";

# ---- `and do` is the same shape the other way round --------------------
my $verbose = 0;
$verbose and do { print "this line never runs\n" };
print "still here\n";

# ---- a do block inside a sub, returning through it ---------------------
sub classify {
    my ($size) = @_;
    return do {
        my $kb = $size / 1024;
        $kb < 1 ? sprintf('%d B', $size) : sprintf('%.1f KB', $kb);
    };
}
print classify($_), "\n" for 512, 2048, 1048576;

# ---- nested do blocks --------------------------------------------------
my $nested = do {
    my $inner = do { 6 * 7 };
    "inner was $inner";
};
print "$nested\n";

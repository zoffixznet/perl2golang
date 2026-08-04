#!/usr/bin/perl
use strict;
use warnings;

# A substitution that both edits the string and hands back what it removed.
# The parser shape: strip the field off the front, keep it, carry on with
# what is left.

my @lines = (
    'web1     A     10.0.0.1',
    '         AAAA  fd00::1',
    'db1      CNAME web1',
    '   MX 10 mail1',
);

my $prev_owner = '';
my %seen;
my @records;

for my $line (@lines) {
    my $owner;
    if ( $line =~ s/^(\S+)\s+// ) {
        $owner      = $1;
        $prev_owner = $owner;
    }
    else {
        $line =~ s/^\s+//;
        $owner = $prev_owner;
    }
    my ( $type, $rest ) = split ' ', $line, 2;
    push @{ $seen{$owner}{$type} }, $rest;
    push @records, "$owner $type $rest";
}

print "--- records ---\n";
print "$_\n" for @records;

print "--- by owner ---\n";
for my $owner ( sort keys %seen ) {
    for my $type ( sort keys %{ $seen{$owner} } ) {
        printf "  %-6s %-5s %d\n", $owner, $type, scalar @{ $seen{$owner}{$type} };
    }
}

# The same shape with two groups, and with the groups read after the edit.
my $range = 'ports 8000-8100 open';
if ( $range =~ s/(\d+)-(\d+)\s*// ) {
    printf "from %d to %d, leaving '%s'\n", $1, $2, $range;
}

# A substitution whose pattern has no groups still answers with a count.
my $noisy = 'a,b,,c,,d';
my $gone  = ( $noisy =~ s/,,/,/g );
printf "collapsed %d doubled comma(s): %s\n", $gone, $noisy;

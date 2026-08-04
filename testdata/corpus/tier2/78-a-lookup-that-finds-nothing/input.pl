#!/usr/bin/perl
use strict;
use warnings;

# A lookup that can fail, in a table where nothing was ever set to undef. Perl
# answers with undef for a key that is not there and the caller tests defined;
# the hash itself gives no hint that a value might be missing.

my %port = ( http => 80, https => 443, echo => 0 );
my %owner = ( http => 'web', https => 'web' );

sub port_of {
    my ($name) = @_;
    return $port{$name};
}

sub owner_of {
    my ($name) = @_;
    return $owner{$name};
}

print "--- a numeric lookup with a real zero in it ---\n";
for my $name (qw(http echo gopher)) {
    my $p = port_of($name);
    printf "%-7s %s\n", $name, ( defined $p ? "port $p" : 'not listed' );
}

print "--- a text lookup where the answer can be empty ---\n";
for my $name (qw(http gopher)) {
    my $o = owner_of($name);
    printf "%-7s %s\n", $name, ( defined $o ? "owned by $o" : 'unowned' );
}

print "--- the caller can ask a different question ---\n";
for my $name (qw(echo gopher)) {
    printf "%-7s exists=%s defined=%s\n", $name,
        ( exists $port{$name} ? 'yes' : 'no' ),
        ( defined $port{$name} ? 'yes' : 'no' );
}

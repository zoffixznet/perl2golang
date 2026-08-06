#!/usr/bin/perl
# Invoice formatter: every pattern here ends in a lookaround. Amounts get
# thousands separators, US country codes are stripped only when a full
# number follows, route paths are normalised, and hidden entries are the
# dotfiles that are not . or .. themselves.
use strict;
use warnings;

sub commify {
    my ($n) = @_;
    my $s = reverse "$n";
    $s =~ s/(\d{3})(?=\d)/$1,/g;
    return scalar reverse $s;
}
print "total: ", commify(1234567), " cents\n";
print "small: ", commify(999), " cents\n";

for my $phone ( '15551234567', '5551234567', '155512345678' ) {
    ( my $p = $phone ) =~ s/^1(?=\d{10}$)//;
    print "dial $p\n";
}

for my $path ( '/api/users/123', '/api/users/123/orders/45', '/health' ) {
    ( my $route = $path ) =~ s{/\d+(?=/|$)}{/:id}g;
    print "$path -> $route\n";
}

for my $entry ( '.', '..', '.git', '..lock', 'README' ) {
    my $hidden = $entry =~ /^\.(?!\.?$)/ ? 'hidden' : 'listed';
    printf "%-7s %s\n", $entry, $hidden;
}

# The substitution's value is how many replacements it made.
my $csv = "a5b23c105d";
my $hits = ( $csv =~ s/\d+(?=[a-z])/#/g );
print "masked $hits runs: $csv\n";

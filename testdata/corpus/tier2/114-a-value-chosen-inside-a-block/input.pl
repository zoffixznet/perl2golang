#!/usr/bin/perl
use strict;
use warnings;

# Perl blocks have values, so a choice can be written where a value belongs
# and the branch that runs is the answer. Three places that happens, all of
# them ordinary in installed module code and none of them written with a
# `return` in sight.

my %scheme = ( utf8 => 'from_to', latin1 => 'perlio' );

# A do block whose value is a conditional, with a second conditional nested
# inside one of its branches.
for my $want ( 'utf8', 'koi8-r', undef ) {
    my $chosen = do {
        if ( defined $want ) {
            if ( exists $scheme{$want} ) { $scheme{$want} }
            else                         { 'fallback' }
        }
        else { 'default' }
    };
    printf "%-8s -> %s\n", ( $want // 'undef' ), $chosen;
}

# A map block whose value is a conditional.
my @words = ( 'a', 'bb', 'ccc' );
my @shown = map {
    if ( length($_) > 1 ) { uc $_ }
    else                  { "'$_'" }
} @words;
print "shown: @shown\n";

# A sub whose last statement is an anonymous sub, which is the closure the
# caller gets back.
sub counter_from {
    my ($start) = @_;
    my $n = $start;
    sub { $n++ };
}

my $next = counter_from(10);
printf "counter: %d %d %d\n", $next->(), $next->(), $next->();

# A grep block whose test is a conditional, for the same reason.
my @kept = grep {
    if   ( $_ eq 'bb' ) { 0 }
    else                { 1 }
} @words;
print "kept: @kept\n";

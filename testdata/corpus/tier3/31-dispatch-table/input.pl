#!/usr/bin/perl
# A hash of code refs used as a dispatch table, which is how Perl writes a
# switch over a set of named actions. The handlers deliberately disagree about
# what they return and how many arguments they take, because that is what a
# real table looks like and it is exactly what a single Go signature cannot
# hold.
use strict;
use warnings;

my @audit;

my %handlers = (
    # returns a string
    greet => sub { my ($name) = @_; "hello, $name" },
    # returns a number
    count => sub { my ($text) = @_; scalar( () = $text =~ /\S+/g ) },
    # returns a list, which the caller reads in scalar context
    parts => sub { my ($text) = @_; split /,/, $text },
    # returns nothing at all, and works by side effect
    note => sub { push @audit, $_[0]; return },
    # takes two arguments where the others take one
    pad => sub { my ( $text, $width ) = @_; sprintf '%-*s|', $width, $text },
);

for my $name ( sort keys %handlers ) {
    printf "%-6s exists=%s\n", $name, ( ref $handlers{$name} eq 'CODE' ? 'yes' : 'no' );
}

print "-- calling through the table --\n";
printf "greet: %s\n", $handlers{greet}->('world');
printf "count: %s\n", $handlers{count}->('one two  three');
printf "parts: %s\n", scalar( $handlers{parts}->('a,b,c') );
printf "pad:   %s\n", $handlers{pad}->( 'left', 8 );

$handlers{note}->('first');
$handlers{note}->('second');
printf "audit: %s\n", join( ' ', @audit );

# The table is also walked as data: every entry called with one argument, and
# whatever comes back turned into text.
print "-- walked --\n";
for my $name (qw(greet count parts)) {
    my $out = $handlers{$name}->('x,y');
    printf "%-6s -> %s\n", $name, defined $out ? $out : '(undef)';
}

# A handler chosen by a name computed at run time, which is the whole point of
# the table over an if/elsif chain.
my $which = ( 'g' . 'reet' );
printf "computed: %s\n", $handlers{$which}->('again');

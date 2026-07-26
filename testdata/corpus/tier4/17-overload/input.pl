#!/usr/bin/perl
# TRAP: operator overloading -- '+', '*', '""', '==' on these objects are
# method calls in disguise. Go has no operator overloading at all.
use strict;
use warnings;

package Money;
use overload
    '+'  => sub { Money->new( $_[0]{c} + $_[1]{c} ) },
    '*'  => sub { my ( $s, $n ) = @_; Money->new( $s->{c} * $n ) },
    '""' => sub { sprintf '$%.2f', $_[0]{c} / 100 },
    '==' => sub { $_[0]{c} == ( ref $_[1] ? $_[1]{c} : $_[1] * 100 ) };

sub new { my ( $c, $cents ) = @_; return bless { c => $cents }, $c }

package main;

my $a   = Money->new(150);
my $b   = Money->new(250);
my $sum = $a + $b;                  # method call in disguise
print "sum: $sum\n";                # stringification is overloaded too
print "tripled: ", $a * 3, "\n";
print "equal: ", ( ( $sum == 4.00 ) ? "yes" : "no" ), "\n";

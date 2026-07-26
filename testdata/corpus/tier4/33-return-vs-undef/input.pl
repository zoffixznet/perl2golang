#!/usr/bin/perl
# TRAP: `return;` yields an EMPTY LIST in list context (zero elements);
# `return undef;` yields a ONE-element list. In a hash constructor the
# empty list SHIFTS every following pair. Identical in scalar context.
use strict;
use warnings;

sub bare       { return; }
sub undefined_ { return undef; }

my @a = bare();
my @b = undefined_();
print "bare in list:  ", scalar(@a), " elements\n";    # 0
print "undef in list: ", scalar(@b), " elements\n";    # 1

no warnings 'misc';    # silence the odd-elements warning
my %h = ( name => bare(), age => 30 );    # list collapses: name=>"age", 30=>undef
print "keys:  ", join( ",", sort keys %h ), "\n";

my %h2 = ( name => undefined_(), age => 30 );          # what was intended
print "keys2: ", join( ",", sort keys %h2 ), "\n";

my $s1 = bare();
my $s2 = undefined_();
print "scalar ctx: ", ( ( !defined $s1 && !defined $s2 ) ? "both undef" : "differ" ), "\n";

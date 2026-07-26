#!/usr/bin/perl
# 3-D vector class blessed from an ARRAY ref, with heavy operator overloading.
use strict;
use warnings;

package Vec3;
use overload
  '+'   => \&add,
  '-'   => \&sub_,
  '*'   => \&mul,          # vector * scalar, or dot product for two vectors
  '=='  => \&equals,
  '!='  => sub { !equals(@_) },
  '<=>' => \&compare,      # compares by magnitude
  '""'  => \&to_string,
  'bool' => sub { my $s = shift; $s->magnitude > 0 },
  'neg' => sub { my $s = shift; Vec3->new( map { -$_ } @$s ) };

sub new {
    my ( $class, @xyz ) = @_;
    die "Vec3 wants 3 components, got " . scalar(@xyz) . "\n"
      unless @xyz == 3;
    return bless [@xyz], $class;    # ARRAY-backed object, not a hashref
}

sub x { $_[0][0] }
sub y { $_[0][1] }
sub z { $_[0][2] }

sub add {
    my ( $a, $b ) = @_;
    return Vec3->new( map { $a->[$_] + $b->[$_] } 0 .. 2 );
}

sub sub_ {
    my ( $a, $b, $swapped ) = @_;
    ( $a, $b ) = ( $b, $a ) if $swapped;
    return Vec3->new( map { $a->[$_] - $b->[$_] } 0 .. 2 );
}

sub mul {
    my ( $a, $b ) = @_;
    if ( ref $b && $b->isa('Vec3') ) {    # dot product
        my $dot = 0;
        $dot += $a->[$_] * $b->[$_] for 0 .. 2;
        return $dot;
    }
    return Vec3->new( map { $_ * $b } @$a );    # scale
}

sub equals {
    my ( $a, $b ) = @_;
    return 0 unless ref $b && $b->isa('Vec3');
    $a->[$_] == $b->[$_] or return 0 for 0 .. 2;
    return 1;
}

sub magnitude {
    my ($s) = @_;
    return sqrt( $s->[0]**2 + $s->[1]**2 + $s->[2]**2 );
}

sub compare {
    my ( $a, $b, $swapped ) = @_;
    my $r = $a->magnitude <=> $b->magnitude;
    return $swapped ? -$r : $r;
}

sub to_string {
    my ($s) = @_;
    return sprintf '(%g, %g, %g)', @$s;
}

package main;

my $i = Vec3->new( 1, 0, 0 );
my $j = Vec3->new( 0, 1, 0 );
my $k = Vec3->new( 0, 0, 1 );
my $v = Vec3->new( 3, -4, 12 );

# Overloaded stringification kicks in inside interpolation.
print "i=$i j=$j v=$v\n";
print "i+j = ", $i + $j, "\n";
print "v-i = ", $v - $i, "\n";
print "-v  = ", -$v, "\n";
print "v*2 = ", $v * 2, "\n";
print "2*v = ", 2 * $v, "\n";       # swapped-operand path
print "v.i (dot) = ", $v * $i, "\n";
print "v.v (dot) = ", $v * $v, "\n";
printf "|v| = %.3f\n", $v->magnitude;

# Overloaded comparison drives sort and equality.
my @vecs = ( $v, $i, $i + $j, $k * 5, Vec3->new( 1, 0, 0 ) );
my @sorted = sort { $a <=> $b } @vecs;
print "sorted by magnitude: @sorted\n";

print "i == copy: ",   ( $i == $vecs[4]        ? 'yes' : 'no' ), "\n";
print "i == j:    ",   ( $i == $j              ? 'yes' : 'no' ), "\n";
print "i != j:    ",   ( $i != $j              ? 'yes' : 'no' ), "\n";
print "zero truthy: ", ( Vec3->new( 0, 0, 0 ) ? 'yes' : 'no' ), "\n";
print "v truthy:    ", ( $v                    ? 'yes' : 'no' ), "\n";

# Chained arithmetic expression exercising several overloads at once.
my $r = ( $v + $i * 2 ) - -$j;
print "chained: $r\n";
printf "max magnitude: %s\n", ( sort { $b <=> $a } @vecs )[0];

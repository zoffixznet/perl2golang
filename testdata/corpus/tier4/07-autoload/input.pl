#!/usr/bin/perl
# TRAP: AUTOLOAD -- methods that are INVENTED at call time by parsing the
# method name. The set of callable methods is unbounded.
use strict;
use warnings;

package Record;
our $AUTOLOAD;
sub new { my ( $c, %f ) = @_; return bless {%f}, $c }

sub AUTOLOAD {
    my $self = shift;
    my $name = $AUTOLOAD;
    $name =~ s/.*:://;
    return if $name eq 'DESTROY';
    if ( $name =~ /^get_(\w+)$/ ) { return $self->{$1} }
    if ( $name =~ /^set_(\w+)$/ ) { $self->{$1} = shift; return }
    die "no such method: $name\n";
}

package main;

my $r = Record->new( color => "red", size => 3 );
print "color=", $r->get_color, "\n";    # no get_color sub exists anywhere
$r->set_size(5);
print "size=", $r->get_size, "\n";
eval { $r->launch_missiles };
print "error: $@";

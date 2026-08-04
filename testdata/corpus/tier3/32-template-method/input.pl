#!/usr/bin/perl
use strict;
use warnings;

# The template method: a base class method that calls another method on its
# own object, where the subclass replaces the one being called. This is the
# shape every report generator, every exporter and every error hierarchy in
# Perl is built on.

package Report;

sub new {
    my ( $class, %args ) = @_;
    return bless { title => $args{title}, rows => $args{rows} || [] }, $class;
}

sub title  { $_[0]{title} }
sub prefix { '' }
sub suffix { '' }

sub format_row {
    my ( $self, $row ) = @_;
    return join( ' | ', @$row );
}

# Nothing below is overridden; everything it calls may be.
sub render {
    my ($self) = @_;
    my @out = ( $self->prefix . uc( $self->title ) . $self->suffix );
    push @out, $self->format_row($_) for @{ $self->{rows} };
    push @out, sprintf '%d row(s), style %s', scalar @{ $self->{rows} }, $self->style;
    return join "\n", @out;
}

sub style { 'plain' }

package Report::Boxed;
our @ISA = ('Report');
sub prefix { '[ ' }
sub suffix { ' ]' }
sub style  { 'boxed' }

package Report::Csv;
our @ISA = ('Report');
sub style { 'csv' }
sub format_row {
    my ( $self, $row ) = @_;
    return join ',', map { /,/ ? qq{"$_"} : $_ } @$row;
}

package main;

my @rows = ( [ 'north', 'widget, large', 12 ], [ 'south', 'gizmo', 7 ] );

for my $r ( Report->new( title => 'quarterly', rows => \@rows ),
            Report::Boxed->new( title => 'quarterly', rows => \@rows ),
            Report::Csv->new( title => 'quarterly', rows => \@rows ) ) {
    print $r->render, "\n";
    print "---\n";
}

# A base object with no subclass in play still works.
my $plain = Report->new( title => 'plain', rows => [] );
print "style of a bare Report: ", $plain->style, "\n";
print "title through the accessor: ", $plain->title, "\n";

package Shape;
# Abstract-ish base class: subclasses must implement area() and perimeter().
use strict;
use warnings;

my $SERIAL = 0;

sub new {
    my ( $class, %args ) = @_;
    die "Shape is abstract; instantiate a subclass\n" if $class eq __PACKAGE__;
    my $self = bless {
        name   => $args{name} // lc($class),
        serial => ++$SERIAL,
    }, $class;
    $self->init(%args);
    return $self;
}

# Subclasses override; base implementation is a no-op hook.
sub init { return }

sub name   { $_[0]{name} }
sub serial { $_[0]{serial} }

# "Pure virtual" methods.
sub area {
    my ($self) = @_;
    die ref($self) . " does not implement area()\n";
}

sub perimeter {
    my ($self) = @_;
    die ref($self) . " does not implement perimeter()\n";
}

sub describe {
    my ($self) = @_;
    return sprintf "#%d %-10s area=%8.3f perimeter=%8.3f",
      $self->serial, $self->name, $self->area, $self->perimeter;
}

1;

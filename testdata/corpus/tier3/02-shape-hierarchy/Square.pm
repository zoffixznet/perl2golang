package Square;
# Two-level inheritance: Square -> Rectangle -> Shape, via old-school @ISA.
use strict;
use warnings;
use Rectangle;
our @ISA = ('Rectangle');

sub init {
    my ( $self, %args ) = @_;
    my $side = $args{side} // 0;
    # Delegate validation and storage to Rectangle's init through SUPER::.
    $self->SUPER::init( %args, width => $side, height => $side );
    return;
}

# Override describe to prove overriding works two levels up.
sub describe {
    my ($self) = @_;
    my $base = $self->SUPER::describe;
    return $base . ( $self->is_square ? ' [square]' : ' [impossible]' );
}

1;

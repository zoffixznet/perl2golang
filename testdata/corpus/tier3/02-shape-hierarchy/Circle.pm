package Circle;
use strict;
use warnings;
use parent -norequire, 'Shape';
use Shape;
use constant PI => 3.14159265358979;

sub init {
    my ( $self, %args ) = @_;
    $self->SUPER::init(%args);
    die "Circle needs a positive radius\n" unless ( $args{radius} // 0 ) > 0;
    $self->{radius} = $args{radius};
    return;
}

sub radius    { $_[0]{radius} }
sub area      { my $s = shift; PI * $s->radius**2 }
sub perimeter { my $s = shift; 2 * PI * $s->radius }

1;

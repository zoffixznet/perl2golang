package Rectangle;
use strict;
use warnings;
use parent -norequire, 'Shape';
use Shape;

sub init {
    my ( $self, %args ) = @_;
    $self->SUPER::init(%args);
    die "Rectangle needs positive width/height\n"
      unless ( $args{width} // 0 ) > 0 && ( $args{height} // 0 ) > 0;
    $self->{width}  = $args{width};
    $self->{height} = $args{height};
    return;
}

sub width  { $_[0]{width} }
sub height { $_[0]{height} }

sub area      { my $s = shift; $s->width * $s->height }
sub perimeter { my $s = shift; 2 * ( $s->width + $s->height ) }

sub is_square { my $s = shift; $s->width == $s->height }

1;

package TextUtil;
# Utility module demonstrating Exporter with tags and package-level state.
use strict;
use warnings;
use Exporter 'import';
use constant {
    DEFAULT_WIDTH => 24,
    ELLIPSIS      => '...',
    VERSION_TAG   => 'textutil/1.2',
};

our @EXPORT_OK = qw(
  trim squeeze title_case truncate_str commify call_stats
  DEFAULT_WIDTH
);
our %EXPORT_TAGS = (
    clean => [qw(trim squeeze)],
    fmt   => [qw(title_case truncate_str commify DEFAULT_WIDTH)],
    all   => \@EXPORT_OK,
);

# Package-level state: every exported function bumps its own counter.
our %CALLS;

sub _count { $CALLS{ $_[0] }++; return }

sub trim {
    _count('trim');
    my ($s) = @_;
    $s =~ s/^\s+//;
    $s =~ s/\s+$//;
    return $s;
}

sub squeeze {
    _count('squeeze');
    my ($s) = @_;
    $s =~ tr/ \t/ /s;
    return $s;
}

sub title_case {
    _count('title_case');
    my ($s) = @_;
    $s =~ s/(\w+)/\u\L$1/g;
    return $s;
}

sub truncate_str {
    _count('truncate_str');
    my ( $s, $width ) = @_;
    $width //= DEFAULT_WIDTH;
    return $s if length($s) <= $width;
    return substr( $s, 0, $width - length(ELLIPSIS) ) . ELLIPSIS;
}

sub commify {
    _count('commify');
    my ($n) = @_;
    my $s = reverse "$n";
    $s =~ s/(\d{3})(?=\d)/$1,/g;
    return scalar reverse $s;
}

sub call_stats {
    my @out;
    push @out, sprintf( '%-12s %3d', $_, $CALLS{$_} ) for sort keys %CALLS;
    return @out;
}

sub identify { return __PACKAGE__ . ' (' . VERSION_TAG . ')' }

1;

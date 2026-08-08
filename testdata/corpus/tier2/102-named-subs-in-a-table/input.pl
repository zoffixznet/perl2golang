#!/usr/bin/perl
use strict;
use warnings;

# A dispatch table built from references to named subs, the shape a script
# grows into once the handlers are too big to write inline. The named subs
# all take a string and answer with one, so the table has a signature to
# find; whether the converter finds it through \&name is this entry's
# question.

sub clean_spaces {
    my ($raw) = @_;
    $raw =~ s/\s+/ /g;
    $raw =~ s/^ | $//g;
    return $raw;
}

sub clean_case {
    my ($raw) = @_;
    return ucfirst( lc $raw );
}

sub clean_digits {
    my ($raw) = @_;
    $raw =~ s/\D//g;
    return $raw;
}

sub clean_passthrough {
    my ($raw) = @_;
    return $raw;
}

my %clean = (
    name  => \&clean_case,
    note  => \&clean_spaces,
    phone => \&clean_digits,
);

my @rows = (
    [ 'name',  ' ada LOVELACE ' ],
    [ 'note',  ' first   programmer  ' ],
    [ 'phone', '(555) 010-0100' ],
    [ 'other', 'left alone' ],
);

for my $row (@rows) {
    my ( $field, $value ) = @$row;
    my $fixer = $clean{$field} || \&clean_passthrough;
    printf "%-6s -> '%s'\n", $field, $fixer->($value);
}

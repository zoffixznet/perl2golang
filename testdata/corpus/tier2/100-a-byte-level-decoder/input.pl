#!/usr/bin/perl
# A quoted-printable-ish decoder for subject lines: the replacement under
# /e is several statements, chr builds UTF-8 sequences one byte at a time,
# and the report reads its named captures straight from interpolation.
use strict;
use warnings;

sub decode_q {
    my ($s) = @_;
    $s =~ s{=\?[\w-]+\?[Qq]\?(.*?)\?=}{
        my $t = $1;
        $t =~ tr/_/ /;
        $t =~ s/=([0-9A-Fa-f]{2})/chr hex $1/ge;
        $t;
    }ge;
    return $s;
}

for my $line (
    '=?UTF-8?Q?Caf=C3=A9_menu?=',
    '=?UTF-8?Q?100=25_natural?=',
    'plain words',
) {
    printf "%s\n", decode_q($line);
}

# Named captures read from inside a string.
my $log = 'status=shipped carrier=DHL weight=1.25';
while ( $log =~ /(?<key>\w+)=(?<val>\S+)/g ) {
    print "  $+{key} -> $+{val}\n";
}

# Fields skipped by position, the tab-separated way.
my $row = "ord-77\tunused\tCafe\t12.50\tunused\tpaid";
my ( $id, undef, $name, $price, undef, $state ) = split /\t/, $row;
print "$id: $name ($price) $state\n";

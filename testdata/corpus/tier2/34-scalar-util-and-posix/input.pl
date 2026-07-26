#!/usr/bin/perl
use strict;
use warnings;
use Scalar::Util qw(blessed reftype looks_like_number);
use POSIX qw(floor ceil fmod strftime setlocale LC_TIME);

# Type interrogation with Scalar::Util and the numeric/date helpers from
# POSIX. The timestamp is fixed and formatted in UTC, and the time locale
# is pinned to C, so the output never depends on when or where this runs.

$ENV{TZ} = 'UTC';
POSIX::tzset();
setlocale(LC_TIME, 'C');    # %A and %B are month/day NAMES: locale-sensitive

package Widget;
sub new { my ($c, %a) = @_; return bless { %a }, $c }
package Gadget;
our @ISA = ('Widget');
package Counter;
sub new { my ($c) = @_; my $n = 0; return bless \$n, $c }
package main;

my @things = (
    [ 'plain string'   => 'hello'            ],
    [ 'integer'        => 42                 ],
    [ 'float string'   => '3.14'             ],
    [ 'exp string'     => '1e3'              ],
    [ 'hex string'     => '0x1f'             ],
    [ 'padded number'  => '  7  '            ],
    [ 'not a number'   => '12abc'            ],
    [ 'undef'          => undef              ],
    [ 'arrayref'       => [ 1, 2, 3 ]        ],
    [ 'hashref'        => { a => 1 }         ],
    [ 'coderef'        => sub { 1 }          ],
    [ 'scalarref'      => \'x'               ],
    [ 'Widget object'  => Widget->new(id => 1) ],
    [ 'Gadget object'  => Gadget->new(id => 2) ],
    [ 'Counter object' => Counter->new()     ],
);

printf "%-15s %-8s %-8s %-8s %s\n", 'LABEL', 'REF', 'REFTYPE', 'BLESSED', 'NUMERIC';
for my $t (@things) {
    my ($label, $val) = @$t;
    my $r  = ref($val)             || '-';
    my $rt = reftype($val)         || '-';
    my $bl = blessed($val)         || '-';
    my $ln = defined $val && looks_like_number($val) ? 'yes' : 'no';
    printf "%-15s %-8s %-8s %-8s %s\n", $label, $r, $rt, $bl, $ln;
}

# blessed plus isa gives a safe "is this one of mine" check.
for my $obj (Widget->new, Gadget->new, [ ], 'string') {
    my $what = blessed($obj)
             ? (($obj->isa('Widget') ? 'a Widget subclass: ' : 'some object: ') . blessed($obj))
             : (ref($obj) ? 'an unblessed ' . ref($obj) : 'not a reference');
    print "$what\n";
}

# Numeric helpers.
printf "%-9s %8s %8s %8s %8s\n", 'VALUE', 'FLOOR', 'CEIL', 'INT', 'ROUND';
for my $n (3.7, -3.7, 2.5, -2.5, 10, 0.001) {
    printf "%-9s %8.1f %8.1f %8d %8.0f\n",
        $n, floor($n), ceil($n), int($n), sprintf('%.0f', $n);
}

printf "fmod(17, 5)=%.2f fmod(-17, 5)=%.2f 17%%5=%d\n", fmod(17, 5), fmod(-17, 5), 17 % 5;

# Rounding to a fixed number of places without sprintf.
sub round_to {
    my ($n, $places) = @_;
    my $scale = 10 ** $places;
    return floor($n * $scale + 0.5) / $scale;
}
printf "round_to: %s %s %s\n", round_to(3.14159, 2), round_to(2.71828, 3), round_to(1.005, 2);

# Fixed timestamp, formatted several ways, always in UTC.
my $epoch = 1_700_000_000;
my @utc = gmtime($epoch);
printf "iso:      %s\n", strftime('%Y-%m-%dT%H:%M:%SZ', @utc);
printf "human:    %s\n", strftime('%A, %d %B %Y', @utc);
printf "compact:  %s\n", strftime('%Y%m%d-%H%M%S', @utc);
printf "day %s of year, week %s, weekday %s\n",
    strftime('%j', @utc), strftime('%U', @utc), strftime('%w', @utc);

# Arithmetic on the epoch, then formatting each result.
for my $delta (0, 3600, 86_400, 7 * 86_400) {
    printf "  +%-7d %s\n", $delta, strftime('%Y-%m-%d %H:%M:%S', gmtime($epoch + $delta));
}

# looks_like_number driving a validation pass.
my @input = ('10', '-3.5', '1e2', 'NaN', 'twelve', '', '0');
my @good  = grep { looks_like_number($_) } @input;
my @bad   = grep { !looks_like_number($_) } @input;
printf "valid: %s\n", join(',', map { defined $_ ? "'$_'" : 'undef' } @good);
printf "invalid: %s\n", join(',', map { "'$_'" } @bad);
my $total = 0;
$total += $_ for grep { /^-?\d+(?:\.\d+)?$/ } @good;
printf "sum of plain decimals: %.1f\n", $total;

#!/usr/bin/perl
use strict;
use warnings;

# Order report: reads a header-driven CSV, groups by two keys, and prints
# aligned columns with sprintf. Deliberately uses plain split (the file is
# known to have no quoted commas) which is what most one-off scripts do.

my $file = 'files/orders.csv';
open my $fh, '<', $file or die "cannot open $file: $!\n";

my $header = <$fh>;
chomp $header;
my @cols = split /,/, $header;
my %idx  = map { $cols[$_] => $_ } 0 .. $#cols;

my @orders;
while (my $line = <$fh>) {
    chomp $line;
    next unless $line =~ /\S/;
    my @f = split /,/, $line, scalar @cols;
    my %row = map { $_ => $f[ $idx{$_} ] } @cols;
    $row{total} = $row{qty} * $row{unit_price};
    push @orders, \%row;
}
close $fh or die "close: $!\n";

printf "columns: %s\n", join('|', @cols);
printf "%d order(s) loaded\n\n", scalar @orders;

# Detail table.
printf "%-6s %-10s %-6s %-10s %4s %9s %9s  %s\n",
    'ID', 'CUSTOMER', 'REGION', 'PRODUCT', 'QTY', 'UNIT', 'TOTAL', 'STATUS';
printf "%s\n", '-' x 72;
for my $o (sort { $a->{order_id} <=> $b->{order_id} } @orders) {
    printf "%-6s %-10s %-6s %-10s %4d %9.2f %9.2f  %s\n",
        $o->{order_id}, $o->{customer}, $o->{region}, $o->{product},
        $o->{qty}, $o->{unit_price}, $o->{total}, $o->{status};
}

# Group by region, then by status inside each region.
my %by_region;
for my $o (@orders) {
    next if $o->{status} eq 'cancelled';
    $by_region{ $o->{region} }{ $o->{status} }{count}++;
    $by_region{ $o->{region} }{ $o->{status} }{value} += $o->{total};
    $by_region{ $o->{region} }{ALL}{count}++;
    $by_region{ $o->{region} }{ALL}{value} += $o->{total};
}

print "\n";
printf "%-8s %-9s %6s %10s\n", 'REGION', 'STATUS', 'ORDERS', 'VALUE';
printf "%s\n", '-' x 36;
my $grand = 0;
for my $region (sort keys %by_region) {
    for my $status (sort keys %{ $by_region{$region} }) {
        my $cell = $by_region{$region}{$status};
        printf "%-8s %-9s %6d %10.2f\n", $region, $status, $cell->{count}, $cell->{value};
    }
    $grand += $by_region{$region}{ALL}{value};
}
printf "%s\n", '-' x 36;
printf "%-8s %-9s %6s %10.2f\n", 'TOTAL', '', '', $grand;

# Per-customer roll-up with a percentage column.
my %cust;
$cust{ $_->{customer} } += $_->{total} for grep { $_->{status} ne 'cancelled' } @orders;
print "\n";
for my $c (sort { $cust{$b} <=> $cust{$a} || $a cmp $b } keys %cust) {
    printf "%-10s %9.2f  %5.1f%%  %s\n",
        $c, $cust{$c}, 100 * $cust{$c} / $grand, '*' x int(20 * $cust{$c} / $grand + 0.5);
}

# A quoted-field aware splitter, for the one line that needs it.
sub parse_csv_line {
    my ($line) = @_;
    my @out;
    while ($line =~ /\G(?:"((?:[^"]|"")*)"|([^,]*))(?:,|$)/gc) {
        my $field = defined $1 ? $1 : $2;
        $field =~ s/""/"/g if defined $1;
        push @out, $field;
        last if pos($line) >= length $line;
    }
    return @out;
}
my @tricky = parse_csv_line('1009,"Epsilon, Ltd",north,"12"" widget",2,9.99,shipped');
printf "tricky: %d fields -> [%s]\n", scalar @tricky, join('][', @tricky);

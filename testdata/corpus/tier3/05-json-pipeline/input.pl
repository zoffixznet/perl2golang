#!/usr/bin/perl
# Reads an orders JSON file, transforms the nested structure into a
# per-customer summary, and emits deterministic (canonical) JSON.
use strict;
use warnings;
use JSON::PP;

my $path = shift @ARGV or die "usage: $0 orders.json\n";

my $json = JSON::PP->new->canonical(1);    # sorted keys => stable output

my $raw = do {                              # slurp via do-block as expression
    open my $fh, '<', $path or die "open $path: $!\n";
    local $/;
    <$fh>;
};
my $doc = $json->decode($raw);

die "unsupported currency\n" unless $doc->{currency} eq 'EUR';

# ---- transform: orders -> per-customer rollup --------------------------
my %by_customer;
my %sku_qty;
for my $order ( @{ $doc->{orders} } ) {
    my $cust  = $order->{customer};
    my $name  = $cust->{name};
    my $total = 0;
    for my $item ( @{ $order->{items} } ) {
        $total += $item->{qty} * $item->{unit_price};
        $sku_qty{ $item->{sku} } += $item->{qty};
    }
    my $slot = $by_customer{$name} //= {
        tier        => $cust->{tier},
        order_count => 0,
        spend       => 0,
        order_ids   => [],
        any_pending => JSON::PP::false,
    };
    $slot->{order_count}++;
    $slot->{spend} += $total;
    push @{ $slot->{order_ids} }, $order->{id};
    $slot->{any_pending} = JSON::PP::true unless $order->{shipped};
}

# Normalise floats so 0.1+0.2-style noise can't leak into the JSON.
for my $slot ( values %by_customer ) {
    $slot->{spend} = 0 + sprintf '%.2f', $slot->{spend};
}

my @vips =
  sort grep { $by_customer{$_}{spend} >= 20 } keys %by_customer;

my $summary = {
    source_generated => $doc->{generated},
    currency         => $doc->{currency},
    customer_count   => scalar keys %by_customer,
    vips             => \@vips,
    customers        => \%by_customer,
    top_skus         => [
        ( sort { $sku_qty{$b} <=> $sku_qty{$a} or $a cmp $b } keys %sku_qty )
        [ 0 .. 1 ]
    ],
};

# ---- human-readable table before the JSON ------------------------------
printf "%-6s %-7s %6s %9s %s\n", 'NAME', 'TIER', 'ORDERS', 'SPEND', 'PENDING';
for my $name ( sort keys %by_customer ) {
    my $c = $by_customer{$name};
    printf "%-6s %-7s %6d %9.2f %s\n", $name, $c->{tier}, $c->{order_count},
      $c->{spend}, ( $c->{any_pending} ? 'yes' : 'no' );
}
print "\n";

# ---- canonical JSON round trip -----------------------------------------
my $encoded = $json->pretty->encode($summary);
print $encoded;

# Prove the round trip: decode what we encoded and re-encode; must match.
my $reparsed = $json->decode($encoded);
print "round-trip stable: ",
  ( $json->encode($reparsed) eq $json->encode($summary) ? 'yes' : 'no' ),
  "\n";
printf "vip check: %s\n", join '+', map { substr $_, 0, 2 } @vips;

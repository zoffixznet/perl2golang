#!/usr/bin/perl
# Small warehouse inventory manager: classic hashref-based OO.
use strict;
use warnings;

package Inventory::Item;

my $ITEMS_CREATED = 0;    # class-level state shared by all instances

sub new {
    my ( $proto, %args ) = @_;
    my $class = ref($proto) || $proto;    # works as class method or clone-ish
    die "Item needs a sku\n"  unless defined $args{sku};
    die "Item needs a name\n" unless defined $args{name};
    my $self = {
        sku      => $args{sku},
        name     => $args{name},
        price    => $args{price} // 0,
        quantity => $args{quantity} // 0,
    };
    $ITEMS_CREATED++;
    return bless $self, $class;
}

# Class method: how many items were ever constructed.
sub created_count {
    my ($class) = @_;
    die "created_count is a class method\n" if ref $class;
    return $ITEMS_CREATED;
}

# Read/write accessors.
sub sku  { $_[0]{sku} }
sub name { $_[0]{name} }

sub price {
    my $self = shift;
    $self->{price} = shift if @_;
    return $self->{price};
}

sub quantity {
    my $self = shift;
    $self->{quantity} = shift if @_;
    return $self->{quantity};
}

sub value { my $self = shift; return $self->price * $self->quantity }

sub restock {
    my ( $self, $n ) = @_;
    die "restock amount must be positive\n" unless $n > 0;
    $self->{quantity} += $n;
    return $self;    # chainable
}

sub sell {
    my ( $self, $n ) = @_;
    die sprintf( "cannot sell %d of %s: only %d on hand\n",
        $n, $self->sku, $self->quantity )
      if $n > $self->quantity;
    $self->{quantity} -= $n;
    return $self;
}

sub to_line {
    my ($self) = @_;
    return sprintf "%-8s %-22s %8.2f %5d %10.2f",
      $self->sku, $self->name, $self->price, $self->quantity, $self->value;
}

package Inventory;

sub new {
    my ($class) = @_;
    return bless { items => {} }, $class;
}

sub add {
    my ( $self, $item ) = @_;
    die "not an item\n" unless ref $item && $item->isa('Inventory::Item');
    $self->{items}{ $item->sku } = $item;
    return $self;
}

sub get { my ( $self, $sku ) = @_; return $self->{items}{$sku} }

sub skus {
    my ($self) = @_;
    return sort keys %{ $self->{items} };
}

sub total_value {
    my ($self) = @_;
    my $total = 0;
    $total += $self->{items}{$_}->value for $self->skus;
    return $total;
}

sub report {
    my ($self) = @_;
    my @out = sprintf "%-8s %-22s %8s %5s %10s", 'SKU', 'NAME', 'PRICE',
      'QTY', 'VALUE';
    push @out, '-' x 57;
    push @out, $self->get($_)->to_line for $self->skus;
    push @out, '-' x 57;
    push @out, sprintf "%-37s %19.2f", 'TOTAL', $self->total_value;
    return join "\n", @out;
}

package main;

my $inv = Inventory->new;
$inv->add( Inventory::Item->new( sku => 'WID-01', name => 'Widget, small',
        price => 2.50, quantity => 100 ) );
$inv->add( Inventory::Item->new( sku => 'WID-02', name => 'Widget, large',
        price => 4.75, quantity => 40 ) );
$inv->add( Inventory::Item->new( sku => 'GAD-77', name => 'Gadget deluxe',
        price => 19.99, quantity => 7 ) );

# Method chaining plus mutation through accessors.
$inv->get('WID-01')->sell(25)->restock(5);
$inv->get('GAD-77')->price(17.49);

# Duck-typing checks the converter must model.
my $item = $inv->get('WID-02');
printf "can(sell)=%s can(fly)=%s\n",
  ( $item->can('sell') ? 'yes' : 'no' ),
  ( $item->can('fly')  ? 'yes' : 'no' );
printf "isa(Inventory::Item)=%s ref=%s\n",
  ( $item->isa('Inventory::Item') ? 'yes' : 'no' ), ref $item;

# Selling more than we have throws; catch and keep going.
my $err = '';
eval { $inv->get('WID-02')->sell(500); 1 } or $err = $@;
print "sell failed: $err";

print $inv->report, "\n";
printf "items created: %d\n", Inventory::Item->created_count;

# Constructor called on an instance (ref($proto)||$proto path).
my $clone = $item->new( sku => 'WID-03', name => 'Widget, clone', price => 1 );
printf "clone sku=%s class=%s\n", $clone->sku, ref $clone;
printf "items created: %d\n", Inventory::Item->created_count;

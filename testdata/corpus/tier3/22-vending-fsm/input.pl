#!/usr/bin/perl
# Vending machine finite state machine: transition table as a hash of
# hashes holding coderefs, event stream on stdin, credit/inventory
# tracking, and an audit trail printed at the end.
use strict;
use warnings;

my %price = ( cola => 120, water => 90, crisps => 150 );
my %stock = ( cola => 2,   water => 1, crisps => 0 );

my $state  = 'idle';
my $credit = 0;
my @audit;
my %event_count;

sub note { push @audit, sprintf '%-10s %s', "[$state]", $_[0] }

sub insert_coin {
    my ($amount) = @_;
    die "coin must be one of 10/20/50/100\n"
      unless grep { $amount == $_ } 10, 20, 50, 100;
    $credit += $amount;
    note("credit now $credit");
    return 'collecting';
}

sub select_item {
    my ($item) = @_;
    my $cost = $price{$item} // die "no such item '$item'\n";
    if ( ( $stock{$item} // 0 ) == 0 ) {
        note("'$item' sold out");
        return $state;    # self-transition, credit kept
    }
    if ( $credit < $cost ) {
        note( sprintf "need %d more for '%s'", $cost - $credit, $item );
        return $state;
    }
    $stock{$item}--;
    my $change = $credit - $cost;
    $credit = 0;
    note( "dispensing '$item'"
          . ( $change ? ", change $change" : ', exact payment' ) );
    return 'idle';
}

sub refund {
    if ($credit) { note("refunding $credit"); $credit = 0 }
    else         { note('nothing to refund') }
    return 'idle';
}

# transition table: state -> event -> handler; missing = illegal
my %fsm = (
    idle => {
        coin   => \&insert_coin,
        refund => \&refund,
    },
    collecting => {
        coin   => \&insert_coin,
        select => \&select_item,
        refund => \&refund,
    },
);

while ( my $line = <STDIN> ) {
    chomp $line;
    next if $line =~ /^\s*(#|$)/;
    my ( $event, $arg ) = split ' ', $line, 2;

    my $handler = $fsm{$state}{$event};
    unless ($handler) {
        note("ILLEGAL event '$event'");
        $event_count{illegal}++;
        next;
    }
    $event_count{$event}++;
    my $next = eval { $handler->( defined $arg ? $arg : () ) };
    if ($@) {
        chomp( my $err = $@ );
        note("REJECTED $event: $err");
        next;    # state unchanged on error
    }
    if ( $next ne $state ) {
        note("-> $next");
        $state = $next;
    }
}

print "--- audit trail ---\n";
print "$_\n" for @audit;

print "--- final ---\n";
print "state: $state, credit: $credit\n";
print "stock: ", join( ' ', map { "$_=$stock{$_}" } sort keys %stock ), "\n";
print "events: ",
  join( ' ', map { "$_=$event_count{$_}" } sort keys %event_count ), "\n";

my $till = 0;
$till += $price{$_} * ( { cola => 2, water => 1, crisps => 0 }->{$_} - $stock{$_} )
  for keys %price;
print "takings: $till\n";

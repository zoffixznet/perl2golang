#!/usr/bin/perl
# TRAP: format/write -- a compile-time report DSL with picture lines,
# per-filehandle pagination state, and its own variable binding rules.
use strict;
use warnings;

our ( $name, $qty, $price );

format STDOUT_TOP =
Item         Qty   Price
------------------------
.

format STDOUT =
@<<<<<<<<<< @>>  @>>>>>
$name,      $qty, $price
.

for ( [ apple => 3, '1.50' ], [ banana => 12, '0.25' ], [ cherry => 100, '9.99' ] ) {
    ( $name, $qty, $price ) = @$_;
    write;                      # renders via the format, tracks page state
}

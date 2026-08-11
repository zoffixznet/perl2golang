#!/usr/bin/perl
use strict;
use warnings;

# The same handle-in-a-container shape, one level further in: the handle is
# one field of a record that also holds ordinary text and numbers. This is how
# a logger, a downloader or anything else with an open resource is written
# without a class.

sub open_sink {
    my ($path) = @_;
    my $sink = { path => $path, written => 0 };
    open( $sink->{fh}, '>', $path ) or die "open $path: $!\n";
    return $sink;
}

sub emit {
    my ( $sink, $line ) = @_;
    print { $sink->{fh} } "$line\n";
    $sink->{written}++;
    return;
}

my $sink = open_sink('record-sink.log');
emit( $sink, 'first' );
emit( $sink, 'second' );
close $sink->{fh} or die "close: $!\n";

printf "%s took %d line(s), %d bytes\n",
    $sink->{path}, $sink->{written}, -s $sink->{path};

open my $in, '<', $sink->{path} or die "reopen: $!\n";
my @back = <$in>;
close $in;
chomp @back;
print "the file holds: @back\n";
unlink $sink->{path};

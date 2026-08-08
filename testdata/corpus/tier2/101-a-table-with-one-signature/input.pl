#!/usr/bin/perl
use strict;
use warnings;

# Closures sharing a slot in every shape that can keep one signature: a
# dispatch table whose members disagree about arity, a member that reads
# $_[0] instead of naming its argument, an int in one position, a fallback
# sub joined in through ||, and a pipeline held in an array.

my @log;

my %format = (
    plain  => sub { my ($text) = @_; $text },
    banner => sub { my ( $text, $width ) = @_; sprintf '[%-*s]', $width, $text },
    shout  => sub { uc( $_[0] ) . '!' },
    log    => sub { push @log, $_[0]; 'logged' },
);

sub format_for {
    my ($name) = @_;
    return $format{$name} || sub { '(no such format)' };
}

for my $name (qw(plain banner shout log missing)) {
    print "$name: ", format_for($name)->( 'report', 10 ), "\n";
}
print "log holds: ", join( '|', @log ), "\n";

# A pipeline: every stage takes a string and answers with one.
my @stages = (
    sub { my ($s) = @_; $s =~ s/\s+/ /g; $s },
    sub { my ($s) = @_; ucfirst $s },
    sub { my ($s) = @_; "<<$s>>" },
);
my $text = "  three   word   phrase ";
$text = $_->($text) for @stages;
print "piped: $text\n";

# A counter closure typed entirely by its calls: nothing in the body says
# what $by is, so the numbers at the call sites are the only evidence.
my %tally;
for my $kind (qw(pass fail)) {
    my $n = 0;
    $tally{$kind} = sub { my ($by) = @_; $n += $by; $n };
}
$tally{pass}->(2);
$tally{pass}->(3);
$tally{fail}->(1);
printf "pass=%d fail=%d\n", $tally{pass}->(0), $tally{fail}->(0);

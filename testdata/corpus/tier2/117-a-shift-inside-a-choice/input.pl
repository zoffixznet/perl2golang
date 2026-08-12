#!/usr/bin/perl
# A side effect that lives inside one arm of a choice. The shift must not
# run unless its arm is chosen, and the test must see the array as it was
# before the arm touched it. The same rule holds for the fallback of // and
# ||: the right side runs only when the left side did not answer.
use strict;
use warnings;

my %config = (retries => "4", quiet => "");

my $threshold = @ARGV && $ARGV[0] =~ /^\d+$/ ? shift @ARGV : 3;
my $label     = @ARGV ? shift @ARGV : 'unnamed';
print "threshold: $threshold\n";
print "label:     $label\n";
print "left over: @ARGV\n";

sub fallback {
    print "fallback ran for $_[0]\n";
    return "0";
}

# The stored value answers, so fallback() must stay silent.
my $retries = $config{retries} // fallback('retries');
# Nothing stored, so this one runs.
my $workers = $config{workers} // fallback('workers');
my $quiet   = $config{quiet}   || fallback('quiet');
print "retries:   $retries\n";
print "workers:   $workers\n";
print "quiet:     $quiet\n";

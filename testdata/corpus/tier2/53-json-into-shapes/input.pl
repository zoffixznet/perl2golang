#!/usr/bin/perl
# The JSON edges a decoder into an untyped value cannot keep: what an integer
# comes back as, what an absent key is, how a boolean survives, and what the
# encoder does to the text itself. This is the neighbouring case, recorded so
# the next round has a target.
use strict;
use warnings;
use JSON::PP;

my $codec = JSON::PP->new->canonical(1);

my $text = <<'JSON';
{
  "id": 7,
  "ratio": 0.5,
  "big": 9007199254740993,
  "name": "widget <&> gadget",
  "ok": true,
  "off": false,
  "nothing": null,
  "nested": { "depth": 2, "list": [1, 2, 3] }
}
JSON

my $doc = $codec->decode($text);

print "-- what came back --\n";
printf "id is 7:            %s\n", ( $doc->{id} == 7 ? 'yes' : 'no' );
printf "id prints as:       %s\n", $doc->{id};
printf "ratio prints as:    %s\n", $doc->{ratio};
printf "big prints as:      %s\n", $doc->{big};
printf "ok is true:         %s\n", ( $doc->{ok} ? 'yes' : 'no' );
printf "off is false:       %s\n", ( $doc->{off} ? 'yes' : 'no' );
printf "nothing is undef:   %s\n", ( defined $doc->{nothing} ? 'no' : 'yes' );
printf "absent key:         %s\n", ( exists $doc->{absent} ? 'present' : 'absent' );
printf "nested depth:       %s\n", $doc->{nested}{depth};
printf "nested list count:  %d\n", scalar @{ $doc->{nested}{list} };

print "-- what goes back out --\n";
my $out = $codec->encode( { name => $doc->{name}, id => $doc->{id} } );
printf "angle brackets kept: %s\n", ( $out =~ /</ ? 'yes' : 'no' );
printf "id still a number:   %s\n", ( $out =~ /"id":7/ ? 'yes' : 'no' );

print "-- booleans out --\n";
my $flags = $codec->encode( { on => JSON::PP::true, off => JSON::PP::false } );
printf "flags: %s\n", $flags;

print "-- pretty --\n";
my $pretty = JSON::PP->new->canonical(1)->pretty->encode( { a => 1, b => [ 2, 3 ] } );
print $pretty;
printf "pretty ends with a newline: %s\n", ( $pretty =~ /\n$/ ? 'yes' : 'no' );

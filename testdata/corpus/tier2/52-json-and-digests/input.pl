#!/usr/bin/perl
# The three ways a script turns data into bytes: JSON for structure, a
# checksum for identity, and base64 for transport. All three are in Go's
# standard library, and all three are functions there rather than objects.
use strict;
use warnings;
use JSON::PP;
use Digest::MD5 qw(md5_hex md5_base64);
use MIME::Base64 qw(encode_base64 decode_base64);

my $codec = JSON::PP->new->canonical(1);

my $doc = {
    name    => 'reporter',
    version => 3,
    enabled => JSON::PP::true,
    limits  => { cpu => 4, mem => 512 },
    tags    => [ 'nightly', 'eu-west' ],
};

print "-- compact --\n";
my $compact = $codec->encode($doc);
printf "length: %d\n", length $compact;
printf "starts with a brace: %s\n", ( $compact =~ /^\{/ ? 'yes' : 'no' );
printf "keys are sorted: %s\n",
    ( $compact =~ /"enabled".*"limits".*"name".*"tags".*"version"/ ? 'yes' : 'no' );

print "-- round trip --\n";
my $back = $codec->decode($compact);
printf "name: %s\n",    $back->{name};
printf "version: %s\n", $back->{version};
printf "cpu: %s\n",     $back->{limits}{cpu};
printf "first tag: %s\n", $back->{tags}[0];
printf "re-encodes to the same text: %s\n",
    ( $codec->encode($back) eq $compact ? 'yes' : 'no' );

print "-- checksums --\n";
printf "md5 of empty:  %s\n", md5_hex('');
printf "md5 of abc:    %s\n", md5_hex('abc');
printf "md5 of a.b.c:  %s\n", md5_hex( 'a', 'b', 'c' );
printf "same as joined: %s\n", ( md5_hex( 'a', 'b', 'c' ) eq md5_hex('abc') ? 'yes' : 'no' );
printf "base64 digest: %s\n", md5_base64('abc');

my $ctx = Digest::MD5->new;
$ctx->add('a');
$ctx->add( 'b', 'c' );
printf "accumulated:   %s\n", $ctx->hexdigest;

print "-- base64 --\n";
my $encoded = encode_base64('hello, world');
printf "encoded: %s", $encoded;
printf "ends in a newline: %s\n", ( $encoded =~ /\n$/ ? 'yes' : 'no' );
printf "decoded: %s\n", decode_base64($encoded);
printf "round trip holds: %s\n",
    ( decode_base64( encode_base64('a longer piece of text to encode') ) eq
        'a longer piece of text to encode' ? 'yes' : 'no' );

my $long = 'x' x 200;
my @wrapped = split /\n/, encode_base64($long);
printf "long input wraps into %d lines of at most %d\n",
    scalar @wrapped, ( sort { $b <=> $a } map { length } @wrapped )[0];

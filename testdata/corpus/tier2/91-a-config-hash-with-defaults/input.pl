#!/usr/bin/perl
# The same config-hash shape one idiom later: every read leans on undef.
# A field is defaulted with //, probed with exists and defined, and one
# key is only sometimes there. A struct field always holds a value, so
# this file has no struct translation with the same meaning; the honest
# type is still a map, and the reads keep their two-answer questions.
use strict;
use warnings;

my %conf = (
    host  => 'db1',
    port  => 0,        # 0 is configured, and different from absent
    label => '',
);

printf "host: %s\n", $conf{host} // 'localhost';
printf "port: %s\n", $conf{port} // 5432;       # stays 0: set, not absent
printf "user: %s\n", $conf{user} // 'anonymous';    # absent: defaulted

printf "label set: %s\n",  defined $conf{label} ? 'yes' : 'no';
printf "has region: %s\n", exists $conf{region} ? 'yes' : 'no';

$conf{region} = 'eu-west' if @ARGV;
printf "region now: %s\n", $conf{region} // '(unset)';

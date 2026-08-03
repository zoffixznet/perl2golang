#!/usr/bin/perl
# The two shapes a hash takes that the record analysis deliberately leaves
# alone: a named hash used as a record, and a record asked the questions you
# ask a collection. Both come out as maps of anything today.
use strict;
use warnings;

# A named hash whose keys are written out and whose values differ in kind is
# just as much a record as a hash reference is. This one stays a map, because
# a named hash is also where `keys`, `delete` and slices turn up.
my %server = (
    host    => 'db-01',
    port    => 5432,
    healthy => 1,
    tags    => [ 'primary', 'eu-west' ],
);
printf "%s:%d healthy=%d tags=%s\n",
    $server{host}, $server{port}, $server{healthy},
    join( ',', @{ $server{tags} } );

# The collection questions, asked of a record.
print "-- asked as a collection --\n";
printf "fields: %s\n", join( ' ', sort keys %server );
printf "count: %d\n", scalar keys %server;
printf "has port: %s, has region: %s\n",
    ( exists $server{port} ? 'yes' : 'no' ),
    ( exists $server{region} ? 'yes' : 'no' );

delete $server{healthy};
printf "after delete: %s\n", join( ' ', sort keys %server );

# A hash reference asked the same questions.
my $job = {
    name    => 'reindex',
    seconds => 45,
    retries => 1,
};
print "-- a reference asked the same --\n";
printf "fields: %s\n", join( ' ', sort keys %$job );
for my $k ( sort keys %$job ) {
    printf "  %-8s = %s\n", $k, $job->{$k};
}
printf "values sorted as text: %s\n", join( ' ', sort map {"$_"} values %$job );

# Copying one record over another, key by key.
my %copy;
$copy{$_} = $job->{$_} for keys %$job;
$copy{name} = 'rebuild';
printf "copy: %s original: %s\n", $copy{name}, $job->{name};

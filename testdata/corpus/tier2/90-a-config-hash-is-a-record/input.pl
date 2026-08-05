#!/usr/bin/perl
# The config-hash idiom: a named hash whose keys are written into the
# program and whose values differ in kind. Nothing here ever treats the
# keys as data, so the whole file is one record with three field kinds,
# read directly, written directly, listed, and reached once through a
# key picked at run time.
use strict;
use warnings;

my %job = (
    name    => 'nightly-backup',
    minutes => 90,
    dry_run => 0,
    targets => [ 'db1', 'files' ],
);

printf "job %s (%d min)\n", $job{name}, $job{minutes};
printf "targets: %s\n", join( '+', @{ $job{targets} } );

# static writes, including one key the initialiser did not mention
$job{minutes} = $job{minutes} + 30;
$job{late} = $job{minutes} > 100 ? 1 : 0;
printf "stretched to %d, late=%d\n", $job{minutes}, $job{late};

# the collection questions a record can still answer
printf "fields: %s\n", join( ',', sort keys %job );

# a field picked at run time, from a written-out set
for my $field (qw(name dry_run)) {
    printf "  %-8s = %s\n", $field, $job{$field};
}

# the record's values flow on into ordinary code
my $headline = uc $job{name};
$headline =~ s/-/ /g;
printf "headline: %s\n", $headline;

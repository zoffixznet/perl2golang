#!/usr/bin/perl
# Hash references used as records: the shape most Perl programs carry their
# data in, and the one a converter has to decide about. Every key here is a
# literal known before the program runs, and the fields have different types,
# which is what makes a struct the right answer and a map of anything the
# wrong one.
use strict;
use warnings;

sub make_job {
    my ( $name, $secs, $retries ) = @_;
    return {
        name    => $name,
        secs    => $secs,
        retries => $retries,
        tags    => [],
        failed  => 0,
    };
}

my @jobs = (
    make_job( 'backup',  120, 3 ),
    make_job( 'reindex', 45,  1 ),
    make_job( 'vacuum',  600, 0 ),
);

# fields written after construction, still all literal names
for my $job (@jobs) {
    push @{ $job->{tags} }, 'nightly' if $job->{secs} > 100;
    push @{ $job->{tags} }, 'quick'   if $job->{secs} < 60;
    $job->{failed} = 1 if $job->{retries} > 2;
    $job->{minutes} = sprintf '%.1f', $job->{secs} / 60;
}

for my $job ( sort { $a->{secs} <=> $b->{secs} } @jobs ) {
    printf "%-8s %5ds %5s min retries=%d failed=%d tags=[%s]\n",
        $job->{name}, $job->{secs}, $job->{minutes}, $job->{retries},
        $job->{failed}, join( ',', @{ $job->{tags} } );
}

# a record nested inside another record
my %plan = (
    window => { start => '01:00', end => '04:00' },
    owner  => 'ops',
    jobs   => scalar @jobs,
);
printf "plan: owner=%s jobs=%d window=%s-%s\n",
    $plan{owner}, $plan{jobs}, $plan{window}{start}, $plan{window}{end};

# the same record read through a key the program computes, which is what
# rules a struct out and is deliberately kept to one place here
my $field = 'name';
print "first job's $field: $jobs[0]{$field}\n";

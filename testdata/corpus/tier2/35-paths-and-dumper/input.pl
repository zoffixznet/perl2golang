#!/usr/bin/perl
use strict;
use warnings;
use File::Basename qw(basename dirname fileparse);
use File::Spec;
use Cwd qw(getcwd abs_path);
use Data::Dumper;

# Path manipulation plus structure dumping. Nothing absolute is printed:
# absolute paths are turned back into relative ones before they are shown,
# so the transcript is the same wherever the tree lives.

$Data::Dumper::Sortkeys = 1;
$Data::Dumper::Indent   = 1;
$Data::Dumper::Terse    = 0;

my @paths = (
    'files/reports/2023-q4.csv',
    'files/README.md',
    '/var/log/nginx/access.log.1',
    'relative.txt',
    'dir/with.dots/file.tar.gz',
    'trailing/slash/',
);

printf "%-30s %-16s %-20s\n", 'PATH', 'BASENAME', 'DIRNAME';
for my $p (@paths) {
    printf "%-30s %-16s %-20s\n", $p, basename($p), dirname($p);
}

print "-- fileparse with suffix patterns --\n";
for my $p ('files/reports/2023-q4.csv', 'archive.tar.gz', 'plain', 'a/b/c.log.1') {
    my ($name, $dirs, $suffix) = fileparse($p, qr/\.[^.]*/);
    printf "%-26s name=%-12s dir=%-12s suffix=%s\n",
        $p, $name, $dirs, (length $suffix ? $suffix : '(none)');
}

print "-- portable path assembly --\n";
my $joined = File::Spec->catfile('files', 'reports', '2023-q4.csv');
print "catfile: $joined\n";
print "catdir:  ", File::Spec->catdir('files', 'reports'), "\n";
my ($vol, $dirs, $file) = File::Spec->splitpath($joined);
printf "splitpath: dirs=%s file=%s\n", $dirs, $file;
print "splitdir: ", join('|', File::Spec->splitdir('files/reports')), "\n";

print "-- absolute paths, reported relatively --\n";
my $cwd = getcwd();
printf "cwd is absolute: %s\n", ($cwd =~ m{^/} ? 'yes' : 'no');
printf "cwd basename matches this entry: %s\n",
    (basename($cwd) =~ /paths-and-dumper$/ ? 'yes' : 'no');

my $abs = abs_path('files/reports/2023-q4.csv');
printf "abs_path is absolute: %s\n", ($abs =~ m{^/} ? 'yes' : 'no');
printf "abs2rel: %s\n", File::Spec->abs2rel($abs, $cwd);
printf "abs_path of a directory: %s\n", File::Spec->abs2rel(abs_path('files/reports'), $cwd);
printf "abs_path of a missing file: %s\n",
    (defined abs_path('files/nope/none.txt') ? 'defined' : 'undef');

print "-- Data::Dumper --\n";
my $config = {
    name    => 'reporter',
    version => [ 2, 1, 0 ],
    limits  => { cpu => 4, mem => 512, disk => undef },
    outputs => [
        { format => 'csv',  path => 'out/report.csv' },
        { format => 'text', path => 'out/report.txt' },
    ],
    enabled => 1,
};

print Dumper($config);

{
    local $Data::Dumper::Indent = 0;
    local $Data::Dumper::Terse  = 1;
    print "one-liner: ", Dumper($config->{limits}), "\n";
}

{
    local $Data::Dumper::Indent   = 1;
    local $Data::Dumper::Terse    = 1;
    local $Data::Dumper::Deepcopy = 1;
    print "just the outputs:\n", Dumper($config->{outputs});
}

# Dumper with several arguments names them $VAR1, $VAR2, ...
{
    local $Data::Dumper::Indent = 0;
    print Dumper('scalar', [ 1, 2 ]), "\n";
}

# Dumper is often used as a poor man's deep comparison.
my $copy = {
    enabled => 1,
    limits  => { cpu => 4, disk => undef, mem => 512 },
    name    => 'reporter',
    outputs => [
        { format => 'csv',  path => 'out/report.csv' },
        { format => 'text', path => 'out/report.txt' },
    ],
    version => [ 2, 1, 0 ],
};
{
    local $Data::Dumper::Indent = 0;
    printf "structures equal: %s\n", (Dumper($config) eq Dumper($copy) ? 'yes' : 'no');
}

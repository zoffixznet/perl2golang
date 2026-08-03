#!/usr/bin/perl
use strict;
use warnings;
use File::Spec;
use File::Basename qw(basename dirname fileparse);
use Cwd qw(getcwd);

# Path arithmetic done through File::Spec's class methods, which name a class
# only so that each operating system can subclass it. Nothing absolute is
# printed: what is printed is either a relative path or a property of an
# absolute one, so the transcript is the same wherever the tree lives.

print "-- assembling --\n";
print "catfile:  ", File::Spec->catfile('var', 'log', 'app.log'), "\n";
print "catdir:   ", File::Spec->catdir('var', 'log'), "\n";

my @parts = ('etc', 'nginx', 'nginx.conf');
print "catfile(\@parts): ", File::Spec->catfile(@parts), "\n";

print "-- taking apart --\n";
my ($vol, $dirs, $file) = File::Spec->splitpath('var/log/app.log');
printf "splitpath: dirs=%s file=%s\n", $dirs, $file;
print "splitdir:  ", join('|', File::Spec->splitdir('var/log/app.log')), "\n";
print "canonpath: ", File::Spec->canonpath('var//log/./app.log'), "\n";
printf "absolute?  %s %s\n",
    (File::Spec->file_name_is_absolute('/etc/hosts') ? 'yes' : 'no'),
    (File::Spec->file_name_is_absolute('etc/hosts')  ? 'yes' : 'no');
print "curdir=", File::Spec->curdir, " updir=", File::Spec->updir, "\n";

print "-- relative and absolute --\n";
my $cwd = getcwd();
printf "cwd is absolute: %s\n",
    (File::Spec->file_name_is_absolute($cwd) ? 'yes' : 'no');

my $abs = File::Spec->rel2abs('files/notes.txt');
printf "rel2abs is absolute: %s\n",
    (File::Spec->file_name_is_absolute($abs) ? 'yes' : 'no');
printf "and back again:      %s\n", File::Spec->abs2rel($abs, $cwd);
printf "rel2abs with a base: %s\n",
    File::Spec->abs2rel(File::Spec->rel2abs('notes.txt', '/srv/data'), '/srv');
printf "abs2rel up a level:  %s\n",
    File::Spec->abs2rel('/srv/data/notes.txt', '/srv/data/inner');

print "-- names and suffixes --\n";
for my $p ('reports/2023-q4.csv', 'archive.tar.gz', 'notes', 'logs/') {
    printf "%-22s base=%-14s dir=%-10s stripped=%s\n",
        $p, basename($p), dirname($p), basename($p, '.csv', '.gz');
}

my ($name, $dir, $suffix) = fileparse('logs/access.log.1', '\.\d+', '\.log');
printf "fileparse: name=%s dir=%s suffix=%s\n", $name, $dir, $suffix;

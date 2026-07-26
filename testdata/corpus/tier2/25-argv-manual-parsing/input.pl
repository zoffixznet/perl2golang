#!/usr/bin/perl
use strict;
use warnings;

# Hand-rolled option parsing over @ARGV, the way small scripts do it
# before anyone reaches for Getopt::Long. Supports long options with and
# without values, short flag clusters, "--" to stop parsing, and
# positional arguments.

my %opt = (limit => 0, sort => 'name', reverse => 0, verbose => 0, prefix => '');
my @positional;

sub usage {
    return join "\n",
        "usage: $0 [options] FILE...",
        "  --limit=N      keep only the first N records",
        "  --sort=KEY     sort by 'name' or 'count'",
        "  --prefix=STR   only names starting with STR",
        "  -r             reverse the sort",
        "  -v             verbose",
        "  --             end of options",
        '';
}

while (@ARGV) {
    my $arg = shift @ARGV;

    if ($arg eq '--') {
        push @positional, @ARGV;
        @ARGV = ();
        last;
    }
    elsif ($arg =~ /^--(\w[\w-]*)=(.*)$/) {
        my ($name, $val) = ($1, $2);
        die "$0: unknown option --$name\n" unless exists $opt{$name};
        $opt{$name} = $val;
    }
    elsif ($arg =~ /^--(\w[\w-]*)$/) {
        my $name = $1;
        die "$0: unknown option --$name\n" unless exists $opt{$name};
        $opt{$name} = 1;
    }
    elsif ($arg =~ /^-(\w+)$/) {
        for my $ch (split //, $1) {
            if    ($ch eq 'r') { $opt{reverse} = 1 }
            elsif ($ch eq 'v') { $opt{verbose}++ }
            else               { die "$0: unknown flag -$ch\n" }
        }
    }
    else {
        push @positional, $arg;
    }
}

print usage() if $opt{verbose} > 1;

printf "options: limit=%s sort=%s reverse=%d verbose=%d prefix=%s\n",
    $opt{limit}, $opt{sort}, $opt{reverse}, $opt{verbose}, ($opt{prefix} || '(none)');
print "files: @positional\n";

my @records;
for my $file (@positional) {
    open my $fh, '<', $file or die "$0: $file: $!\n";
    while (my $line = <$fh>) {
        chomp $line;
        next unless $line =~ /\S/;
        my ($name, $count) = split ' ', $line;
        push @records, { name => $name, count => $count };
    }
    close $fh or die "close $file: $!\n";
}

@records = grep { index($_->{name}, $opt{prefix}) == 0 } @records if length $opt{prefix};

my $cmp = $opt{sort} eq 'count'
        ? sub { $_[0]{count} <=> $_[1]{count} || $_[0]{name} cmp $_[1]{name} }
        : sub { $_[0]{name}  cmp $_[1]{name} };

@records = sort { $cmp->($a, $b) } @records;
@records = reverse @records if $opt{reverse};
@records = @records[0 .. $opt{limit} - 1] if $opt{limit} && $opt{limit} < @records;

printf "%-12s %5d\n", $_->{name}, $_->{count} for @records;
printf "%d record(s) shown\n", scalar @records;

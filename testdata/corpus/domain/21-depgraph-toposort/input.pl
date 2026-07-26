#!/usr/bin/perl
# depsolve -- resolve build order from dependency declarations.
#
# Kahn's algorithm with alphabetical tie-breaking so the build order is
# reproducible run to run (a hard requirement since we started caching
# by build-plan hash).  Also prints "levels": everything in level N can
# build in parallel once level N-1 is done -- the CI farm consumes that.
#
# Each input file is processed independently under eval so one broken
# graph (cycles, typos) reports and moves on; exit is the count of bad
# graphs.
use strict;
use warnings;

die "usage: $0 <deps.txt> [...]\n" unless @ARGV;

my $failures = 0;
for my $file (@ARGV) {
    print "== $file\n";
    my $ok = eval {
        my $graph = load_graph($file);
        solve($graph);
        1;
    };
    unless ($ok) {
        my $err = $@;
        chomp $err;
        print "FAILED: $err\n";
        $failures++;
    }
    print "\n";
}
exit $failures;

# ----------------------------------------------------------------------
sub load_graph {
    my ($file) = @_;
    open my $fh, '<', $file or die "open $file: $!\n";
    my (%deps, %declared);
    while (<$fh>) {
        next if /^\s*(?:#|$)/;
        chomp;
        my ($target, $rest) = /^(\S+):\s*(.*)$/
            or die "$file line $.: unparseable line '$_'\n";
        die "$file line $.: duplicate declaration of '$target'\n"
            if $declared{$target}++;
        $deps{$target} = [split ' ', $rest];
    }
    close $fh;

    # undeclared dependencies are hard errors: a typo'd dep name used to
    # vanish silently and the target just built without it
    my @unknown;
    for my $t (sort keys %deps) {
        for my $d (@{ $deps{$t} }) {
            push @unknown, "$t -> $d" unless exists $deps{$d};
        }
    }
    die "undeclared dependencies: " . join(', ', @unknown) . "\n" if @unknown;
    return \%deps;
}

sub solve {
    my ($deps) = @_;

    # in-degree = number of unbuilt deps; reverse index for decrementing
    my (%indegree, %rdeps);
    for my $t (keys %$deps) {
        $indegree{$t} = scalar @{ $deps->{$t} };
        push @{ $rdeps{$_} }, $t for @{ $deps->{$t} };
    }

    my @ready = sort grep { $indegree{$_} == 0 } keys %indegree;
    my (@order, $level);
    my $n = 0;

    while (@ready) {
        # everything currently ready forms one parallel level
        my @level = @ready;
        @ready = ();
        $level++;
        printf "level %d: %s\n", $level, join(' ', @level);
        for my $t (@level) {
            push @order, $t;
            $n++;
            for my $waiter (@{ $rdeps{$t} || [] }) {
                push @ready, $waiter if --$indegree{$waiter} == 0;
            }
        }
        @ready = sort @ready;
    }

    if ($n != keys %$deps) {
        # anything still with indegree > 0 is on (or behind) a cycle;
        # walk from the alphabetically-first stuck node to name it
        my @stuck = sort grep { $indegree{$_} > 0 } keys %indegree;
        my $cycle = find_cycle($deps, $stuck[0]);
        die "cycle detected: " . join(' -> ', @$cycle) . "\n";
    }
    print "build order: ", join(' ', @order), "\n";
    printf "%d targets in %d levels\n", $n, $level;
    return 1;
}

# DFS from a stuck node until we revisit something on the current path.
sub find_cycle {
    my ($deps, $start) = @_;
    my (%on_path, @path, $found);
    my $walk;
    $walk = sub {
        my ($node) = @_;
        return if $found;
        if ($on_path{$node}) {
            # trim the path to the cycle proper
            my $i = 0;
            $i++ while $path[$i] ne $node;
            $found = [@path[$i .. $#path], $node];
            return;
        }
        $on_path{$node} = 1;
        push @path, $node;
        $walk->($_) for sort @{ $deps->{$node} || [] };
        pop @path;
        delete $on_path{$node};
    };
    $walk->($start);
    return $found || [$start, '???'];
}

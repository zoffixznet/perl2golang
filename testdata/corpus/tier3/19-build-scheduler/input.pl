#!/usr/bin/perl
# Build scheduler: parses a dependency file, topologically sorts with a
# priority-queue tie-break (hash-of-arrays), simulates 2 parallel workers,
# and proves cycle detection on a corrupted copy of the graph.
use strict;
use warnings;
use List::Util qw(max);

my $depfile = shift @ARGV // 'files/tasks.dep';

# ---- parse -------------------------------------------------------------
my ( %task, %deps, %rdeps );
open my $fh, '<', $depfile or die "open $depfile: $!\n";
while (<$fh>) {
    chomp;
    s/#.*//;
    next unless /\S/;
    my @f = split ' ';
    if ( $f[0] eq 'task' ) {
        die "duplicate task $f[1]\n" if exists $task{ $f[1] };
        $task{ $f[1] } = { duration => $f[2], priority => $f[3] };
    }
    elsif ( $f[0] eq 'dep' ) {
        my ( $t, $pre ) = @f[ 1, 2 ];
        push @{ $deps{$t} },   $pre;
        push @{ $rdeps{$pre} }, $t;
    }
    else { die "bad directive '$f[0]' at line $.\n" }
}
close $fh;
for my $t ( sort keys %deps ) {
    exists $task{$_} or die "dep on unknown task '$_'\n" for @{ $deps{$t} };
}

# ---- priority queue: hash of arrays keyed by priority ------------------
# push/pop keep FIFO order within a priority; highest priority pops first;
# alphabetical tie-break happens at insert time via sorted feeding.
{

    package PQueue;

    sub new { bless { q => {}, n => 0 }, shift }

    sub add {
        my ( $self, $prio, $item ) = @_;
        push @{ $self->{q}{$prio} }, $item;
        $self->{n}++;
        return;
    }

    sub take {
        my ($self) = @_;
        return undef unless $self->{n};
        my ($top) = sort { $b <=> $a } grep { @{ $self->{q}{$_} } }
          keys %{ $self->{q} };
        $self->{n}--;
        return shift @{ $self->{q}{$top} };
    }

    sub count { $_[0]{n} }
}

# ---- Kahn toposort with priority tie-break -----------------------------
sub schedule {
    my ( $tasks, $deps ) = @_;
    my %indegree = map { $_ => scalar @{ $deps->{$_} // [] } } keys %$tasks;
    my $ready    = PQueue->new;
    $ready->add( $tasks->{$_}{priority}, $_ )
      for sort grep { !$indegree{$_} } keys %indegree;

    my @order;
    while ( $ready->count ) {
        my $t = $ready->take;
        push @order, $t;
        for my $succ ( sort @{ $rdeps{$t} // [] } ) {
            $ready->add( $tasks->{$succ}{priority}, $succ )
              if --$indegree{$succ} == 0;
        }
    }
    if ( @order != keys %$tasks ) {
        my @stuck = sort grep { $indegree{$_} > 0 } keys %indegree;
        die "cycle detected involving: @stuck\n";
    }
    return @order;
}

my @order = schedule( \%task, \%deps );
print "topological order (priority tie-break):\n";
printf "  %2d. %-14s p=%d d=%d\n", $_ + 1, $order[$_],
  $task{ $order[$_] }{priority}, $task{ $order[$_] }{duration}
  for 0 .. $#order;

# ---- simulate two workers ----------------------------------------------
my %done_at;    # task -> completion time
my @workers = ( 0, 0 );    # next-free time per worker
for my $t (@order) {
    my $earliest = max( 0, map { $done_at{$_} } @{ $deps{$t} // [] } );
    # pick the worker free soonest; tie -> lower index (deterministic)
    my ($widx) = sort { $workers[$a] <=> $workers[$b] or $a <=> $b } 0 .. $#workers;
    my $start = max( $earliest, $workers[$widx] );
    $done_at{$t} = $start + $task{$t}{duration};
    $workers[$widx] = $done_at{$t};
    printf "t=%2d..%2d w%d %s\n", $start, $done_at{$t}, $widx, $t;
}
printf "makespan: %d time units\n", max( values %done_at );

# critical path length via memoized DFS
my %memo;
sub cp {
    my ($t) = @_;
    $memo{$t} //= $task{$t}{duration} +
      max( 0, map { cp($_) } @{ $deps{$t} // [] } );
}
my ($crit) = sort { cp($b) <=> cp($a) or $a cmp $b } keys %task;
printf "critical path: %d units (ends at %s)\n", cp($crit), $crit;

# ---- cycle detection on a corrupted graph ------------------------------
my %broken = map { $_ => [ @{ $deps{$_} // [] } ] } keys %task;
push @{ $broken{'fetch-src'} }, 'publish';    # closes the loop
if ( eval { schedule( \%task, \%broken ); 1 } ) {
    print "corrupt graph unexpectedly scheduled\n";
}
else { print "corrupt graph: $@" }

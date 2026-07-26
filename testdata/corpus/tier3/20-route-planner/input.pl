#!/usr/bin/perl
# Transit route planner: Dijkstra over a hash-of-hashes adjacency map with
# deterministic tie-breaking, path reconstruction, reachability report,
# and an all-pairs eccentricity summary for the connected component.
use strict;
use warnings;
use List::Util qw(min max);

my ( $file, @queries ) = @ARGV;
$file ||= 'files/network.txt';
@queries = ( 'Depot:Airport', 'Riverside:Uptown', 'Central:Island' )
  unless @queries;

# ---- load graph --------------------------------------------------------
my %edge;    # $edge{from}{to} = minutes
open my $fh, '<', $file or die "open $file: $!\n";
while (<$fh>) {
    next if /^\s*(#|$)/;
    my $directed = s/^>\s*//;
    my ( $a, $b, $w ) = split ' ';
    die "bad edge line: $_" unless defined $w && $w =~ /^\d+$/;
    die "duplicate edge $a-$b\n" if exists $edge{$a}{$b};
    $edge{$a}{$b} = $w;
    $edge{$b}{$a} = $w unless $directed;
}
close $fh;
my @stations = sort keys %edge;
printf "network: %d stations, %d directed edges\n",
  scalar @stations, scalar( map { keys %$_ } values %edge );

# ---- dijkstra ----------------------------------------------------------
sub shortest {
    my ( $src, $dst ) = @_;
    die "unknown station '$src'\n" unless exists $edge{$src};
    my %dist = ( $src => 0 );
    my %prev;
    my %visited;

    while (1) {
        # deterministic extract-min: distance, then name
        my ($u) =
          sort { $dist{$a} <=> $dist{$b} or $a cmp $b }
          grep { !$visited{$_} } keys %dist;
        last unless defined $u;
        last if defined $dst && $u eq $dst;
        $visited{$u} = 1;
        for my $v ( sort keys %{ $edge{$u} } ) {
            my $cand = $dist{$u} + $edge{$u}{$v};
            if ( !exists $dist{$v} || $cand < $dist{$v} ) {
                $dist{$v} = $cand;
                $prev{$v} = $u;
            }
        }
    }
    return ( \%dist, \%prev );
}

sub route {
    my ( $src, $dst ) = @_;
    my ( $dist, $prev ) = shortest( $src, $dst );
    return unless exists $dist->{$dst};
    my @path = ($dst);
    unshift @path, $prev->{ $path[0] } while exists $prev->{ $path[0] };
    return ( $dist->{$dst}, @path );
}

# ---- answer queries ----------------------------------------------------
for my $q (@queries) {
    my ( $src, $dst ) = split /:/, $q;
    my ( $cost, @path ) = eval { route( $src, $dst ) };
    if ($@)          { chomp( my $e = $@ ); print "$q: error: $e\n" }
    elsif ( !@path ) { print "$q: unreachable\n" }
    else {
        printf "%s: %d min via %s\n", $q, $cost, join ' -> ', @path;
    }
}

# ---- eccentricity table for everything reachable from Central ----------
my ($dist_central) = shortest('Central');
my @reachable = sort grep { $_ ne 'Central' } keys %$dist_central;
print "--- from Central ---\n";
printf "  %-10s %3d min\n", $_, $dist_central->{$_}
  for sort { $dist_central->{$a} <=> $dist_central->{$b} or $a cmp $b }
  @reachable;

my @unreached = grep { !exists $dist_central->{$_} } @stations;
print "unreachable from Central: @unreached\n";

# farthest-apart pair among a fixed probe set (all-pairs over 4 stations)
my @probe = qw(Central Docks Uptown Stadium);
my ( $best, @bestpair ) = (-1);
for my $i ( 0 .. $#probe - 1 ) {
    for my $j ( $i + 1 .. $#probe ) {
        my ($d) = shortest( $probe[$i] );
        my $val = $d->{ $probe[$j] };
        ( $best, @bestpair ) = ( $val, @probe[ $i, $j ] ) if $val > $best;
    }
}
printf "widest probe gap: %s <-> %s = %d min\n", @bestpair, $best;

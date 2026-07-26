#!/usr/bin/perl
# cidr-report -- network plan sanity checks + longest-prefix IP lookup.
#
# All IPv4-as-u32 integer math; no Net::IP on the jump hosts, so the
# CIDR class below has lived here since 2013.  Nesting (a subnet wholly
# inside a bigger allocation) is expected and reported as containment;
# *partial* overlap between plan entries is a plan bug.
use strict;
use warnings;

# ======================================================================
package CIDR;

sub new {
    my ($class, $spec, $label) = @_;
    my ($quad, $len) = $spec =~ m{^(\d+\.\d+\.\d+\.\d+)/(\d+)$}
        or die "bad CIDR spec '$spec'\n";
    die "bad prefix length /$len\n" if $len > 32;

    my $self = bless {
        spec  => $spec,
        label => $label // '',
        len   => $len,
        addr  => CIDR::ip_to_u32($quad),
    }, $class;

    # mask: careful with the /0 edge -- (1<<32) overflows on 32-bit
    # perls, hence the ternary rather than the obvious shift
    $self->{mask}    = $len == 0 ? 0 : (0xFFFFFFFF << (32 - $len)) & 0xFFFFFFFF;
    $self->{network} = $self->{addr} & $self->{mask};
    $self->{bcast}   = $self->{network} | (~$self->{mask} & 0xFFFFFFFF);

    die "'$spec' has host bits set (network is @{[ CIDR::u32_to_ip($self->{network}) ]})\n"
        if $self->{network} != $self->{addr};
    return $self;
}

sub network  { $_[0]{network} }
sub bcast    { $_[0]{bcast} }
sub len      { $_[0]{len} }
sub label    { $_[0]{label} }
sub spec     { $_[0]{spec} }

sub hosts {
    my ($self) = @_;
    # /31 and /32 have no network/broadcast convention
    return 1 if $self->{len} == 32;
    return 2 if $self->{len} == 31;
    return 2 ** (32 - $self->{len}) - 2;
}

sub contains_ip {
    my ($self, $u32) = @_;
    return ($u32 & $self->{mask}) == $self->{network};
}

sub contains_net {
    my ($self, $other) = @_;
    return $self->{len} <= $other->{len}
        && $self->contains_ip($other->{network});
}

sub overlaps {
    my ($self, $other) = @_;
    return $self->{network} <= $other->{bcast}
        && $other->{network} <= $self->{bcast};
}

sub ip_to_u32 {
    my ($quad) = @_;
    my @o = split /\./, $quad;
    die "bad octet in '$quad'\n" if grep { $_ > 255 or /^0\d/ } @o;
    return ($o[0] << 24) | ($o[1] << 16) | ($o[2] << 8) | $o[3];
}

sub u32_to_ip {
    my ($u) = @_;
    return join '.', ($u >> 24) & 0xFF, ($u >> 16) & 0xFF, ($u >> 8) & 0xFF, $u & 0xFF;
}

# ======================================================================
package main;

my ($net_file, $ip_file) = @ARGV;
die "usage: $0 <networks.txt> <ips.txt>\n" unless defined $ip_file;

# ---- load plan ----
my @nets;
open my $nf, '<', $net_file or die "open $net_file: $!\n";
while (<$nf>) {
    next if /^\s*(?:#|$)/;
    chomp;
    my ($spec, $label) = split /\t/;
    push @nets, CIDR->new($spec, $label);
}
close $nf;

# stable order: by network address, then most-specific first
@nets = sort { $a->network <=> $b->network or $b->len <=> $a->len } @nets;

printf "%-18s %-15s %-15s %10s  %s\n",
    'CIDR', 'NETWORK', 'BROADCAST', 'HOSTS', 'LABEL';
for my $n (@nets) {
    printf "%-18s %-15s %-15s %10d  %s\n",
        $n->spec, CIDR::u32_to_ip($n->network), CIDR::u32_to_ip($n->bcast),
        $n->hosts, $n->label;
}

# ---- containment / overlap audit ----
print "\nplan audit:\n";
my $problems = 0;
for my $i (0 .. $#nets) {
    for my $j ($i + 1 .. $#nets) {
        my ($a, $b) = @nets[$i, $j];
        next unless $a->overlaps($b);
        if    ($a->contains_net($b)) { printf "  nested:  %s inside %s\n", $b->spec, $a->spec }
        elsif ($b->contains_net($a)) { printf "  nested:  %s inside %s\n", $a->spec, $b->spec }
        else {
            printf "  PARTIAL OVERLAP: %s vs %s\n", $a->spec, $b->spec;
            $problems++;
        }
    }
}
print "  no partial overlaps\n" unless $problems;

# ---- longest-prefix match for the IP list ----
print "\nip assignments (longest prefix wins):\n";
open my $if, '<', $ip_file or die "open $ip_file: $!\n";
while (<$if>) {
    chomp;
    next unless /\S/;
    my $u = eval { CIDR::ip_to_u32($_) };
    if (!defined $u) { printf "  %-15s INVALID\n", $_; next }
    my ($best) = sort { $b->len <=> $a->len }
                 grep { $_->contains_ip($u) } @nets;
    printf "  %-15s %s\n", $_,
        $best ? sprintf('%s (%s)', $best->spec, $best->label) : 'unrouted';
}
close $if;
exit($problems ? 1 : 0);

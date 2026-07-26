#!/usr/bin/perl
# zonelint -- parse a BIND zone file and run the checks that have
# actually bitten us: CNAME-and-other-data, CNAME chains, MX pointing
# at CNAMEs, dangling in-zone targets.
#
# Handles the usual zone-file головоломка... er, puzzles: $TTL/$ORIGIN
# directives, '@' owner, blank owner meaning "same as previous", the
# multi-line parenthesised SOA, and per-record TTL overrides.
use strict;
use warnings;

my $file = shift @ARGV or die "usage: $0 <zonefile>\n";
open my $fh, '<', $file or die "open $file: $!\n";

my ($origin, $default_ttl) = ('', 3600);
my $prev_owner;
my %zone;      # fqdn -> type -> list of { ttl, data, line }
my @order;     # fqdn first-seen order
my @problems;

while (my $line = <$fh>) {
    chomp $line;

    # glue parenthesised records into one logical line (SOA mostly);
    # strip ;-comments per physical line BEFORE gluing, or the ')' hides
    $line = strip_comment($line);
    while ($line =~ tr/(// > $line =~ tr/)//) {
        my $more = <$fh>;
        last unless defined $more;
        chomp $more;
        $line .= ' ' . strip_comment($more);
    }
    $line =~ s/[()]/ /g;

    next unless $line =~ /\S/;

    if ($line =~ /^\$(\w+)\s+(\S+)/) {
        my ($dir, $val) = ($1, $2);
        if    ($dir eq 'ORIGIN') { $origin = $val; $prev_owner = undef }
        elsif ($dir eq 'TTL')    { $default_ttl = $val }
        else { push @problems, "line $.: unknown directive \$$dir" }
        next;
    }

    # owner is present unless the line starts with whitespace
    my $owner;
    if ($line =~ s/^(\S+)\s+//) {
        $owner = $1;
    } else {
        $line =~ s/^\s+//;
        $owner = $prev_owner;
        if (!defined $owner) {
            push @problems, "line $.: continuation record with no previous owner";
            next;
        }
    }
    $prev_owner = $owner;

    my @f = split ' ', $line;
    my $ttl = $default_ttl;
    $ttl = shift @f if @f and $f[0] =~ /^\d+$/;
    shift @f if @f and uc $f[0] eq 'IN';        # class, always IN here
    my $type = uc(shift @f // '');
    unless ($type =~ /^(?:SOA|NS|A|AAAA|MX|TXT|CNAME|PTR|SRV)$/) {
        push @problems, "line $.: unsupported record type '$type'";
        next;
    }
    my $data = join ' ', @f;

    my $fqdn = qualify($owner);
    push @order, $fqdn unless exists $zone{$fqdn};
    push @{ $zone{$fqdn}{$type} }, { ttl => $ttl, data => $data, line => $. };
}
close $fh;

# ---- summary ----
my %type_count;
for my $fqdn (keys %zone) {
    for my $type (keys %{ $zone{$fqdn} }) {
        $type_count{$type} += @{ $zone{$fqdn}{$type} };
    }
}
print "zone: ", scalar @order, " names, ",
    join(', ', map { "$_=$type_count{$_}" } sort keys %type_count), "\n\n";

print "records:\n";
for my $fqdn (@order) {
    for my $type (sort keys %{ $zone{$fqdn} }) {
        for my $r (@{ $zone{$fqdn}{$type} }) {
            printf "  %-24s %6d %-6s %s\n", $fqdn, $r->{ttl}, $type,
                $type eq 'SOA' ? soa_brief($r->{data}) : $r->{data};
        }
    }
}

# ---- lint ----
for my $fqdn (sort keys %zone) {
    my $types = $zone{$fqdn};
    if ($types->{CNAME}) {
        my @other = grep { $_ ne 'CNAME' } keys %$types;
        push @problems, "$fqdn: CNAME coexists with " . join(',', sort @other)
            if @other;
        push @problems, "$fqdn: multiple CNAME records"
            if @{ $types->{CNAME} } > 1;
        my $target = qualify($types->{CNAME}[0]{data});
        push @problems, "$fqdn: CNAME chain via $target"
            if $zone{$target} and $zone{$target}{CNAME};
    }
    for my $mx (@{ $types->{MX} || [] }) {
        my ($prio, $host) = split ' ', $mx->{data};
        my $t = qualify($host);
        push @problems, "$fqdn: MX $prio targets CNAME $t"
            if $zone{$t} and $zone{$t}{CNAME};
        push @problems, "$fqdn: MX $prio targets missing in-zone host $t"
            if in_zone($t) and !$zone{$t};
    }
}

print "\nproblems: ", scalar @problems, "\n";
print "  $_\n" for sort @problems;
exit(@problems ? 1 : 0);

# ----------------------------------------------------------------------
sub strip_comment {
    my ($s) = @_;
    # naive but fine for our zones: no ';' inside TXT strings... except
    # the SPF one, so respect double quotes
    my ($out, $inq) = ('', 0);
    for my $c (split //, $s) {
        $inq = !$inq if $c eq '"';
        last if $c eq ';' and !$inq;
        $out .= $c;
    }
    return $out;
}

sub qualify {
    my ($name) = @_;
    return $origin                     if $name eq '@';
    return $name                       if $name =~ /\.$/;   # already absolute
    return "$name.$origin";
}

sub in_zone {
    my ($fqdn) = @_;
    # "in zone" means under the *apex* origin, i.e. the first $ORIGIN
    return $fqdn =~ /(?:^|\.)\Qexample.com.\E$/;   # site-specific, sorry
}

sub soa_brief {
    my ($data) = @_;
    my @f = split ' ', $data;
    return "$f[0] $f[1] serial=$f[2] refresh=$f[3] retry=$f[4] expire=$f[5] min=$f[6]";
}

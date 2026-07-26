#!/usr/bin/perl
# contact-dedupe -- find likely duplicate contacts in a TSV export.
#
# Two records are considered duplicates when they share a zip code AND
# their normalised names are within a small edit distance (or one name
# is the nickname form of the other -- Bob/Robert etc.).  Exact email
# matches short-circuit everything.  Union-find keeps clusters
# transitive: if A~B and B~C then A,B,C land in one cluster even when
# A and C would not match directly.
use strict;
use warnings;

my %NICK = (
    bob => 'robert',  rob => 'robert',  bobby => 'robert',
    liz => 'elizabeth', beth => 'elizabeth',
    bill => 'william', will => 'william', wm => 'william',
    pat => 'patrick',       # ambiguous (patricia) but our data skews this way
    janette => 'janette',   # NOT a form of "jane"; guard added after a bad merge
);

my $MAX_DIST = 2;

my $file = shift @ARGV or die "usage: $0 <contacts.tsv>\n";
open my $fh, '<', $file or die "open $file: $!\n";
my $hdr = <$fh>;
my @contacts;
while (<$fh>) {
    chomp;
    my ($id, $name, $email, $zip) = split /\t/;
    next unless defined $zip;
    push @contacts, {
        id    => $id,
        name  => $name,
        email => lc $email,
        zip   => $zip,
        norm  => normalize_name($name),
    };
}
close $fh;

# ---- union-find over pairwise comparisons ----
my %parent = map { $_->{id} => $_->{id} } @contacts;

for my $i (0 .. $#contacts) {
    for my $j ($i + 1 .. $#contacts) {
        my ($a, $b) = @contacts[$i, $j];
        union($a->{id}, $b->{id}) if is_dup($a, $b);
    }
}

# ---- collect clusters ----
my %cluster;
push @{ $cluster{ find($_->{id}) } }, $_ for @contacts;

my @dupes = grep { @{ $cluster{$_} } > 1 } sort keys %cluster;
my $singles = grep { @{ $cluster{$_} } == 1 } keys %cluster;

printf "%d contacts, %d duplicate cluster(s), %d unique\n\n",
    scalar @contacts, scalar @dupes, $singles;

for my $root (@dupes) {
    my @members = sort { $a->{id} cmp $b->{id} } @{ $cluster{$root} };
    # survivor: most fields filled, then longest email, then lowest id;
    # "most fields" is a leftover from when phone/address existed
    my ($survivor) = sort {
        length($b->{email}) <=> length($a->{email})
            or $a->{id} cmp $b->{id}
    } @members;
    print "cluster (keep $survivor->{id}):\n";
    for my $m (@members) {
        printf "  %s %-6s %-22s %-30s %s\n",
            ($m == $survivor ? '*' : '-'),
            $m->{id}, $m->{name}, $m->{email}, $m->{zip};
    }
    print "\n";
}
exit(@dupes ? 1 : 0);

# ----------------------------------------------------------------------
sub is_dup {
    my ($a, $b) = @_;
    return 1 if $a->{email} eq $b->{email};
    return 0 unless $a->{zip} eq $b->{zip};
    return 0 if $a->{norm} eq '' or $b->{norm} eq '';
    return 1 if $a->{norm} eq $b->{norm};
    return 1 if levenshtein($a->{norm}, $b->{norm}) <= $MAX_DIST;
    return 0;
}

sub normalize_name {
    my ($name) = @_;
    my $n = lc $name;
    $n =~ s/[^a-z\s]//g;          # drop punctuation (O'Neil -> oneil)
    $n =~ s/\s+/ /g;
    $n =~ s/^\s+|\s+$//g;
    # first-token nickname folding
    my @parts = split / /, $n;
    if (@parts and exists $NICK{ $parts[0] }) {
        $parts[0] = $NICK{ $parts[0] };
    }
    return join ' ', @parts;
}

# textbook two-row DP; ~n*m but names are short so nobody has cared
sub levenshtein {
    my ($s, $t) = @_;
    my @s = split //, $s;
    my @t = split //, $t;
    my @prev = (0 .. scalar @t);
    for my $i (1 .. @s) {
        my @cur = ($i);
        for my $j (1 .. @t) {
            my $cost = $s[$i - 1] eq $t[$j - 1] ? 0 : 1;
            my $min = $prev[$j] + 1;                       # deletion
            $min = $cur[$j - 1] + 1 if $cur[$j - 1] + 1 < $min;   # insertion
            $min = $prev[$j - 1] + $cost if $prev[$j - 1] + $cost < $min;
            push @cur, $min;
        }
        @prev = @cur;
    }
    return $prev[-1];
}

sub find {
    my ($x) = @_;
    $parent{$x} = find($parent{$x}) if $parent{$x} ne $x;   # path compression
    return $parent{$x};
}

sub union {
    my ($x, $y) = @_;
    my ($rx, $ry) = (find($x), find($y));
    return if $rx eq $ry;
    # smaller id becomes the root so cluster keys are deterministic
    ($rx, $ry) = ($ry, $rx) if $ry lt $rx;
    $parent{$ry} = $rx;
}

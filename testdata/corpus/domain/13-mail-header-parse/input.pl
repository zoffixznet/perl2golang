#!/usr/bin/perl
# hdrparse -- unfold and audit RFC 5322 message headers from STDIN.
#
# Used by the list-server ops to eyeball delivery paths without opening
# the whole message in a client.  Understands folded headers, repeated
# headers (Received keeps them all, in order), case-insensitive names,
# and just enough RFC 2047 to make Q-encoded subjects readable.
use strict;
use warnings;

# ---- slurp header block (stop at first blank line) ----
my @raw;
while (my $line = <STDIN>) {
    $line =~ s/\r?\n\z//;
    last if $line eq '';
    push @raw, $line;
}

# ---- unfold: continuation lines start with SP or TAB ----
my @unfolded;
for my $line (@raw) {
    if ($line =~ /^[ \t]/ and @unfolded) {
        (my $cont = $line) =~ s/^[ \t]+/ /;
        $unfolded[-1] .= $cont;
    } else {
        push @unfolded, $line;
    }
}

# ---- parse into an order-preserving multi-map ----
my (%headers, @order);       # lc name -> [values...], first-seen order
my $bad = 0;
for my $line (@unfolded) {
    unless ($line =~ /^([!-9;-~]+):\s*(.*)$/) {   # RFC: field-name is printable minus colon
        $bad++;
        next;
    }
    my ($name, $value) = (lc $1, $2);
    push @order, $name unless exists $headers{$name};
    push @{ $headers{$name} }, $value;
}

printf "%d raw lines -> %d headers (%d names, %d unparseable)\n\n",
    scalar @raw, scalar @unfolded, scalar @order, $bad;

# ---- the fields ops actually read, in fixed order ----
for my $want (qw(from to subject date message-id)) {
    my $v = $headers{$want} ? $headers{$want}[0] : '(missing)';
    $v = decode_q($v) if $want eq 'subject';
    $v = join_addresses($v) if $want eq 'to';
    printf "%-11s %s\n", ucfirst($want) . ':', $v;
}

# ---- the Received chain, oldest first (headers stack newest-on-top) ----
my @received = @{ $headers{received} || [] };
print "\ndelivery path (", scalar @received, " hops, oldest first):\n";
my $hop = 0;
for my $r (reverse @received) {
    $hop++;
    # "from HOST (COMMENT) by HOST with PROTO id ID for <ADDR>; DATE"
    if ($r =~ m{
            ^ from \s+ (?<from> \S+ )
            .*?  by   \s+ (?<by>   \S+ )
            (?: .*? with \s+ (?<with> \S+ ) )?
            (?: .*? ;    \s* (?<date> .+ ) )?
            $
        }xi) {
        printf "  %d. %s -> %s%s%s\n", $hop, $+{from}, $+{by},
            defined $+{with} ? " ($+{with})" : '',
            defined $+{date} ? " at $+{date}" : '';
    } else {
        printf "  %d. (unparsed) %s\n", $hop, substr($r, 0, 50);
    }
}

# ---- quick hygiene checks ----
my @notes;
push @notes, 'no DKIM signature'      unless $headers{'dkim-signature'};
push @notes, 'multiple From headers'  if ($headers{from} && @{ $headers{from} }) > 1;
push @notes, 'missing Message-ID'     unless $headers{'message-id'};
my $spam = $headers{'x-spam-score'} ? $headers{'x-spam-score'}[0] : undef;
push @notes, "spam score $spam" if defined $spam and $spam > 5;
print "\nchecks: ", (@notes ? join('; ', @notes) : 'all clean'), "\n";
exit 0;

# ----------------------------------------------------------------------
# Minimal RFC 2047: only Q-encoding, only one charset, which is all our
# lists ever produce.  B-encoding falls through untouched on purpose.
sub decode_q {
    my ($s) = @_;
    $s =~ s{=\?[\w-]+\?[Qq]\?(.*?)\?=}{
        my $t = $1;
        $t =~ tr/_/ /;
        $t =~ s/=([0-9A-Fa-f]{2})/chr hex $1/ge;
        $t;
    }ge;
    return $s;
}

sub join_addresses {
    my ($s) = @_;
    # collapse the whitespace that unfolding left behind
    my @addrs = split /\s*,\s*/, $s;
    return join ', ', grep { length } @addrs;
}

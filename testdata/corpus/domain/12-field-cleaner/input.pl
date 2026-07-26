#!/usr/bin/perl
# field-cleaner -- normalise the pipe-delimited CRM export.
#
# Every field type has its own cleaner sub in %CLEANERS; the loop applies
# whichever ones the header says are present, so column order in the
# export can change (it does, every time marketing "improves" it).
# Cleaners return (cleaned_value, changed_flag, note-or-undef).
use strict;
use warnings;

my %STATES = (
    texas => 'TX', california => 'CA', 'new york' => 'NY',
    tx => 'TX', ca => 'CA', ny => 'NY', fl => 'FL', florida => 'FL',
);

my %MONTHS = (
    jan => 1, feb => 2, mar => 3, apr => 4, may => 5, jun => 6,
    jul => 7, aug => 8, sep => 9, oct => 10, nov => 11, dec => 12,
);

my %CLEANERS = (
    name   => \&clean_name,
    email  => \&clean_email,
    phone  => \&clean_phone,
    joined => \&clean_date,
    state  => \&clean_state,
);

# ---- read header, wire up cleaner pipeline ----
my $hdr = <STDIN>;
die "no input\n" unless defined $hdr;
chomp $hdr;
my @cols = split /\|/, $hdr;
my @pipeline = map { $CLEANERS{$_} || \&clean_passthrough } @cols;

print join('|', @cols), "\n";

my %changes;     # column name -> count of changed values
my @rejects;     # [line, column, note]
my $rows = 0;

while (my $line = <STDIN>) {
    chomp $line;
    # continuation glue: the exporter wraps long emails onto a second
    # physical line starting with no delimiter at all (seen 2022-04)
    while ($line =~ tr/|// < $#cols and defined(my $more = <STDIN>)) {
        chomp $more;
        $line .= $more;
    }
    $rows++;
    my @f = split /\|/, $line, -1;
    my @out;
    for my $i (0 .. $#cols) {
        my ($clean, $changed, $note) = $pipeline[$i]->($f[$i] // '');
        push @out, $clean;
        $changes{ $cols[$i] }++ if $changed;
        push @rejects, [$rows, $cols[$i], $note] if defined $note;
    }
    print join('|', @out), "\n";
}

print "# --- cleaning report ---\n";
printf "# rows: %d\n", $rows;
for my $col (grep { $changes{$_} } @cols) {
    printf "# %-8s %d value(s) rewritten\n", $col, $changes{$col};
}
printf "# UNFIXABLE row %d %s: %s\n", @$_ for @rejects;
exit(@rejects ? 1 : 0);

# ---------------- cleaners ----------------
# each: ($raw) -> ($clean, $changed, $note_if_unfixable)

sub clean_passthrough {
    my ($v) = @_;
    return ($v, 0, undef);
}

sub squeeze {
    my ($v) = @_;
    $v =~ s/^\s+|\s+$//g;
    $v =~ s/\s+/ /g;
    return $v;
}

sub clean_name {
    my ($raw) = @_;
    my $v = squeeze(lc $raw);
    # title case with the usual surname exceptions
    $v =~ s/\b(\w)/\u$1/g;
    $v =~ s/\bMc(\w)/Mc\u$1/g;             # McGregor, not Mcgregor
    $v =~ s/\bO'(\w)/O'\u$1/g;             # O'Neil
    return ($v, $v ne $raw ? 1 : 0, undef);
}

sub clean_email {
    my ($raw) = @_;
    my $v = squeeze($raw);
    $v =~ s/\s+//g;                        # wrapped emails re-joined above
    $v = lc $v;
    # exactly one @, at least one dot after it -- deliberately naive,
    # full RFC 5322 was rejected as overkill in review
    if ($v =~ /^[^@\s]+@[^@\s]+\.[^@\s]+$/) {
        return ($v, $v ne $raw ? 1 : 0, undef);
    }
    return ($raw, 0, "bad email '$v'");
}

sub clean_phone {
    my ($raw) = @_;
    (my $digits = $raw) =~ tr/0-9//cd;     # strip everything non-digit
    $digits =~ s/^1(?=\d{10}$)//;          # drop US country code
    if (length $digits == 10) {
        return (sprintf('(%s) %s-%s',
            substr($digits, 0, 3), substr($digits, 3, 3), substr($digits, 6)),
            1, undef);
    }
    if (length $digits == 7) {
        return (sprintf('%s-%s', substr($digits, 0, 3), substr($digits, 3)),
            1, undef);
    }
    return ($raw, 0, 'phone has ' . length($digits) . ' digits');
}

sub clean_date {
    my ($raw) = @_;
    my $v = squeeze($raw);
    my ($y, $m, $d);
    if ($v =~ m{^(\d{4})-(\d{1,2})-(\d{1,2})$}) {
        ($y, $m, $d) = ($1, $2, $3);
    } elsif ($v =~ m{^(\d{1,2})/(\d{1,2})/(\d{2,4})$}) {
        ($m, $d, $y) = ($1, $2, $3);
        $y += $y < 50 ? 2000 : 1900 if $y < 100;   # pivot chosen in 2003
    } elsif ($v =~ m{^([A-Za-z]{3})[a-z]* \s+ (\d{1,2}) ,? \s* (\d{4})$}x) {
        ($m, $d, $y) = ($MONTHS{ lc $1 }, $2, $3);
        return ($raw, 0, "unknown month in '$v'") unless $m;
    } else {
        return ($raw, 0, "unparseable date '$v'");
    }
    my @dim = (31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31);
    $dim[1] = 29 if $y % 4 == 0 and ($y % 100 != 0 or $y % 400 == 0);
    return ($raw, 0, "impossible date '$v'")
        if $m < 1 or $m > 12 or $d < 1 or $d > $dim[$m - 1];
    my $iso = sprintf '%04d-%02d-%02d', $y, $m, $d;
    return ($iso, $iso ne $raw ? 1 : 0, undef);
}

sub clean_state {
    my ($raw) = @_;
    my $v = lc squeeze($raw);
    $v =~ s/\.$//;                         # "Ca." style abbreviations
    if (my $code = $STATES{$v}) {
        return ($code, $code ne $raw ? 1 : 0, undef);
    }
    return ($raw, 0, "unknown state '$raw'");
}

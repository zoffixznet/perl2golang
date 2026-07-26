#!/usr/bin/perl
# csv2fixed -- convert CSV to the fixed-width layout the mainframe wants.
#
# Text::CSV is not on the intake host (don't ask), so the parser below is
# hand-rolled: handles quoted fields, embedded commas, doubled quotes,
# and quoted embedded newlines (multi-line records).  Anything the layout
# can't hold gets truncated and reported at the end.
use strict;
use warnings;

my ($layout_file, $csv_file) = @ARGV;
die "usage: $0 <layout> <input.csv>\n" unless defined $csv_file;

# ---- layout ----
my @layout;    # ordered list of { field, width, align, pad }
open my $lf, '<', $layout_file or die "open $layout_file: $!\n";
while (<$lf>) {
    next if /^\s*(?:#|$)/;
    chomp;
    my ($field, $width, $align, $pad) = split /:/;
    die "layout line $.: bad width '$width'\n" unless $width =~ /^\d+$/;
    $align ||= 'left';
    die "layout line $.: bad align '$align'\n"
        unless $align eq 'left' or $align eq 'right';
    push @layout, {
        field => $field, width => $width, align => $align,
        pad => (defined $pad and $pad eq 'zero') ? '0' : ' ',
    };
}
close $lf;

# ---- CSV ----
open my $fh, '<', $csv_file or die "open $csv_file: $!\n";
my $header = read_record($fh);
die "empty input\n" unless $header;
my %col;    # field name -> index
$col{ $header->[$_] } = $_ for 0 .. $#$header;

for my $l (@layout) {
    die "layout field '$l->{field}' not in CSV header\n"
        unless exists $col{ $l->{field} };
}

my ($records, @truncated) = (0);
while (my $rec = read_record($fh)) {
    $records++;
    my $out = '';
    for my $l (@layout) {
        my $val = $rec->[ $col{ $l->{field} } ];
        $val = '' unless defined $val;
        $val =~ s/\r?\n/ | /g;          # embedded newlines become ' | '
        $val =~ s/\t/ /g;               # the intake job chokes on tabs
        if (length $val > $l->{width}) {
            push @truncated, [$records, $l->{field}, length $val];
            $val = substr $val, 0, $l->{width};
        }
        if ($l->{align} eq 'right') {
            $out .= ($l->{pad} x ($l->{width} - length $val)) . $val;
        } else {
            $out .= $val . (' ' x ($l->{width} - length $val));
        }
    }
    print "$out\n";
}
close $fh;

print '-' x 10, "\n";
printf "%d records, %d truncation(s)\n", $records, scalar @truncated;
printf "  record %d field %s: %d chars\n", @$_ for @truncated;
exit(@truncated ? 3 : 0);   # 3 = "delivered but lossy" in the runbook

# ----------------------------------------------------------------------
# Read one logical CSV record, which may span physical lines when a
# quoted field contains newlines.  Returns arrayref of fields, or undef
# at EOF.  RFC 4180-ish; bare quotes inside unquoted fields pass through.
sub read_record {
    my ($fh) = @_;
    my $line = <$fh>;
    return undef unless defined $line;

    # keep pulling physical lines while quotes are unbalanced
    while (($line =~ tr/"//) % 2) {
        my $more = <$fh>;
        last unless defined $more;
        $line .= $more;
    }
    $line =~ s/\r?\n\z//;

    my @fields;
    my $field     = '';
    my $in_quotes = 0;
    my @chars = split //, $line;
    my $i = 0;
    while ($i < @chars) {
        my $c = $chars[$i];
        if ($in_quotes) {
            if ($c eq '"') {
                if (defined $chars[$i + 1] and $chars[$i + 1] eq '"') {
                    $field .= '"';    # doubled quote
                    $i++;
                } else {
                    $in_quotes = 0;
                }
            } else {
                $field .= $c;
            }
        } else {
            if    ($c eq '"' and $field eq '') { $in_quotes = 1 }
            elsif ($c eq ',') { push @fields, $field; $field = '' }
            else  { $field .= $c }
        }
        $i++;
    }
    push @fields, $field;
    return \@fields;
}

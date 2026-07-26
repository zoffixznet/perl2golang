#!/usr/bin/perl
# Hand-rolled CSV parser (no Text::CSV): quoted fields, doubled-quote
# escapes, embedded commas AND embedded newlines, then a category report.
use strict;
use warnings;
use List::Util qw(sum0 max);

my $file = shift @ARGV // 'files/products.csv';

# Character-by-character state machine parser. Returns arrayref of rows,
# each row an arrayref of fields.
sub parse_csv {
    my ($text) = @_;
    my @rows;
    my @field_row = ('');
    my $in_quotes = 0;
    my $i         = 0;
    my $len       = length $text;

    while ( $i < $len ) {
        my $c = substr $text, $i, 1;
        if ($in_quotes) {
            if ( $c eq '"' ) {
                if ( $i + 1 < $len && substr( $text, $i + 1, 1 ) eq '"' ) {
                    $field_row[-1] .= '"';    # doubled quote -> literal quote
                    $i++;
                }
                else { $in_quotes = 0 }
            }
            else { $field_row[-1] .= $c }     # commas/newlines kept verbatim
        }
        elsif ( $c eq '"' ) {
            die "stray quote mid-field at offset $i\n"
              if length $field_row[-1];
            $in_quotes = 1;
        }
        elsif ( $c eq ',' ) { push @field_row, '' }
        elsif ( $c eq "\n" ) {
            push @rows, [@field_row];
            @field_row = ('');
        }
        else { $field_row[-1] .= $c }
        $i++;
    }
    die "unterminated quoted field\n" if $in_quotes;
    push @rows, [@field_row]
      if @field_row > 1 || length $field_row[0];    # trailing partial line
    return \@rows;
}

my $text = do {
    open my $fh, '<', $file or die "open $file: $!\n";
    local $/;
    <$fh>;
};

my $rows   = parse_csv($text);
my $header = shift @$rows;
print "columns: ", join( '|', @$header ), "\n";
printf "data rows: %d\n\n", scalar @$rows;

# Build records keyed by header names (hash slice from two parallel lists).
my @records;
for my $row (@$rows) {
    die "row has " . @$row . " fields, want " . @$header . "\n"
      unless @$row == @$header;
    my %rec;
    @rec{@$header} = @$row;
    push @records, \%rec;
}

# Multi-line product names get flattened for display.
for my $r (@records) {
    ( my $flat = $r->{product} ) =~ s/\n/ /g;
    $r->{display} = $flat;
}

my %cat;
for my $r (@records) {
    push @{ $cat{ $r->{category} } }, $r;
}

for my $c ( sort keys %cat ) {
    my @in = sort { $a->{id} <=> $b->{id} } @{ $cat{$c} };
    printf "%s (%d items, total %.2f)\n", $c, scalar @in,
      sum0( map { $_->{price} } @in );
    for my $r (@in) {
        printf "  %-4s %-28s %8.2f  %s\n", $r->{id}, $r->{display},
          $r->{price}, ( $r->{notes} eq '' ? '-' : $r->{notes} );
    }
}

my $priciest = ( sort { $b->{price} <=> $a->{price} } @records )[0];
printf "\npriciest: %s (%s)\n", $priciest->{display}, $priciest->{price};
printf "longest name: %d chars\n",
  max( map { length $_->{display} } @records );

# Round-trip check: re-emit CSV with minimal quoting and reparse.
sub emit_field {
    my ($f) = @_;
    return $f unless $f =~ /[",\n]/;
    ( my $q = $f ) =~ s/"/""/g;
    return qq{"$q"};
}
my $out = join '',
  map { join( ',', map { emit_field($_) } @$_ ) . "\n" }
  ( $header, @$rows );
my $reparsed = parse_csv($out);
printf "round trip ok: %s\n",
  (
    join( "\x1f", map { join "\x1e", @$_ } @$reparsed ) eq
      join( "\x1f", map { join "\x1e", @$_ } ( $header, @$rows ) )
    ? 'yes'
    : 'no'
  );

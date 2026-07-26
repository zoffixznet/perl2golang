#!/usr/bin/perl
# Serialization workout: Storable freeze/thaw/dclone/store, Digest::MD5,
# MIME::Base64. All output derives from fixed inputs, never raw frozen bytes
# (those vary by Storable version, so we only assert round-trip fidelity).
use strict;
use warnings;
use Storable qw(freeze thaw dclone store retrieve);
use Digest::MD5 qw(md5_hex md5_base64);
use MIME::Base64 qw(encode_base64 decode_base64);
use File::Temp qw(tempdir);
use File::Spec;

# ---- build a gnarly nested structure from the manifest fixture ---------
open my $fh, '<', 'files/manifest.txt' or die "manifest: $!\n";
my %release;
while ( my $line = <$fh> ) {
    chomp $line;
    my ( $name, $size, $date ) = split ' ', $line;
    my ($base) = $name =~ /^(\w+)\./;
    $release{$base} = {
        file  => $name,
        size  => $size + 0,
        date  => $date,
        tags  => [ sort ( substr( $base, 0, 1 ), length($base) % 2 ? 'odd' : 'even' ) ],
    };
}
close $fh;
$release{_meta} = { count => scalar( keys %release ), schema => 2 };

sub structurally_equal {    # deep compare without Data::Dumper ordering woes
    my ( $a, $b ) = @_;
    return 0 if ref $a ne ref $b;
    if ( ref $a eq 'HASH' ) {
        return 0 unless keys %$a == keys %$b;
        for ( keys %$a ) {
            return 0 unless exists $b->{$_};
            return 0 unless structurally_equal( $a->{$_}, $b->{$_} );
        }
        return 1;
    }
    if ( ref $a eq 'ARRAY' ) {
        return 0 unless @$a == @$b;
        structurally_equal( $a->[$_], $b->[$_] ) or return 0 for 0 .. $#$a;
        return 1;
    }
    return ( !defined $a && !defined $b ) || ( defined $a && defined $b && $a eq $b );
}

# ---- in-memory freeze/thaw ---------------------------------------------
my $frozen = freeze( \%release );
my $thawed = thaw($frozen);
print "freeze/thaw equal: ",
  structurally_equal( \%release, $thawed ) ? 'yes' : 'no', "\n";
printf "frozen is a plain string of positive length: %s\n",
  ( !ref($frozen) && length($frozen) > 0 ) ? 'yes' : 'no';

# ---- dclone: mutating the copy must not touch the original -------------
my $copy = dclone( \%release );
$copy->{alpha}{size} = 1;
push @{ $copy->{beta}{tags} }, 'mutated';
printf "original alpha size intact: %s\n",
  $release{alpha}{size} == 1048576 ? 'yes' : 'no';
printf "original beta tags: %s\n", join ',', @{ $release{beta}{tags} };
printf "copy beta tags: %s\n",     join ',', @{ $copy->{beta}{tags} };

# ---- disk round trip through a temp dir --------------------------------
my $dir  = tempdir( CLEANUP => 1 );
my $path = File::Spec->catfile( $dir, 'release.stor' );
store( \%release, $path ) or die "store failed\n";
my $loaded = retrieve($path);
print "store/retrieve equal: ",
  structurally_equal( \%release, $loaded ) ? 'yes' : 'no', "\n";

# ---- MD5 over the fixture and derived data -----------------------------
my $manifest_text = do {
    open my $mfh, '<', 'files/manifest.txt' or die "manifest: $!\n";
    local $/;
    <$mfh>;
};
print "manifest md5: ",    md5_hex($manifest_text),    "\n";
print "manifest md5b64: ", md5_base64($manifest_text), "\n";

# Content-addressed ids for each record (from canonical field order).
for my $base ( sort grep { !/^_/ } keys %release ) {
    my $r  = $release{$base};
    my $id = md5_hex( join '|', $base, $r->{file}, $r->{size}, $r->{date} );
    printf "%-6s %s\n", $base, substr( $id, 0, 12 );
}

# ---- Base64 ------------------------------------------------------------
my $secret  = "release-key:\x00\x01\x02\xff";
my $b64     = encode_base64( $secret, '' );    # no line breaks
my $decoded = decode_base64($b64);
print "b64: $b64\n";
print "b64 round trip: ", ( $decoded eq $secret ? 'yes' : 'no' ), "\n";
printf "decoded length: %d bytes\n", length $decoded;

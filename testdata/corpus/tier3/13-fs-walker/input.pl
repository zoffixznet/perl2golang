#!/usr/bin/perl
# Directory auditor: File::Find with sorted traversal, pruning, File::Spec
# path work, per-extension rollup, and a "largest files" table.
use strict;
use warnings;
use File::Find;
use File::Spec;
use File::Basename qw(basename dirname fileparse);

my $root = shift @ARGV // File::Spec->catdir( 'files', 'project' );
die "no such directory: $root\n" unless -d $root;

my ( @files, %by_ext );
my ( $dirs_seen, $total_bytes ) = ( 0, 0 );

find(
    {
        preprocess => sub { sort @_ },    # deterministic traversal order
        wanted     => sub {
            if ( -d $_ ) {
                # prune dot-directories (fake VCS dir in the fixture)
                if ( /^\.(?!\.?$)/ ) {
                    $File::Find::prune = 1;
                    return;
                }
                $dirs_seen++;
                return;
            }
            return unless -f _;           # reuse cached stat
            my $rel  = File::Spec->abs2rel( $File::Find::name, $root );
            my $size = -s $_;
            my ( $base, $dir, $ext ) = fileparse( $rel, qr/\.[^.]+$/ );
            push @files,
              {
                rel  => $rel,
                size => $size,
                ext  => ( $ext eq '' ? '(none)' : $ext ),
                depth => scalar( File::Spec->splitdir($dir) ) - 1,
              };
            $by_ext{ $ext eq '' ? '(none)' : $ext }{count}++;
            $by_ext{ $ext eq '' ? '(none)' : $ext }{bytes} += $size;
            $total_bytes += $size;
        },
    },
    $root
);

printf "scanned %d dirs, %d files, %d bytes\n\n",
  $dirs_seen, scalar @files, $total_bytes;

print "TREE (relative, sorted)\n";
for my $f ( sort { $a->{rel} cmp $b->{rel} } @files ) {
    printf "  %s%-30s %4d\n", '  ' x $f->{depth},
      basename( $f->{rel} ), $f->{size};
}

print "\nBY EXTENSION\n";
for my $ext ( sort { $by_ext{$b}{bytes} <=> $by_ext{$a}{bytes} or $a cmp $b }
    keys %by_ext )
{
    printf "  %-8s %2d file(s) %5d bytes\n", $ext, $by_ext{$ext}{count},
      $by_ext{$ext}{bytes};
}

print "\nTOP 3 LARGEST\n";
my @largest =
  ( sort { $b->{size} <=> $a->{size} or $a->{rel} cmp $b->{rel} } @files )
  [ 0 .. 2 ];
printf "  %-24s %d\n", $_->{rel}, $_->{size} for @largest;

# File::Spec gymnastics on a known path.
my $sample = File::Spec->catfile( $root, 'src', 'util', 'Helper.pm' );
my ( $vol, $dir, $leaf ) = File::Spec->splitpath($sample);
print "\nPATH OPS\n";
printf "  dirname:  %s\n", dirname($sample);
printf "  basename: %s\n", basename($sample);
printf "  splitpath leaf: %s\n", $leaf;
printf "  updir join: %s\n",
  File::Spec->canonpath(
    File::Spec->catdir( $dir, File::Spec->updir, 'lib' ) );
printf "  is_absolute('%s'): %s\n", $sample,
  File::Spec->file_name_is_absolute($sample) ? 'yes' : 'no';

# Slurp one known file to prove read access + content-based work.
open my $fh, '<', $sample or die "open $sample: $!\n";
my @lines = <$fh>;
close $fh;
printf "  Helper.pm lines: %d, first: %s", scalar @lines, $lines[0];

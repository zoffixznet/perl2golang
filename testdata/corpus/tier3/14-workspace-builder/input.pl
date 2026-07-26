#!/usr/bin/perl
# Builds a project skeleton inside a File::Temp sandbox using
# File::Path::make_path, writes structured files, then audits what it made.
# Only relative paths ever reach stdout, so output is machine-independent.
use strict;
use warnings;
use File::Temp qw(tempdir tempfile);
use File::Path qw(make_path);
use File::Spec;
use File::Find;

my $app  = $ENV{SKEL_APP_NAME} // 'shipit';
my $work = tempdir( CLEANUP => 1 );

# ---- build the skeleton ------------------------------------------------
my @dirs = map { File::Spec->catdir( $work, $app, $_ ) }
  qw(bin lib/App t data/cache);
my $made = make_path(@dirs);
printf "make_path created %d directories\n", $made;

# Calling make_path again on existing dirs is a no-op creating nothing.
my $again = make_path(@dirs);
printf "second make_path created %d\n", $again;

my %files = (
    "bin/$app"        => "#!/usr/bin/perl\nuse App;\nApp->run;\n",
    'lib/App.pm'      => "package App;\nsub run { print \"running\\n\" }\n1;\n",
    't/00-load.t'     => "use Test::More tests => 1;\nuse_ok('App');\n",
    'data/cache/.keep' => '',
    'Makefile'        => "test:\n\tprove -l t\n",
);
for my $rel ( sort keys %files ) {
    my $path = File::Spec->catfile( $work, $app, split m{/}, $rel );
    open my $fh, '>', $path or die "write $rel: $!\n";
    print {$fh} $files{$rel};
    close $fh or die "close $rel: $!\n";
}

# A scratch tempfile inside the workspace (name never printed).
my ( $tfh, $tname ) =
  tempfile( 'scratch-XXXX', DIR => $work, UNLINK => 1 );
print {$tfh} "line $_\n" for 1 .. 5;
close $tfh;
printf "scratch tempfile matches template: %s\n",
  ( File::Spec->splitpath($tname) )[2] =~ /^scratch-\w{4}$/ ? 'yes' : 'no';

# ---- audit -------------------------------------------------------------
my @found;
find(
    {
        preprocess => sub { sort @_ },
        wanted     => sub {
            return unless -f $_;
            my $rel = File::Spec->abs2rel( $File::Find::name, $work );
            push @found, [ $rel, -s _ ];
        },
    },
    $work
);

print "--- workspace audit ---\n";
my $total = 0;
for my $f ( sort { $a->[0] cmp $b->[0] } @found ) {
    my ( $rel, $size ) = @$f;
    next if $rel =~ /^scratch-/;    # exclude the randomly named scratch file
    printf "%-18s %3d bytes%s\n", $rel, $size,
      ( $rel =~ /\.keep$/ ? ' (placeholder)' : '' );
    $total += $size;
}
printf "total (sans scratch): %d bytes\n", $total;

# ---- read back a file we wrote and execute a check on its content ------
my $appmod = File::Spec->catfile( $work, $app, 'lib', 'App.pm' );
open my $in, '<', $appmod or die "reopen App.pm: $!\n";
my $pkg;
while (<$in>) {
    $pkg = $1, last if /^package\s+([\w:]+);/;
}
close $in;
print "App.pm declares package: $pkg\n";

# -e / -d / -z file test operators against what we just built.
my $cache = File::Spec->catdir( $work, $app, 'data', 'cache' );
printf "cache dir exists: %s\n",   ( -d $cache                 ? 'yes' : 'no' );
printf ".keep is empty: %s\n",
  ( -z File::Spec->catfile( $cache, '.keep' ) ? 'yes' : 'no' );
printf "bin/%s executable bit: %s (not set; we never chmod'd)\n", $app,
  ( -x File::Spec->catfile( $work, $app, 'bin', $app ) ? 'yes' : 'no' );

# make_path error handling: a path through an existing FILE must fail.
my $bad = File::Spec->catdir( $work, $app, 'Makefile', 'sub' );
my $err;
make_path( $bad, { error => \$err } );
printf "make_path through a file failed as expected: %s\n",
  ( @$err ? 'yes' : 'no' );

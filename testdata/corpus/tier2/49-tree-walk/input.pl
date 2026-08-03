#!/usr/bin/perl
# Building a scratch tree, walking it, and taking it down again. Every path is
# under a temporary directory whose name differs on every run, so nothing
# prints a path: what prints is what was made, what the walk found, and what
# is left afterwards.
use strict;
use warnings;
use File::Temp qw(tempdir tempfile);
use File::Path qw(make_path remove_tree);
use File::Find;
use File::Spec;

my $root = tempdir( CLEANUP => 1 );
printf "root is a directory: %s\n", ( -d $root ? 'yes' : 'no' );

my @dirs = map { File::Spec->catdir( $root, @$_ ) }
    ( [ 'src', 'lib' ], [ 'src', 'bin' ], [ 'doc' ] );
my @made = make_path(@dirs);
printf "created %d directories\n", scalar @made;
printf "created %d on a second pass\n", scalar make_path(@dirs);

for my $spec (
    [ 'src/lib/core.pm', "package Core;\n1;\n" ],
    [ 'src/lib/util.pm', "package Util;\n1;\n" ],
    [ 'src/bin/run.pl',  "#!/usr/bin/perl\nprint \"hi\\n\";\n" ],
    [ 'doc/readme.md',   "# readme\n" ],
    )
{
    my ( $rel, $text ) = @$spec;
    my $path = File::Spec->catfile( $root, split m{/}, $rel );
    open my $fh, '>', $path or die "cannot write $rel: $!";
    print {$fh} $text;
    close $fh;
}

my ( $scratch, $scratch_name ) = tempfile( 'work-XXXX', DIR => $root, SUFFIX => '.tmp' );
print {$scratch} "scratch\n";
close $scratch;
printf "scratch file ends in .tmp: %s\n", ( $scratch_name =~ /\.tmp$/ ? 'yes' : 'no' );

print "-- walking --\n";
my ( $files, $dirs, $bytes ) = ( 0, 0, 0 );
my @found;
find(
    sub {
        if ( -d $_ ) {
            $dirs++;
            return;
        }
        return unless -f $_;
        return if /\.tmp$/;    # the scratch file's name is different every run
        $files++;
        $bytes += -s $_;
        push @found, File::Spec->abs2rel( $File::Find::name, $root );
    },
    $root
);
printf "directories: %d\n", $dirs;
printf "files: %d\n",       $files;
printf "bytes: %d\n",       $bytes;
print "  $_\n" for sort @found;

print "-- pruning --\n";
my @kept;
find(
    sub {
        if ( -d $_ && $_ =~ /bin$/ ) {
            $File::Find::prune = 1;
            return;
        }
        return unless -f $_;
        return if /\.tmp$/;
        push @kept, File::Spec->abs2rel( $File::Find::name, $root );
    },
    $root
);
print "  $_\n" for sort @kept;

print "-- taking it down --\n";
my $removed = remove_tree( File::Spec->catdir( $root, 'src' ) );
printf "removed %d entries\n", $removed;
printf "src is gone: %s\n", ( -d File::Spec->catdir( $root, 'src' ) ? 'no' : 'yes' );
printf "doc survives: %s\n", ( -d File::Spec->catdir( $root, 'doc' ) ? 'yes' : 'no' );

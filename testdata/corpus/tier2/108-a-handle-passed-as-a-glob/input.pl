#!/usr/bin/perl
use strict;
use warnings;

# Before lexical filehandles a handle was a name in a symbol table, and the
# way to pass one around was to pass the glob or a reference to it. Modules
# written that way are still everywhere, so \*STDOUT and *STDERR{IO} turn up
# in code that otherwise looks modern.

sub emit {
    my ( $fh, $msg ) = @_;
    print {$fh} "$msg\n";
    return length $msg;
}

my $out = \*STDOUT;
print {$out} "written through a glob reference\n";

my $wrote = emit( \*STDOUT, 'a handle passed to a sub' );
printf "the sub wrote %d characters\n", $wrote;

my $path = 'glob-handle.txt';
open my $fh, '>', $path or die "open $path: $!\n";
emit( $fh, 'a lexical handle passed the same way' );
close $fh;

open my $back, '<', $path or die "open $path: $!\n";
my @lines = <$back>;
close $back;
chomp @lines;
print "the file holds: $lines[0]\n";
unlink $path;

# The two spellings name the same handle.
my $again = \*STDOUT;
printf "both references name one handle: %s\n",
    ( "$again" eq "$out" ? 'yes' : 'no' );

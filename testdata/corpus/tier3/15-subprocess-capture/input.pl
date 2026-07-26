#!/usr/bin/perl
# Subprocess management with the interpreter itself ($^X) as the only
# external command, so behavior is identical on any box with perl.
# Exercises backticks, open '-|' pipes, open '|-' write pipes, system(),
# and full $? decoding (exit code, signal, core-dump bit).
use strict;
use warnings;
use File::Temp qw(tempdir);
use File::Spec;

my $perl = $^X;    # absolute path; never printed directly
my $tmp  = tempdir( CLEANUP => 1 );
my $report_path = File::Spec->catfile( $tmp, 'child-report.txt' );

# ---- backticks: scalar and list context --------------------------------
my $one = `"$perl" -e 'print qq{alpha\\nbeta\\ngamma\\n}'`;
printf "backtick scalar: %d bytes, %d newlines\n",
  length $one, ( $one =~ tr/\n// );

my @lines = `"$perl" -le 'print for map { \$_ * \$_ } 1..5'`;
chomp @lines;
print "squares: @lines\n";
printf "backtick exit status: %d\n", $? >> 8;

# ---- read pipe with open '-|' (list form: no shell) --------------------
open my $rd, '-|', $perl, '-e',
  'print "$_:", $_ % 3 ? "keep" : "drop", "\n" for 1..7'
  or die "pipe open: $!\n";
my @kept;
while ( my $line = <$rd> ) {
    chomp $line;
    my ( $n, $verdict ) = split /:/, $line;
    push @kept, $n if $verdict eq 'keep';
}
close $rd or die "close rd: $!\n";
print "kept: ", join( ',', @kept ), "\n";
printf "pipe close status: %d\n", $? >> 8;

# ---- write pipe with open '|-': child counts what we send --------------
open my $wr, '|-', $perl, '-e',
  'my $n = 0; $n += length while <STDIN>; ' .
  'open my $out, ">", $ARGV[0] or die; print $out "child saw $n bytes\n";',
  $report_path
  or die "write pipe: $!\n";
print {$wr} "chunk-$_\n" for 1 .. 4;
close $wr or die "close wr: $!\n";

open my $report, '<', $report_path
  or die "no child report: $!\n";
print "report: ", scalar <$report>;
close $report;

# ---- system(): success, failure exit code, and list-form safety --------
my $rc = system( $perl, '-e', 'exit 0' );
printf "system ok: rc=%d decoded=%d\n", $rc, $rc >> 8;

$rc = system( $perl, '-e', 'exit 42' );
printf "system fail: decoded=%d signal=%d core=%s\n",
  $? >> 8, $? & 127, ( $? & 128 ) ? 'yes' : 'no';

# A command that dies (Perl die exits 255 when $! and $? are clear).
system( $perl, '-e', 'open STDERR, ">", "/dev/null"; $! = 0; die "boom\n"' );
printf "die in child: decoded=%d\n", $? >> 8;

# ---- capture stderr separately via shell redirection -------------------
my $errout =
  `"$perl" -e 'print STDOUT qq{to-out\\n}; print STDERR qq{to-err\\n}' 2>&1 1>/dev/null`;
chomp $errout;
print "stderr captured: [$errout]\n";

# ---- $? survives in a variable but is reset by the next child ----------
system( $perl, '-e', 'exit 7' );
my $saved = $? >> 8;
system( $perl, '-e', 'exit 0' );
printf "saved=%d current=%d\n", $saved, $? >> 8;

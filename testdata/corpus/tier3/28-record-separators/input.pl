#!/usr/bin/perl
# $/ decides where a read stops, and every mode of it is here: the default
# newline, undef for a slurp, '' for paragraph mode, a literal separator,
# and - the hard one - a separator read out of a configuration file, which
# is only known once the program is running.
use strict;
use warnings;

sub slurp {
    my ($path) = @_;
    open my $fh, '<', $path or die "open $path: $!\n";
    local $/;
    return <$fh>;
}

# ---- default: one line at a time ---------------------------------------
{
    open my $fh, '<', 'files/settings.conf' or die "settings: $!\n";
    my @lines = <$fh>;
    close $fh;
    printf "default: %d line(s), first is %s", scalar @lines, $lines[0];
}

# ---- slurp through a sub, so the whole file is one string --------------
my $conf = slurp('files/settings.conf');
printf "slurped: %d bytes, %d newline(s)\n", length($conf), ($conf =~ tr/\n//);

# ---- paragraph mode ----------------------------------------------------
{
    open my $fh, '<', 'files/paras.txt' or die "paras: $!\n";
    local $/ = '';
    my @paras = <$fh>;
    close $fh;
    print "paragraphs: ", scalar @paras, "\n";
    for my $i (0 .. $#paras) {
        my $p = $paras[$i];
        $p =~ s/\s+\z//;
        $p =~ s/\n/ | /g;
        print "  [$i] $p\n";
    }
}

# ---- a literal separator -----------------------------------------------
{
    open my $fh, '<', 'files/records.txt' or die "records: $!\n";
    local $/ = '::';
    my @recs = <$fh>;
    close $fh;
    chomp @recs;
    print "literal sep: ", join(' / ', @recs), "\n";
}

# ---- the separator taken from configuration, so it is only known while
#      the program runs. This is the case the converter cannot fold into
#      the call, because there is nothing to fold yet. ------------------
my ($sep) = $conf =~ /^sep \s* = \s* (\S+)/mx;
{
    open my $fh, '<', 'files/records.txt' or die "records: $!\n";
    local $/ = $sep;
    my @recs = <$fh>;
    close $fh;
    chomp @recs;
    print "configured sep ($sep): ", scalar @recs, " record(s)\n";
}

# ---- back to the default outside every block ---------------------------
{
    open my $fh, '<', 'files/paras.txt' or die "paras: $!\n";
    my @lines = <$fh>;
    close $fh;
    print "restored: ", scalar @lines, " line(s)\n";
}

#!/usr/bin/perl
# tsvjoin -- poor man's relational join for TSV extracts.
#
# We get nightly TSV dumps from the HR system and join them here because
# the reporting DB refresh only happens weekly.  Supports left and inner
# joins; a duplicate key on the right side fans out (like SQL), which
# surprised everyone the first time D30 split into two cost centers.
use strict;
use warnings;
use Getopt::Long;

my %opt = (mode => 'left');
GetOptions(\%opt, 'mode=s', 'key=s', 'empty=s')
    or die "bad options\n";
$opt{empty} = 'NULL' unless defined $opt{empty};
die "mode must be left|inner\n" unless $opt{mode} =~ /^(?:left|inner)$/;

my ($left_file, $right_file) = @ARGV;
die "usage: $0 --key col [--mode left|inner] <left.tsv> <right.tsv>\n"
    unless defined $right_file and defined $opt{key};

my ($lhdr, $lrows) = read_tsv($left_file);
my ($rhdr, $rrows) = read_tsv($right_file);

my $lkey = col_index($lhdr, $opt{key}, $left_file);
my $rkey = col_index($rhdr, $opt{key}, $right_file);

# index the right side: key -> list of rows (dup keys fan out)
my %right;
push @{ $right{ $_->[$rkey] } }, $_ for @$rrows;

# output header: left columns, then right columns minus the join key
my @rkeep = grep { $_ != $rkey } 0 .. $#$rhdr;
print join("\t", @$lhdr, @{$rhdr}[@rkeep]), "\n";

my ($matched, $fanned, $dropped) = (0, 0, 0);
for my $lrow (@$lrows) {
    my $k = $lrow->[$lkey];
    my $matches = (defined $k and $k ne '') ? $right{$k} : undef;

    if (!$matches) {
        if ($opt{mode} eq 'left') {
            print join("\t", @$lrow, map { $opt{empty} } @rkeep), "\n";
        } else {
            $dropped++;
        }
        next;
    }
    $matched++;
    $fanned++ if @$matches > 1;
    for my $rrow (@$matches) {
        print join("\t", @$lrow, @{$rrow}[@rkeep]), "\n";
    }
}

print "# $matched matched, $fanned fanned out, $dropped dropped, ",
      scalar(keys %right), " right keys\n";
exit 0;

# ----------------------------------------------------------------------
sub read_tsv {
    my ($path) = @_;
    open my $fh, '<', $path or die "open $path: $!\n";
    my $hdr_line = <$fh>;
    die "$path: empty file\n" unless defined $hdr_line;
    chomp $hdr_line;
    my @hdr = split /\t/, $hdr_line, -1;

    my @rows;
    while (my $line = <$fh>) {
        chomp $line;
        next unless length $line;
        my @f = split /\t/, $line, -1;
        # ragged rows happen when the HR export hiccups; pad, don't die
        push @f, '' while @f < @hdr;
        push @rows, \@f;
    }
    close $fh;
    return (\@hdr, \@rows);
}

sub col_index {
    my ($hdr, $name, $whence) = @_;
    for my $i (0 .. $#$hdr) {
        return $i if $hdr->[$i] eq $name;
    }
    die "column '$name' not found in $whence (have: @$hdr)\n";
}

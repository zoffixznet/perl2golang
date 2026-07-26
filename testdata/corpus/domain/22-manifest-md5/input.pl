#!/usr/bin/perl
# manifest-check -- verify a release tree against its MANIFEST.
#
# The MANIFEST format is "md5  size  path", same as the old tape backup
# scripts used, which is why size sits in the middle (nobody wants to
# regenerate thirty years of manifests).  We walk the tree ourselves
# rather than trusting File::Find's readdir order -- output must be
# byte-identical across machines for the release-signing step.
use strict;
use warnings;
use Digest::MD5;

my ($manifest_file, $root) = @ARGV;
die "usage: $0 <MANIFEST> <tree-root>\n" unless defined $root;
$root =~ s{/+$}{};
die "no such directory: $root\n" unless -d $root;

# ---- load manifest ----
my %want;    # rel path -> { md5, size }
open my $mf, '<', $manifest_file or die "open $manifest_file: $!\n";
while (<$mf>) {
    next if /^\s*(?:#|$)/;
    chomp;
    my ($md5, $size, $path) = split /\s+/, $_, 3;
    unless (defined $path and $md5 =~ /^[0-9a-f]{32}$/ and $size =~ /^\d+$/) {
        die "$manifest_file line $.: malformed entry\n";
    }
    die "$manifest_file line $.: duplicate path '$path'\n" if $want{$path};
    $want{$path} = { md5 => $md5, size => $size };
}
close $mf;

# ---- walk the tree (sorted, recursive, no File::Find) ----
my %have;    # rel path -> { md5, size }
walk('');

sub walk {
    my ($rel) = @_;
    my $abs = length $rel ? "$root/$rel" : $root;
    opendir my $dh, $abs or die "opendir $abs: $!\n";
    my @entries = sort grep { $_ ne '.' and $_ ne '..' } readdir $dh;
    closedir $dh;
    for my $e (@entries) {
        my $rpath = length $rel ? "$rel/$e" : $e;
        my $apath = "$root/$rpath";
        if (-d $apath) {
            walk($rpath);
        } elsif (-f _) {
            # note the bare _ : reuses the stat buffer from -d above
            $have{$rpath} = { md5 => md5_of($apath), size => -s $apath };
        }
    }
}

# ---- compare ----
my (@ok, @changed, @size_only, @missing, @extra);
for my $path (sort keys %want) {
    my $h = $have{$path};
    if (!$h) {
        push @missing, $path;
    } elsif ($h->{md5} eq $want{$path}{md5}) {
        push @ok, $path;
    } elsif ($h->{size} == $want{$path}{size}) {
        push @changed, $path;      # same size, different content: sneaky
    } else {
        push @size_only, sprintf '%s (manifest %d, disk %d)',
            $path, $want{$path}{size}, $h->{size};
    }
}
for my $path (sort keys %have) {
    push @extra, $path unless $want{$path};
}

# ---- report ----
printf "manifest: %d entries, disk: %d files\n",
    scalar keys %want, scalar keys %have;
printf "ok: %d\n", scalar @ok;
report('CHANGED (same size!)', \@changed);
report('SIZE MISMATCH',        \@size_only);
report('MISSING from disk',    \@missing);
report('EXTRA on disk',        \@extra);

my $bad = @changed + @size_only + @missing + @extra;
print $bad ? "FAIL: $bad problem(s)\n" : "PASS\n";
exit($bad ? 1 : 0);

# ----------------------------------------------------------------------
sub md5_of {
    my ($path) = @_;
    open my $fh, '<', $path or die "open $path: $!\n";
    binmode $fh;
    my $ctx = Digest::MD5->new;
    $ctx->addfile($fh);
    close $fh;
    return $ctx->hexdigest;
}

sub report {
    my ($label, $list) = @_;
    return unless @$list;
    print "$label:\n";
    print "  $_\n" for @$list;
}

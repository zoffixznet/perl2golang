#!/usr/bin/perl
# bump-version -- bump the release version across every file that embeds it.
#
# Each file type has its own line-matcher in %HANDLERS because every
# format spells the version differently (and README prose must only be
# touched on lines that really carry the release number -- see the
# 2019.06 incident where a date got "bumped").
#
# --dry-run prints a unified-ish diff of what would change instead of
# rewriting files; the release pipeline runs with --write after a human
# eyeballs the dry run.  It also cross-checks that all files agree on
# the current version BEFORE bumping; a drifted file fails the run.
use strict;
use warnings;
use Getopt::Long;

my %opt;
GetOptions(\%opt, 'part=s', 'dry-run|n', 'write') or die "bad options\n";
my $part = $opt{part} // 'patch';
die "part must be major|minor|patch\n" unless $part =~ /^(?:major|minor|patch)$/;
die "refusing to run with both --dry-run and --write\n" if $opt{'dry-run'} and $opt{write};

my $dir = shift @ARGV or die "usage: $0 --part X (--dry-run|--write) <srcdir>\n";

# file basename -> matcher returning (current_version, line_rewriter)
my %HANDLERS = (
    'Makefile' => sub {
        my ($line) = @_;
        return unless $line =~ /^(VERSION\s*:?=\s*)(\d+\.\d+\.\d+)\s*$/;
        return ($2, "$1%s");
    },
    'Widgetd.pm' => sub {
        my ($line) = @_;
        return unless $line =~ /^(our \$VERSION = ')(\d+\.\d+\.\d+)(';)$/;
        return ($2, "$1%s$3");
    },
    'README.md' => sub {
        my ($line) = @_;
        # only prose lines that name the release explicitly
        return unless $line =~ /^(Current release: \*\*)(\d+\.\d+\.\d+)(\*\*)$/;
        return ($2, "$1%s$3");
    },
    'widgetd.spec' => sub {
        my ($line) = @_;
        return unless $line =~ /^(Version:\s+)(\d+\.\d+\.\d+)\s*$/;
        return ($2, "$1%s");
    },
);

# ---- pass 1: find current version in every handled file ----
my %found;      # file -> { version, lineno, template, lines => [...] }
for my $base (sort keys %HANDLERS) {
    my $path = "$dir/$base";
    open my $fh, '<', $path or die "open $path: $!\n";
    my @lines = <$fh>;
    close $fh;
    chomp @lines;

    my $rec;
    for my $i (0 .. $#lines) {
        my ($ver, $tmpl) = $HANDLERS{$base}->($lines[$i]);
        next unless defined $ver;
        die "$base: version found on two lines (" . ($rec->{lineno}) . " and "
            . ($i + 1) . ")\n" if $rec;
        $rec = { version => $ver, lineno => $i + 1, template => $tmpl };
    }
    die "$base: no version line found\n" unless $rec;
    $rec->{lines} = \@lines;
    $found{$base} = $rec;
}

# ---- pass 2: consensus check ----
my %by_version;
push @{ $by_version{ $found{$_}{version} } }, $_ for sort keys %found;
my @versions = sort keys %by_version;

if (@versions > 1) {
    print "version drift detected:\n";
    for my $v (@versions) {
        printf "  %-8s %s\n", $v, join(', ', @{ $by_version{$v} });
    }
    # Majority wins for the report; the exit code still fails the build.
    my ($majority) = sort {
        @{ $by_version{$b} } <=> @{ $by_version{$a} } or $a cmp $b
    } @versions;
    my $next = bump($majority, $part);
    print "majority version $majority would bump to $next; fix drift first\n";
    exit 2;
}

my $current = $versions[0];
my $next    = bump($current, $part);
print "bumping $current -> $next ($part) in $dir\n\n";

# ---- pass 3: emit diffs (and optionally write) ----
for my $base (sort keys %found) {
    my $rec  = $found{$base};
    my $old  = $rec->{lines}[ $rec->{lineno} - 1 ];
    my $new  = sprintf $rec->{template}, $next;
    print "--- $base line $rec->{lineno}\n";
    print "-$old\n";
    print "+$new\n";
    if ($opt{write}) {
        $rec->{lines}[ $rec->{lineno} - 1 ] = $new;
        open my $out, '>', "$dir/$base" or die "write $dir/$base: $!\n";
        print {$out} map { "$_\n" } @{ $rec->{lines} };
        close $out;
    }
}
print "\n", $opt{write} ? 'wrote' : 'dry-run, would write',
    ' ', scalar keys %found, " file(s)\n";
exit 0;

# ----------------------------------------------------------------------
sub bump {
    my ($ver, $what) = @_;
    my ($maj, $min, $pat) = split /\./, $ver;
    if    ($what eq 'major') { $maj++; $min = 0; $pat = 0 }
    elsif ($what eq 'minor') { $min++; $pat = 0 }
    else                     { $pat++ }
    return join '.', $maj, $min, $pat;
}

#!/usr/bin/perl
# tmpl-expand -- expand {{var}} templates for deploy-time config files.
#
# Supports {{var}}, {{var|default}}, and nested references: variable
# values may themselves contain {{...}}, and a default may contain a
# {{...}} (which works because inner tokens get rewritten before the
# outer token can match -- do not "optimise" the while loop into /g).
# Circular references die with the full path; the caller catches this
# per token so one bad variable cannot kill the whole file (INC-2019-0142).
use strict;
use warnings;

my ($vars_file, $tmpl_file) = @ARGV;
die "usage: $0 <vars.conf> <template>\n" unless defined $tmpl_file;

# ---- load variable definitions ----
my %vars;
open my $vf, '<', $vars_file or die "open $vars_file: $!\n";
while (<$vf>) {
    next if /^\s*(?:#|$)/;
    chomp;
    my ($name, $value) = /^(\w+)\s*=\s*(.*?)\s*$/
        or die "$vars_file line $.: cannot parse\n";
    if (exists $vars{$name}) {
        warn_dup($name);        # earlier definitions win, like the old shell version
    } else {
        $vars{$name} = $value;
    }
}
close $vf;

# ---- expand the template line by line ----
my $TOKEN = qr/\{\{\s*(\w+)\s*(?:\|([^{}]*))?\s*\}\}/;
my @errors;    # [line, message]
my $lineno = 0;

open my $tf, '<', $tmpl_file or die "open $tmpl_file: $!\n";
while (my $line = <$tf>) {
    $lineno++;
    chomp $line;

    my $passes = 0;
    while ($line =~ $TOKEN) {
        my ($name, $default) = ($1, $2);
        die "line $lineno: expansion did not converge\n" if ++$passes > 100;

        my $rep = eval { resolve($name, {}) };
        if (!defined $rep) {
            my $err = $@;
            chomp $err;
            if (defined $default) {
                $rep = $default;        # default swallows the failure
            } else {
                push @errors, [$lineno, $err || "undefined variable '$name'"];
                $rep = "[[UNRESOLVED:$name]]";
            }
        }
        $line =~ s/$TOKEN/$rep/;        # replace ONE token, then rescan
    }
    print "$line\n";
}
close $tf;

print "# --- expansion report ---\n";
if (@errors) {
    printf "# line %-3d %s\n", @$_ for @errors;
    printf "# %d error(s)\n", scalar @errors;
} else {
    print "# clean\n";
}
exit(@errors ? 1 : 0);

# ----------------------------------------------------------------------
# Resolve one variable to its fully-expanded value.  $seen maps names on
# the current resolution path, for cycle detection with a useful message.
sub resolve {
    my ($name, $seen) = @_;
    if ($seen->{$name}) {
        my @path = sort { $seen->{$a} <=> $seen->{$b} } keys %$seen;
        die 'circular reference: ' . join(' -> ', @path, $name) . "\n";
    }
    die "undefined variable '$name'\n" unless exists $vars{$name};

    $seen->{$name} = 1 + keys %$seen;   # record position on the path
    my $text = $vars{$name};
    my $guard = 0;
    while ($text =~ $TOKEN) {
        my ($inner, $default) = ($1, $2);
        die "expansion of '$name' did not converge\n" if ++$guard > 100;
        my $rep = eval { resolve($inner, $seen) };
        if (!defined $rep) {
            die $@ unless defined $default;   # propagate with path intact
            $rep = $default;
        }
        $text =~ s/$TOKEN/$rep/;
    }
    delete $seen->{$name};              # backtrack: siblings may reuse it
    return $text;
}

sub warn_dup {
    my ($name) = @_;
    # earlier definitions win; this matches the old shell implementation
    print "# note: duplicate definition of '$name' ignored\n";
}

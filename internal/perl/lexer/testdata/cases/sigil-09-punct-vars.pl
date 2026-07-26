#!/usr/bin/perl
# CASE sigil-09: punctuation variables. `$` followed by almost any single
# punctuation character is a variable, which means `$/`, `$\`, `$,`, `$;`, `$|`,
# `$"`, `$'`, `$#` and friends all collide with operators, comments and quote
# delimiters. The lexer must special-case `$` + punctuation BEFORE its normal
# operator scan.
use strict; use warnings;

local $_ = "underscore value";
print "sigil-09 underscore: $_\n";

sub args { return "args=[" . join("|", @_) . "] count=" . scalar(@_) }
print "sigil-09 at-underscore: ", args(1,2,3), "\n";

print "sigil-09 zero: ", ($0 =~ /sigil-09/ ? "script name ok" : "unexpected $0"), "\n";

"hello world" =~ /(\w+)\s(\w+)/;
print "sigil-09 captures: $1 $2 amp=[$&] pre=[$`] post=[$']\n";

{
    local $, = "-";
    local $\ = "!\n";
    print "sigil-09", "ofs", "ors";
}

{
    local $" = "+";
    my @a = (1,2,3);
    print "sigil-09 list-sep: @a\n";
}

{
    local $/ = undef;
    print "sigil-09 irs-undef: ", (defined $/ ? "defined" : "slurp mode"), "\n";
}

my %multi;
{
    local $; = ":";
    $multi{1,2} = "multidim";
    my ($k) = keys %multi;
    print "sigil-09 subsep: ", join("", map { ord } split //, $k), "\n";
}

print "sigil-09 autoflush-before: ", (defined $| ? $| : "undef"), "\n";

eval { die "boom\n" };
print "sigil-09 eval-error: $@";
open(my $fh, '<', "/no/such/file/here") or print "sigil-09 errno: ", ($! ? "set" : "unset"), "\n";
system($^X, "-e", "exit 3");
print "sigil-09 child-status: ", ($? >> 8), "\n";
print "sigil-09 perl-executable-var: ", ($^X ? "set" : "unset"), "\n";

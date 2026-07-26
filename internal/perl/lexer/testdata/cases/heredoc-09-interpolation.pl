#!/usr/bin/perl
# CASE heredoc-09: interpolation inside a double-quoted heredoc follows the same
# grammar as "" -- scalars, elements, arrow chains, @{[ ]}, and escapes.
use strict; use warnings;
binmode(STDOUT, ':encoding(UTF-8)');

my $name = "Ada";
my @list = (1,2,3);
my %h    = (k => "vee");
my $obj  = { f => "field", a => [10,20] };

my $t = <<"EOT";
scalar: $name
braced: ${name}
elem:   $h{k}
array:  @list
index:  $list[1] and $list[-1]
arrow:  $obj->{f} and $obj->{a}[1]
expr:   @{[ 6*7 ]}
ref:    ${\ join("-", @list) }
esc:    \t<-tab \x41 \x{263A} \\ \$name
EOT
print $t;
print "heredoc-09 dollar-literal-present: ", ($t =~ /\$name/ ? "yes" : "no"), "\n";

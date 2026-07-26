use strict;
use warnings;

my $name  = "Ada";
my $count = 3;
my @items = qw(pen book lamp);

print <<"EOT";
Hello, $name!
You have $count messages and @items in your bag.
Math is not interpolated: 1+1
EOT

print <<'EOT';
Literal heredoc: $name is not interpolated.
A backslash-n stays as-is: \n
And so does \t and \$name.
EOT

my $indented = <<~EOT;
    This indented heredoc has its common leading
    whitespace stripped, based on the terminator.
      This line keeps two extra spaces.
    EOT
print $indented;

print <<"A", <<"B";
first chunk for $name
A
second chunk, $count of them
B

my $body = <<~'RAW';
    keep $this literal
    and this \n too
    RAW
print $body;

my $len = length(<<'X');
abcdef
X
print "that heredoc was $len bytes\n";


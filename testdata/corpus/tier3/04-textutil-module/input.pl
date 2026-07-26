#!/usr/bin/perl
# Exercises TextUtil (Exporter, tags, constants) and TextUtil::Stats.
use strict;
use warnings;
use lib '.';
use TextUtil qw(:clean truncate_str commify DEFAULT_WIDTH);
use TextUtil::Stats qw(word_freq summarize);

my $messy = "   the  quick\tbrown   fox jumps over the lazy dog  ";
my $clean = squeeze( trim($messy) );
print "clean: [$clean]\n";

# Qualified call for a function we did NOT import.
print "title: ", TextUtil::title_case($clean), "\n";

print "width const: ", DEFAULT_WIDTH, "\n";
print "trunc: ",  truncate_str( $clean, 20 ), "\n";
print "trunc2: ", truncate_str('short'), "\n";
print "commify: ", commify(90210), " / ", commify(1234567), "\n";
print "module: ", TextUtil::identify(), "\n";

my $speech = <<'END';
We shall fight on the beaches, we shall fight on the landing grounds,
we shall fight in the fields and in the streets, we shall fight in
the hills; we shall never surrender.
END

my $freq = word_freq($speech);
my $sum  = summarize($freq);
print  "--- word stats ($sum->{package}) ---\n";
printf "distinct=%d total=%d longest=%d shortest=%d\n",
  @{$sum}{qw(distinct total longest shortest)};    # hash slice of a hashref
printf "top: %s\n", join ', ',
  map { "$_ ($freq->{$_})" } @{ $sum->{top} };

print "--- call accounting ---\n";
print "$_\n" for TextUtil::call_stats();

# Direct read of the module's package variable from outside.
printf "trim was called %d time(s)\n", $TextUtil::CALLS{trim};

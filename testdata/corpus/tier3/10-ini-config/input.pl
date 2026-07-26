#!/usr/bin/perl
# INI-style config parser: sections, DEFAULT fallback, 'extends' section
# inheritance, ${var} interpolation with cycle detection, typed getters.
use strict;
use warnings;

my $file = shift @ARGV // 'files/app.ini';

sub parse_ini {
    my ($path) = @_;
    my %cfg;
    my $section = 'DEFAULT';
    open my $fh, '<', $path or die "open $path: $!\n";
    while ( my $line = <$fh> ) {
        chomp $line;
        $line =~ s/^\s+|\s+$//g;
        next if $line eq '' || $line =~ /^[;#]/;
        if ( $line =~ /^\[([\w.]+)\]$/ ) {
            $section = $1;
            $cfg{$section} //= {};
            next;
        }
        my ( $k, $v ) = $line =~ /^(\w+)\s*=\s*(.*)$/
          or die "$path:$.: cannot parse line: $line\n";
        die "$path:$.: duplicate key '$k' in [$section]\n"
          if exists $cfg{$section}{$k};
        $cfg{$section}{$k} = $v;
    }
    close $fh;
    return \%cfg;
}

# Resolve one section: DEFAULT < extends-chain < own keys, then interpolate.
sub resolve_section {
    my ( $cfg, $name, $seen ) = @_;
    $seen //= {};
    die "extends cycle at [$name]\n" if $seen->{$name}++;
    my $raw = $cfg->{$name} or die "no such section [$name]\n";

    my %merged = %{ $cfg->{DEFAULT} // {} };
    if ( my $parent = $raw->{extends} ) {
        my $base = resolve_section( $cfg, $parent, $seen );
        %merged = ( %merged, %$base );
    }
    %merged = ( %merged, %$raw );
    delete $merged{extends};

    # ${var} interpolation: repeat until fixpoint, bail on runaway.
    for my $key ( sort keys %merged ) {
        my $rounds = 0;
        while ( $merged{$key} =~ /\$\{(\w+)\}/ ) {
            my $ref = $1;
            die "unknown variable \${$ref} in $name.$key\n"
              unless exists $merged{$ref};
            die "interpolation loop in $name.$key\n" if ++$rounds > 10;
            $merged{$key} =~ s/\$\{\Q$ref\E\}/$merged{$ref}/g;
        }
    }
    return \%merged;
}

sub get_int {
    my ( $sec, $key ) = @_;
    my $v = $sec->{$key} // die "missing key $key\n";
    $v =~ /^-?\d+$/ or die "key $key is not an integer: '$v'\n";
    return $v + 0;
}

my $cfg = parse_ini($file);

for my $name ( grep { $_ ne 'DEFAULT' } sort keys %$cfg ) {
    my $sec = resolve_section( $cfg, $name );
    print "[$name]\n";
    printf "  %-10s = %s\n", $_, $sec->{$_} for sort keys %$sec;
}

# Typed access and arithmetic on config values.
my $web    = resolve_section( $cfg, 'web' );
my $worker = resolve_section( $cfg, 'worker' );
printf "web capacity: %d slots\n",
  get_int( $web, 'workers' ) * get_int( $web, 'timeout' );
printf "worker inherits port: %s\n", $worker->{port};
printf "dev dsn: %s\n", resolve_section( $cfg, 'database.dev' )->{dsn};

# Error handling: every failure mode is a catchable die.
for my $probe (
    [ 'missing section', sub { resolve_section( $cfg, 'nope' ) } ],
    [ 'bad int', sub { get_int( $web, 'static_dir' ) } ],
    [
        'unknown var',
        sub {
            my %c = %$cfg;
            $c{broken} = { path => '${nowhere}/x' };
            resolve_section( \%c, 'broken' );
        }
    ],
  )
{
    my ( $label, $code ) = @$probe;
    if   ( eval { $code->(); 1 } ) { print "$label: no error?!\n" }
    else                           { print "$label: caught: $@" }
}

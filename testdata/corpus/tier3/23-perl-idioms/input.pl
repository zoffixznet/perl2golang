#!/usr/bin/perl
# A grab bag of the awkward idioms real Perl code actually contains:
# local on an our-global, @_ aliasing, wantarray, goto &sub, do-blocks as
# expressions, unless/until, %*d and %vd formats, and fancy slices.
use strict;
use warnings;

our $TRACE_DEPTH = 0;    # package global, dynamically scoped below
my @trace_log;

sub trace {
    push @trace_log, ( '  ' x $TRACE_DEPTH ) . $_[0];
    return;
}

# ---- local on a global: depth restored automatically on scope exit -----
sub descend {
    my ( $label, $levels ) = @_;
    trace("enter $label");
    if ( $levels > 0 ) {
        local $TRACE_DEPTH = $TRACE_DEPTH + 1;
        descend( "$label.$levels", $levels - 1 );
    }
    trace("leave $label");    # depth already restored here
    return;
}
descend( 'root', 2 );
print "$_\n" for @trace_log;

# ---- @_ aliasing: callee mutates the caller's variables ----------------
sub embiggen { $_ *= 10 for @_ }        # aliases!
sub swap     { @_[ 0, 1 ] = @_[ 1, 0 ] }

my ( $low, $high ) = ( 3, 7 );
my @sizes = ( 1, 2, 3 );
embiggen( $low, @sizes );
swap( $low, $high );
print "after aliasing: low=$low high=$high sizes=@sizes\n";

# ---- wantarray: context-sensitive return -------------------------------
sub span {
    my @range = ( $_[0] .. $_[1] );
    return wantarray ? @range : scalar @range;
}
my @all   = span( 4, 8 );
my $count = span( 4, 8 );
my ($first) = span( 4, 8 );    # list context via list assignment
print "span list=@all scalar=$count first=$first\n";

# ---- goto &sub: tail-dispatch keeping @_ -------------------------------
sub legacy_render { goto &render_v2 }    # old name forwards, no new frame

sub render_v2 {
    my (%opt) = @_;
    return sprintf '<%s size=%d>', $opt{shape} // 'box', $opt{size} // 1;
}
print "goto: ", legacy_render( shape => 'disc', size => 3 ), "\n";

# ---- do-block as an expression, unless/until ---------------------------
my $threshold = 45;
my $bucket = do {
    if    ( $threshold < 10 )  { 'tiny' }
    elsif ( $threshold < 100 ) { 'medium' }
    else                       { 'huge' }
};
print "bucket: $bucket\n" unless $bucket eq 'tiny';

my @countdown;
my $n = 5;
until ( $n == 0 ) { push @countdown, $n--; }
print "countdown: @countdown\n";

# ---- memoized fibonacci with a closure cache ---------------------------
my $fib = do {
    my %cache = ( 0 => 0, 1 => 1 );
    my $self;
    $self = sub {
        my ($k) = @_;
        $cache{$k} //= $self->( $k - 1 ) + $self->( $k - 2 );
    };
    $self;
};
print "fib: ", join( ',', map { $fib->($_) } 0 .. 10, 30 ), "\n";

# ---- sprintf %*d (runtime width) and %vd (version vector) --------------
my @cols  = ( 42, 7, 31337 );
my $width = 7;
print 'cells:';
printf " |%*d|", $width, $_ for @cols;
print "\n";
printf "narrow:%*d wide:%*d\n", 3, 42, 9, 42;

my $version = v2.14.7;
printf "engine version %vd (raw length %d)\n", $version, length $version;
printf "compare: %s\n", ( $version ge v2.14.0 ? 'new enough' : 'too old' );

# ---- slices: hash, array, negative, kv ---------------------------------
my %config = ( host => 'db1', port => 5432, user => 'app', ssl => 'on' );
my ( $h, $p ) = @config{qw(host port)};             # hash slice
my %subset = %config{ 'user', 'ssl' };              # kv-slice (5.20+)
my @week   = qw(mon tue wed thu fri sat sun);
my @mid    = @week[ 2 .. 4 ];                       # array slice
my @ends   = @week[ 0, -1 ];                        # mixed indices
my @picked = @week[ grep { $_ % 2 == 0 } 0 .. $#week ];    # computed slice

print "conn: $h:$p\n";
print "subset: ", join( ',', map { "$_=$subset{$_}" } sort keys %subset ),
  "\n";
print "mid=@mid ends=@ends picked=@picked\n";

# slice assignment writes several keys at once
@config{qw(host port)} = ( 'db2', 6432 );
print "moved to: $config{host}:$config{port}\n";

# ---- chained string ops that trip converters ---------------------------
my $report = join '',
  map { $_ % 3 ? '.' : '#' }    # ternary inside map
  grep { $_ != 13 }             # superstition filter
  1 .. 20;
print "pattern: $report\n";
print "pattern length ok\n" if length($report) == 19;

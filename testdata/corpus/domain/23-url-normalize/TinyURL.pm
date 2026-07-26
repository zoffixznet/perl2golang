package TinyURL;
# Hand-rolled URL parser/normaliser.  Yes, URI.pm exists; no, it was not
# on the bastion hosts, and by the time it was, this file had grown its
# own personality.  Implements the RFC 3986 normalisations we actually
# need for cache-key deduplication: case, default ports, dot-segments,
# percent-encoding, and query-param sorting (documented site policy:
# our origin ignores param order).
use strict;
use warnings;

my %DEFAULT_PORT = (http => 80, https => 443, ftp => 21);

# ---- constructor: parses or dies ----
sub parse {
    my ($class, $raw) = @_;
    my $self = bless { raw => $raw }, $class;

    my ($scheme, $rest) = $raw =~ m{^([A-Za-z][A-Za-z0-9+.-]*)://(.*)$}
        or die "no scheme in '$raw'\n";
    $self->{scheme} = lc $scheme;
    die "unsupported scheme '$self->{scheme}'\n"
        unless exists $DEFAULT_PORT{ $self->{scheme} };

    # split off fragment, then query, in that order (RFC order matters:
    # a '?' after '#' belongs to the fragment)
    ($rest, $self->{fragment}) = split /#/, $rest, 2;
    ($rest, my $query)         = split /\?/, $rest, 2;

    my ($authority, $path) = $rest =~ m{^([^/]*)(/.*)?$};
    die "empty host in '$raw'\n" unless length $authority;

    if ($authority =~ /^(?:([^@]*)@)?([^:]+)(?::(\d+))?$/) {
        $self->{userinfo} = $1;
        $self->{host}     = lc $2;
        $self->{port}     = $3;
    } else {
        die "cannot parse authority '$authority'\n";
    }
    die "userinfo not allowed by site policy in '$raw'\n"
        if defined $self->{userinfo};

    $self->{port} //= $DEFAULT_PORT{ $self->{scheme} };
    $self->{path}  = defined $path ? $path : '/';
    $self->{query} = $query;
    return $self;
}

# ---- accessors (the hand-written kind) ----
sub scheme { $_[0]{scheme} }
sub host   { $_[0]{host} }
sub port   { $_[0]{port} }
sub path   { $_[0]{path} }

# ---- normalisation steps, each its own method for testability ----
sub normalize {
    my ($self) = @_;
    $self->_normalize_percent;
    $self->_normalize_dots;
    $self->_normalize_query;
    return $self;
}

sub _normalize_percent {
    my ($self) = @_;
    for my $part (\$self->{path}, \$self->{query}) {
        next unless defined $$part;
        # decode unreserved characters only; uppercase surviving triplets
        $$part =~ s/%(3[0-9]|[46][1-9A-Fa-f]|[57][0-9Aa]|2[DdEe]|5[Ff]|7[Ee])/chr hex $1/ge;
        $$part =~ s/%([0-9a-f]{2})/'%' . uc $1/ge;
    }
}

sub _normalize_dots {
    my ($self) = @_;
    my @out;
    # note: -1 keeps the trailing empty segment ("/a/" -> "a","")
    my @segs = split m{/}, $self->{path}, -1;
    shift @segs;    # leading empty segment from the root '/'
    for my $seg (@segs) {
        if    ($seg eq '.')  { next }
        elsif ($seg eq '..') { pop @out if @out }
        else                 { push @out, $seg }
    }
    # a trailing '.' or '..' implies a directory
    push @out, '' if @segs and $segs[-1] =~ /^\.\.?$/;
    $self->{path} = '/' . join '/', @out;
}

sub _normalize_query {
    my ($self) = @_;
    return unless defined $self->{query} and length $self->{query};
    my @pairs = grep { length } split /&/, $self->{query};
    # sort by key then value; '=' -less params sort as key with empty value
    my @split = map { [split /=/, $_, 2] } @pairs;
    @split = sort {
        $a->[0] cmp $b->[0] or ($a->[1] // '') cmp ($b->[1] // '')
    } @split;
    $self->{query} = join '&',
        map { defined $_->[1] ? "$_->[0]=$_->[1]" : $_->[0] } @split;
}

# ---- canonical string form ----
sub canon {
    my ($self) = @_;
    my $s = $self->{scheme} . '://' . $self->{host};
    $s .= ':' . $self->{port}
        unless $self->{port} == $DEFAULT_PORT{ $self->{scheme} };
    $s .= $self->{path};
    $s .= '?' . $self->{query}
        if defined $self->{query} and length $self->{query};
    # fragments are dropped on purpose: cache keys ignore them
    return $s;
}

1;

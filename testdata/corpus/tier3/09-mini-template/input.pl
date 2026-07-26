#!/usr/bin/perl
# Tiny mustache-ish template engine: {{var.path|filter}}, {{#each}}, {{#if}},
# recursive rendering with lexical-scope-style context fallback.
use strict;
use warnings;

# ---- template compiler: token list -> nested node tree -----------------
sub compile_template {
    my ($src) = @_;
    my @tokens = grep { length } split /(\{\{.*?\}\})/s, $src;
    my @stack  = ( { type => 'root', kids => [] } );
    for my $tok (@tokens) {
        if ( $tok =~ /^\{\{#(each|if)\s+(\S+)\}\}$/ ) {
            my $node = { type => $1, expr => $2, kids => [] };
            push @{ $stack[-1]{kids} }, $node;
            push @stack, $node;
        }
        elsif ( $tok =~ /^\{\{\/(each|if)\}\}$/ ) {
            die "unbalanced {{/$1}}\n"
              unless @stack > 1 && $stack[-1]{type} eq $1;
            pop @stack;
        }
        elsif ( $tok =~ /^\{\{([\w.]+)(?:\|(\w+))?\}\}$/ ) {
            push @{ $stack[-1]{kids} },
              { type => 'var', path => $1, filter => $2 };
        }
        elsif ( $tok =~ /^\{\{/ ) { die "bad tag: $tok\n" }
        else { push @{ $stack[-1]{kids} }, { type => 'text', text => $tok } }
    }
    die "unclosed $stack[-1]{type} block\n" if @stack != 1;
    return $stack[0];
}

# ---- path lookup with context-chain fallback ---------------------------
sub lookup {
    my ( $frames, $path ) = @_;
    for my $frame ( reverse @$frames ) {    # innermost context wins
        my $cur = $frame;
        my $ok  = 1;
        for my $part ( split /\./, $path ) {
            if ( ref $cur eq 'HASH' && exists $cur->{$part} ) {
                $cur = $cur->{$part};
            }
            else { $ok = 0; last }
        }
        return $cur if $ok;
    }
    return undef;
}

my %filters = (
    uc    => sub { uc $_[0] },
    money => sub { sprintf '%.2f', $_[0] },
    pad20 => sub { sprintf '%-20s', $_[0] },
);

sub render {
    my ( $node, $frames ) = @_;
    my $out = '';
    for my $kid ( @{ $node->{kids} } ) {
        my $t = $kid->{type};
        if    ( $t eq 'text' ) { $out .= $kid->{text} }
        elsif ( $t eq 'var' ) {
            my $val = lookup( $frames, $kid->{path} );
            die "undefined template variable '$kid->{path}'\n"
              unless defined $val;
            if ( my $f = $kid->{filter} ) {
                die "unknown filter '$f'\n" unless $filters{$f};
                $val = $filters{$f}->($val);
            }
            $out .= $val;
        }
        elsif ( $t eq 'if' ) {
            my $val = lookup( $frames, $kid->{expr} );
            $out .= render( $kid, $frames ) if $val;    # perl truthiness
        }
        elsif ( $t eq 'each' ) {
            my $list = lookup( $frames, $kid->{expr} );
            die "'$kid->{expr}' is not a list\n" unless ref $list eq 'ARRAY';
            for my $item (@$list) {
                $out .= render( $kid, [ @$frames, $item ] );
            }
        }
    }
    return $out;
}

# ---- data --------------------------------------------------------------
my @items = (
    { name => 'Sourdough loaf', qty => 2, price => 6.50, note => 'sliced' },
    { name => 'Butter',         qty => 1, price => 4.25, note => '' },
    { name => 'Raspberry jam',  qty => 3, price => 5.10, note => 'gift wrap' },
);
$_->{total} = $_->{qty} * $_->{price} for @items;
my $subtotal = 0;
$subtotal += $_->{total} for @items;
my $tax = $subtotal * 0.08;

my %data = (
    store    => 'Hearth & Crumb',
    invoice  => { number => 'INV-2024-0117', date => '2024-06-01' },
    customer => { name => 'Ada Lovelace', email => 'ada@example.com', vip => 1 },
    items    => \@items,
    subtotal   => $subtotal,
    tax_rate   => 8,
    tax        => $tax,
    grand_total => $subtotal + $tax,
    overdue    => 0,
);

my $src = do {
    open my $fh, '<', 'files/invoice.tmpl' or die "template: $!\n";
    local $/;
    <$fh>;
};

my $tree = compile_template($src);
print render( $tree, [ \%data ] );

# Error paths must be catchable.
for my $bad ( '{{#each items}}no close', '{{oops}}', '{{store|nope}}' ) {
    if ( eval { render( compile_template($bad), [ \%data ] ); 1 } ) {
        print "no error?!\n";
    }
    else { ( my $e = $@ ) =~ s/\n$//; print "error: $e\n" }
}

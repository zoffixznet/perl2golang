#!/usr/bin/perl
use strict;
use warnings;

# A nested structure built by walking a cursor down it, which is how a tree
# gets grown from flat paths without a single "create this node" statement.
# `\%{ $node->{$part} }` is a reference to the hash at that key, and the key
# comes into existence because the reference was taken.

my %tree;
my @paths = (
    'etc/hosts',
    'etc/ssh/sshd_config',
    'var/log/messages',
    'var/log/nginx/access.log',
);

for my $path (@paths) {
    my @parts = split m{/}, $path;
    my $leaf  = pop @parts;
    my $node  = \%tree;
    for my $part (@parts) {
        $node = \%{ $node->{$part} };
    }
    $node->{$leaf} = length $path;
}

sub render {
    my ( $node, $depth ) = @_;
    my $out = '';
    for my $key ( sort keys %$node ) {
        $out .= ( '  ' x $depth ) . $key;
        if ( ref $node->{$key} eq 'HASH' ) {
            $out .= "/\n" . render( $node->{$key}, $depth + 1 );
        }
        else {
            $out .= " ($node->{$key})\n";
        }
    }
    return $out;
}

print render( \%tree, 0 );
printf "top level: %s\n", join( ',', sort keys %tree );

#!/usr/bin/perl
# CASE stmt-02: `sub` introduces a named declaration, an anonymous sub, a forward
# declaration, or (with a signature/prototype/attributes) something with extra
# tokens between the name and the body.
use strict; use warnings;
use feature 'signatures';
no warnings 'experimental::signatures';

sub named { return "named" }
my $anon = sub { return "anon" };
sub forward_decl;
sub forward_decl { return "forward" }

print "stmt-02 named: ", named(), "\n";
print "stmt-02 anon: ", $anon->(), "\n";
print "stmt-02 forward: ", forward_decl(), "\n";

# `sub f ($) {...}` means DIFFERENT things depending on whether the signatures
# feature is enabled: a prototype without it, a signature with it. Same tokens.
my $noSig = `$^X -e 'sub f (\$) { 1 } print prototype(\\&f) // q{undef};' 2>&1`;
my $wiSig = `$^X -e 'use feature "signatures"; sub f (\$) { 1 } print prototype(\\&f) // q{undef};' 2>&1`;
$_ =~ s/\s+\z// for $noSig, $wiSig;
print "stmt-02 dollar-parens: without-signatures=prototype[$noSig] with-signatures=prototype[$wiSig]\n";

# Under the signatures feature a prototype must be spelled as an attribute.
sub proto_one :prototype($) ($x) { return "proto($x)" }
print "stmt-02 prototype-attribute: ", proto_one("x"), " prototype=", prototype(\&proto_one), "\n";

# Signature.
sub sig ($a, $b = 5, @rest) { return "sig a=$a b=$b rest=" . scalar(@rest) }
print "stmt-02 signature: ", sig(1), " / ", sig(1,2,3,4), "\n";

# Attributes.
sub attr :lvalue { $main::store }
attr() = "via lvalue";
print "stmt-02 attribute: $main::store\n";

# Anonymous sub with a signature, immediately invoked.
print "stmt-02 anon-sig: ", (sub ($x) { "got $x" })->("y"), "\n";

# A sub named like a keyword.
sub length_of { return length($_[0]) }
print "stmt-02 keywordish-name: ", length_of("abcd"), "\n";

# `sub` inside a hash literal, as a value.
my %d = ( cb => sub { "callback" } );
print "stmt-02 in-hash: ", $d{cb}->(), "\n";

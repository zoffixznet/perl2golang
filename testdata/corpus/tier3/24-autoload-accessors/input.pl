#!/usr/bin/perl
# AUTOLOAD-driven accessor generation: methods that don't exist until the
# first call, installed into the symbol table on demand, with DESTROY
# handled correctly and unknown methods dying with a useful message.
use strict;
use warnings;

package Record;

our $AUTOLOAD;
my %ALLOWED = map { $_ => 1 } qw(title author year rating);
my @generated;    # audit: which accessors got materialized, in order

sub new {
    my ( $class, %args ) = @_;
    my $self = bless { created => 0 }, $class;
    $self->{$_} = $args{$_} for grep { $ALLOWED{$_} } sort keys %args;
    $self->{created}++;
    return $self;
}

sub AUTOLOAD {
    my $name = $AUTOLOAD;
    $name =~ s/.*:://;          # strip package qualifier
    return if $name eq 'DESTROY';    # never autoload the destructor

    die "no such accessor '$name' on " . __PACKAGE__ . "\n"
      unless $ALLOWED{$name};

    # materialize a real method so AUTOLOAD only fires once per name
    push @generated, $name;
    {
        no strict 'refs';
        *{ __PACKAGE__ . "::$name" } = sub {
            my $self = shift;
            $self->{$name} = shift if @_;
            return $self->{$name};
        };
    }
    # re-dispatch the original call to the freshly installed method
    my $self = shift;
    return $self->$name(@_);
}

sub generated_list { join ',', @generated }

sub summary {
    my ($self) = @_;
    return sprintf '%s by %s (%d) %s', $self->title, $self->author,
      $self->year, '*' x ( $self->rating // 0 );
}

package Record::Signed;
our @ISA = ('Record');

# a REAL method with the same name an accessor could have had:
# real methods shadow AUTOLOAD, so this never triggers generation
sub rating { my $self = shift; ( $self->SUPER::rating() // 0 ) + 1 }

sub sign { my ($self) = @_; return '[signed] ' . $self->summary }

package main;

my $book = Record->new(
    title  => 'Higher-Order Perl',
    author => 'MJD',
    year   => 2005,
);

# Before any accessor call, none are materialized.
printf "generated before: [%s]\n", Record::generated_list();
printf "can(title) before: %s\n", Record->can('title') ? 'yes' : 'no';

print "title:  ", $book->title, "\n";
print "author: ", $book->author, "\n";

# Now 'title' and 'author' exist as real methods.
printf "generated after: [%s]\n", Record::generated_list();
printf "can(title) after: %s\n", Record->can('title') ? 'yes' : 'no';
printf "can(rating) still: %s\n", Record->can('rating') ? 'yes' : 'no';

# setter through the generated accessor
$book->rating(4);
$book->year(2006);
print $book->summary, "\n";
printf "generated final: [%s]\n", Record::generated_list();

# unknown accessor dies through AUTOLOAD
if ( eval { $book->isbn; 1 } ) { print "isbn worked?!\n" }
else                           { print "caught: $@" }

# subclass: real method shadows would-be accessor, SUPER:: reaches the
# generated one in the parent
my $signed = Record::Signed->new(
    title  => 'HOP',
    author => 'MJD',
    year   => 2005,
    rating => 4,
);
print $signed->sign, "\n";
printf "signed isa Record: %s\n", $signed->isa('Record') ? 'yes' : 'no';

# calling through a computed method name (symbolic dispatch)
for my $field (qw(title year)) {
    printf "%-6s => %s\n", $field, $signed->$field;
}

# objects go out of scope here; DESTROY guard means no autoload explosion

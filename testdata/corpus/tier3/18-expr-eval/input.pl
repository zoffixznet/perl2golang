#!/usr/bin/perl
# Recursive descent expression evaluator: tokenizer, precedence-climbing
# grammar, right-associative '^', unary minus, parens, variables, and
# per-line error recovery. Reads one expression per line on stdin.
use strict;
use warnings;

# grammar:
#   stmt    := IDENT '=' expr | expr
#   expr    := term  (('+'|'-') term)*
#   term    := factor (('*'|'/'|'%') factor)*
#   factor  := power
#   power   := unary ('^' power)?          # right associative
#   unary   := '-' unary | primary
#   primary := NUMBER | IDENT | '(' expr ')'

my %vars = ( pi => 3.14159, e => 2.71828 );

sub tokenize {
    my ($src) = @_;
    my @toks;
    while ( $src =~ /\G\s*( \d+(?:\.\d+)? | [A-Za-z_]\w* | [-+*\/%^()=] )/gcx )
    {
        push @toks, $1;
    }
    $src =~ /\G\s*/gc;    # eat trailing whitespace, pos() kept by /c
    my $pos = pos($src) // 0;
    die "lex error near '" . substr( $src, $pos, 8 ) . "'\n"
      if $pos < length $src;
    return \@toks;
}

# --- parser state: token array + cursor, closed over by helpers ---------
my ( $toks, $ix );
sub peek { $ix < @$toks ? $toks->[$ix] : undef }
sub take { $toks->[ $ix++ ] }

sub expect {
    my ($want) = @_;
    my $got = peek();
    die "expected '$want', got " . ( defined $got ? "'$got'" : 'end' ) . "\n"
      unless defined $got && $got eq $want;
    take();
}

sub parse_primary {
    my $t = peek();
    die "unexpected end of input\n" unless defined $t;
    if ( $t eq '(' ) {
        take();
        my $v = parse_expr();
        expect(')');
        return $v;
    }
    if ( $t =~ /^\d/ ) { return take() + 0 }
    if ( $t =~ /^[A-Za-z_]/ ) {
        take();
        die "undefined variable '$t'\n" unless exists $vars{$t};
        return $vars{$t};
    }
    die "unexpected token '$t'\n";
}

sub parse_unary {
    if ( defined peek() && peek() eq '-' ) {
        take();
        return -parse_unary();
    }
    return parse_primary();
}

sub parse_power {
    my $base = parse_unary();
    if ( defined peek() && peek() eq '^' ) {
        take();
        my $exp = parse_power();    # recurse right for right-assoc
        return $base**$exp;
    }
    return $base;
}

sub parse_term {
    my $v = parse_power();
    while ( defined peek() && peek() =~ /^[*\/%]$/ ) {
        my $op  = take();
        my $rhs = parse_power();
        if    ( $op eq '*' ) { $v *= $rhs }
        elsif ( $op eq '/' ) {
            die "division by zero\n" if $rhs == 0;
            $v /= $rhs;
        }
        else {
            die "modulo by zero\n" if $rhs == 0;
            $v %= $rhs;             # integer semantics, like Perl's %
        }
    }
    return $v;
}

sub parse_expr {
    my $v = parse_term();
    while ( defined peek() && ( peek() eq '+' || peek() eq '-' ) ) {
        my $op = take();
        my $rhs = parse_term();
        $v = $op eq '+' ? $v + $rhs : $v - $rhs;
    }
    return $v;
}

sub evaluate {
    my ($line) = @_;
    $toks = tokenize($line);
    $ix   = 0;
    die "empty expression\n" unless @$toks;

    # assignment?
    if ( @$toks >= 2 && $toks->[0] =~ /^[A-Za-z_]\w*$/ && $toks->[1] eq '=' )
    {
        my $name = take();
        take();    # '='
        my $v = parse_expr();
        die "trailing garbage after assignment\n" if defined peek();
        $vars{$name} = $v;
        return ( $name, $v );
    }
    my $v = parse_expr();
    die "trailing garbage: '" . peek() . "'\n" if defined peek();
    return ( undef, $v );
}

sub fmt { my $n = $_[0]; $n == int $n ? sprintf '%d', $n : sprintf '%g', $n }

my $lineno = 0;
while ( my $line = <STDIN> ) {
    $lineno++;
    chomp $line;
    next if $line =~ /^\s*(#|$)/;
    my ( $name, $val ) = eval { evaluate($line) };
    if ($@) {
        chomp( my $err = $@ );
        printf "%2d| %-24s !! %s\n", $lineno, $line, $err;
    }
    elsif ( defined $name ) {
        printf "%2d| %-24s => %s = %s\n", $lineno, $line, $name, fmt($val);
    }
    else {
        printf "%2d| %-24s => %s\n", $lineno, $line, fmt($val);
    }
}
print "--- final symbol table ---\n";
printf "%-8s = %s\n", $_, fmt( $vars{$_} ) for sort keys %vars;

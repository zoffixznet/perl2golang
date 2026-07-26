#!/usr/bin/perl
# Deployment preflight: reads config from %ENV with fixed defaults, uses
# local %ENV overrides, installs signal handlers (never fired), and
# collects warnings via $SIG{__WARN__}. "Now" comes from a pinned epoch.
use strict;
use warnings;
use POSIX qw(strftime);

# ---- clean environment namespace ---------------------------------------
# Preflight must not inherit DEPLOY_* from whoever invoked it: a variable
# left over in an operator's shell has picked the wrong region before now.
# Everything below therefore starts from the defaults in this file.
delete @ENV{ grep { /^DEPLOY_/ } keys %ENV };

# ---- pinned clock ------------------------------------------------------
my $epoch = $ENV{DEPLOY_EPOCH} // 1717243200;    # 2024-06-01T12:00:00Z
my @gm    = gmtime $epoch;
my $stamp = strftime '%Y-%m-%dT%H:%M:%SZ', @gm;
print "preflight at $stamp (epoch $epoch)\n";
printf "weekday index %d, day-of-year %d\n", $gm[6], $gm[7] + 1;

# ---- environment-driven config with defaults ---------------------------
my %cfg = (
    stage    => $ENV{DEPLOY_STAGE}    // 'staging',
    region   => $ENV{DEPLOY_REGION}   // 'eu-central-1',
    replicas => $ENV{DEPLOY_REPLICAS} // 2,
    debug    => $ENV{DEPLOY_DEBUG}    // 0,
);
print "config: ", join( ' ', map { "$_=$cfg{$_}" } sort keys %cfg ), "\n";

# ---- warning collector -------------------------------------------------
my @warnings;
local $SIG{__WARN__} = sub { push @warnings, $_[0]; return };

warn "replica count below 3 in $cfg{stage}\n" if $cfg{replicas} < 3;
warn "debug enabled in production!\n" if $cfg{debug} && $cfg{stage} eq 'prod';

# ---- signal handlers: installed and inspected, never relied upon -------
my $interrupted = 0;
$SIG{INT}  = sub { $interrupted++; };
$SIG{TERM} = 'IGNORE';
$SIG{HUP}  = 'DEFAULT';

for my $sig (qw(INT TERM HUP USR1)) {
    my $h = $SIG{$sig};
    my $desc =
        !defined $h          ? 'unset'
      : ref $h eq 'CODE'     ? 'coderef'
      : $h eq 'IGNORE'       ? 'ignored'
      : $h eq 'DEFAULT'      ? 'default'
      :                        "string($h)";
    printf "SIG%-4s => %s\n", $sig, $desc;
}
print "interrupted flag: $interrupted\n";

# ---- local %ENV override, scoped and restored --------------------------
$ENV{DEPLOY_PATH_STYLE} = 'legacy';

sub effective_style { $ENV{DEPLOY_PATH_STYLE} // '(none)' }

print "style before block: ", effective_style(), "\n";
{
    local $ENV{DEPLOY_PATH_STYLE} = 'modern';
    print "style inside block: ", effective_style(), "\n";
    {
        local $ENV{DEPLOY_PATH_STYLE};    # localized to undef
        delete $ENV{DEPLOY_PATH_STYLE};
        print "style inner delete: ", effective_style(), "\n";
    }
    print "style back to outer local: ", effective_style(), "\n";
}
print "style after block: ", effective_style(), "\n";

# ---- env sanitation pass ----------------------------------------------
my @suspicious = sort grep { /^DEPLOY_SECRET/ } keys %ENV;
printf "secrets in env: %d%s\n", scalar @suspicious,
  @suspicious ? ' (' . join( ',', @suspicious ) . ')' : '';

# PATH-style split on a synthetic variable (never the real PATH).
local $ENV{DEPLOY_SEARCH} = '/opt/tools:/usr/local/deploy::/opt/tools';
my ( %seen, @clean );
for my $d ( split /:/, $ENV{DEPLOY_SEARCH} ) {
    next if $d eq '' || $seen{$d}++;
    push @clean, $d;
}
print "search path: ", join( ':', @clean ), "\n";

# ---- report ------------------------------------------------------------
printf "%d warning(s):\n", scalar @warnings;
print "  - $_" for @warnings;
my $verdict =
  @warnings && $cfg{stage} eq 'prod' ? 'BLOCK'
  : @warnings                        ? 'PROCEED WITH CAUTION'
  :                                    'CLEAR';
print "verdict: $verdict\n";
exit( $verdict eq 'BLOCK' ? 2 : 0 );

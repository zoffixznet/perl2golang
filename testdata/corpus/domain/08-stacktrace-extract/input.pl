#!/usr/bin/perl
# trace-extract -- pull Java stack traces out of an app log and dedupe.
#
# A small state machine: NORMAL until we see an exception header, then
# IN_TRACE while frames / "Caused by:" / "... N more" lines continue.
# Dedup key is exception class + first app frame, NOT the message text,
# because messages embed order ids and would never dedupe.
use strict;
use warnings;

my $APP_PREFIX = 'com.example.';   # frames "we own" -- overridable someday

my $file = shift @ARGV or die "usage: $0 <app-log>\n";
open my $fh, '<', $file or die "open $file: $!\n";

my $HEADER_RE = qr{
    ^
    (?<class> [\w.]+ (?:Exception|Error) )   # fully-qualified class
    (?: : \s (?<message> .*) )?              # optional message
    $
}x;

my $FRAME_RE = qr{
    ^\t at \s
    (?<frame> \S+ )
    \( (?<src> [^)]*) \)
    \s* $
}x;

my (@traces, $cur);
my $state = 'NORMAL';

while (my $line = <$fh>) {
    chomp $line;

    if ($state eq 'NORMAL') {
        if ($line =~ $HEADER_RE) {
            $cur = {
                class    => $+{class},
                message  => $+{message} // '',
                frames   => [],
                caused   => [],
                start    => $.,
            };
            $state = 'IN_TRACE';
        }
        next;
    }

    # state IN_TRACE
    if ($line =~ $FRAME_RE) {
        push @{ $cur->{frames} }, { frame => $+{frame}, src => $+{src} };
    }
    elsif ($line =~ /^Caused by: (.+)$/) {
        my $cause = $1;
        $cause =~ s/:.*// unless $cause =~ /^\s*$/;   # class only
        push @{ $cur->{caused} }, $cause;
    }
    elsif ($line =~ /^\t\.\.\. \d+ more$/) {
        # suppressed common frames -- part of the trace, nothing to record
    }
    else {
        finish_trace();
        $state = 'NORMAL';
        # the current line might itself be a new header (back-to-back
        # traces happen when the executor logs them without timestamps)
        if ($line =~ $HEADER_RE) {
            $cur = { class => $+{class}, message => $+{message} // '',
                     frames => [], caused => [], start => $. };
            $state = 'IN_TRACE';
        }
    }
}
finish_trace() if $state eq 'IN_TRACE';
close $fh;

# ---------------- dedupe + report ----------------
my (%groups, @order);
for my $t (@traces) {
    my $key = $t->{key};
    push @order, $key unless $groups{$key};
    push @{ $groups{$key} }, $t;
}

printf "%d traces, %d distinct\n\n", scalar @traces, scalar @order;

for my $key (@order) {
    my @g = @{ $groups{$key} };
    my $t = $g[0];
    printf "%dx %s\n", scalar @g, $t->{class};
    printf "    message:   %s\n", $t->{message} || '(none)';
    printf "    app frame: %s\n", $t->{app_frame} // '(no app frames)';
    printf "    depth:     %d frames", scalar @{ $t->{frames} };
    printf ", caused by %s", join(' <- ', @{ $t->{caused} }) if @{ $t->{caused} };
    print  "\n";
    printf "    at lines:  %s\n", join(', ', map { $_->{start} } @g);
    print  "\n";
}
exit 0;

# --------------------------------------------------------------
sub finish_trace {
    return unless $cur and @{ $cur->{frames} };
    # first frame in code we own; falls back to very first frame
    my ($app) = grep { index($_->{frame}, $APP_PREFIX) == 0 } @{ $cur->{frames} };
    $cur->{app_frame} = $app
        ? "$app->{frame} ($app->{src})"
        : undef;
    $cur->{key} = join '|', $cur->{class},
        $app ? $app->{frame} : $cur->{frames}[0]{frame};
    push @traces, $cur;
    undef $cur;
}

# What this exercises

A chaining builder: every with_* method returns $self, and a subclass built
through the base's chain must come out of it still knowing what it is, so
its own run() is the one that fires and ref $self answers with its name.
Around it, three guard lifetimes: a lock scoped to a bare block that
releases at the closing brace, one released by hand with undef, and one
that lives exactly as long as a sub call and releases as the sub returns,
before the caller's print.

# What makes it hard

Inside a promoted method the Go receiver is the embedded base struct, so
returning it would strip the subclass; the return has to travel through the
object's own back-pointer, and the method's result type becomes the
hierarchy's interface. The guards need three different spellings for one
Perl mechanism: an explicit call at the brace, a call at the undef, and a
defer for the sub body.

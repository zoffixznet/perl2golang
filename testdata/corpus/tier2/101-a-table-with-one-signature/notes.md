# What this exercises

Closures sharing a slot in every shape that can still keep one written
signature. A dispatch table whose members disagree about arity, with one
member reading $_[0] instead of naming its argument and one position
holding an int where the rest hold strings. A fallback sub joined into
the table's type through ||. A pipeline of one-argument stages held in an
array. And a counter closure whose parameter's type exists nowhere in its
body: the numbers at the call sites are the only evidence.

# What makes it hard

A Go map or slice gives all of these values exactly one type, so the
members have to agree on a signature none of them wrote. The agreement
has to come from three places at once: the parameters the bodies name,
the positions they read by number, and the arguments the calls pass. A
member that reads fewer positions than the widest one still has to accept
the full list, and the call that passes two arguments to a one-argument
formatter has to stay legal, because in Perl the extra argument simply
sat unread in @_.

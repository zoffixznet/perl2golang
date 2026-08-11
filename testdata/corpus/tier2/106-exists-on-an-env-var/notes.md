# What this exercises

`exists $ENV{X}` against variables holding "0", holding the empty
string, never set, and deleted. Exists asks whether the variable is set
at all, which is a different question from whether its value is true.

# What makes it hard

os.Getenv cannot ask the question: it returns the empty string both for
an unset variable and for one set to empty, and a truthiness test on it
also calls "0" absent. The faithful form is os.LookupEnv's second
result, which is the comma-ok idiom applied to the environment. A
conversion that reaches for Getenv prints "no" on the first two lines
where Perl prints "yes".

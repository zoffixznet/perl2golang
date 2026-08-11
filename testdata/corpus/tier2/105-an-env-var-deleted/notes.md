# What this exercises

%ENV as an editable hash: setting a variable, deleting one and reading
the value the delete answers with, reading a deleted variable through a
`//` default, and deleting a variable that was never set.

# What makes it hard

The environment is not a map, so each hash operation has its own Go
call: assignment is os.Setenv, a read is os.Getenv, and delete is
os.Unsetenv, with the removed value read out first when the delete's
result is used. The trap this entry guards against is the delete being
dropped on the floor: a conversion that skips it leaves the variable
set, and "read after delete" prints the old value instead of the
default.

# 43-a-glob-slot-on-a-handle: a filehandle carrying its own fields

Group: **B — convertible only by rewriting the data model**

## Construct

`*$self->{Strict}`, the way the IO:: modules give a handle fields: the
object is a reference to a glob, and the fields live in the glob's HASH
slot. Writes at lines 17, 18 and 26, reads at 22 and 30.

## Why it resists conversion

A glob is one entry in the symbol table with a separate slot per sigil, and
Go has no symbol table to reach into. The translation is not a translation
of the expression at all: it is a change of data model, from "a handle with
slots hanging off its name" to "a struct holding an *os.File beside its
flags". Nothing local can do that.

## What the tool must do

Refuse it, at each site, saying what a glob's slots are and what the struct
would look like. It does. Two earlier failures are gone: the form did not
parse at all, which cost eight parse errors and took the surrounding
statements down with it, and before the refusal existed the write lowered to
a hash index on the wrong value, which compiled into a program that read a
different place than the original.

## What is still recorded here

The generated program does not build. The object is a reference to a glob,
so it has no class the method calls can resolve against, and `$h->errno`
comes out as a method call on a value of no type. Making the whole idiom
convert means recognising `\do { local *HANDLE }` as an object constructor
and giving the blessed value the class's type, which is a larger piece of
work than the refusal this entry is judged by.

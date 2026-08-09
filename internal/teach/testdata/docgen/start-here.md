# Start here

You asked for a conversion and got a directory. This page says what is in it, how far the conversion actually got, and the order to read things in. Ten minutes here saves an hour of guessing.

## What was produced

- The clean program, at the root of this directory. Run it with `go run .` from the parent directory of this `docs/` folder.
- The annotated program, in `annotated/`. Same behaviour, heavy commentary. Run it with `go run ./annotated`.
- This documentation. It is specific to your file: the lessons in `concepts/` were chosen by what your code actually does, not from a fixed curriculum.
- [The project readme](../README.md), which repeats the build instructions.

## How completely it converted

`summarise.pl` is 21 lines long. Of the 24 statements in it, 22 converted directly. 1 construct was approximated, meaning the Go runs but does not do exactly what the original did, and 1 construct was refused outright, meaning no Go was produced for it at all. There are 2 TODO markers in the generated code, each naming the specific problem at the place it occurs. No refusal stops the program: each one stands in for the value its position wanted, so this builds and runs, does the parts it could, and says on standard error which gap it walked past. Type inference gave a concrete Go type to 5 of the 7 variables it tracked; the other 2 fall back to a dynamic value, which works but is not the Go you would write by hand. Treat this as a starting point that still needs work, not a finished port.

## What to read, in what order

1. [The walkthrough](walkthrough.md). Your file and its translation, region by region, each remark keyed to a line. This is the one to read while looking at the generated code in another window.
2. [The conversion report](conversion-report.md). The counts, and every construct the converter had something to say about.
3. [What did not translate](not-translated.md). The list of things you have to write yourself, with the reasoning for each.
4. [The concept lessons](concepts/index.md). The lessons your code triggered, ordered so that nothing depends on something you have not read yet.
5. [Go for Perl developers](go-for-perl-developers.md). The general tour, worth reading once whether or not it relates to this program.
6. [The exercises](exercises.md). Small tasks against this code. Reading Go is not learning Go; changing it is.

## A word on how to use the annotated program

The annotated copy is not a text file with code in it. It is the same program, so you can edit it, break it, and see what the compiler says. That feedback loop is the fastest way into a new language, and the comments in it are placed where the surprises are rather than where prose is easy to write. Each explanation appears once, at the first place it applies, so the file stays readable top to bottom.

Written by perl2golang 0.1.0, from your source.

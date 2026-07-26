# Long-form guide sources

This directory holds the long-form teaching documents that are copied into every
generated `docs/` directory verbatim, as opposed to the per-concept lessons in
`kb/`, which are selected by what a script actually triggered.

One file is looked for by name:

- `go-for-perl-developers.md` - the general orientation shipped as
  `docs/go-for-perl-developers.md`.

The document generator reads it at run time from an `embed.FS` over this
directory. If the file is absent, the generator falls back to composing a short
orientation from the knowledge base, so its absence is never a build failure.
This placeholder exists so that the `//go:embed guide/*.md` directive always has
something to match; it is never copied into a generated project.

Relative links inside `go-for-perl-developers.md` are rewritten when the file is
copied: a link whose target names a concept lesson present in the generated
bundle is pointed at that lesson, and a link to a lesson that this particular
conversion did not trigger becomes plain text with an `explain` pointer, because
the generated bundle only carries the lessons the script earned.

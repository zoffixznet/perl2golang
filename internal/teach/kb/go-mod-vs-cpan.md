---
id: go-mod-vs-cpan
title: Modules, go.mod, and where code lives
tags: [orientation, modules, dependencies, layout]
perl_triggers: [use-lib, inc-array, cpanm, makefile-pl, cpanfile, local-lib, require, use-parent]
severity: info
prerequisites: [packages-and-exported-names]
---

There is no `@INC`, no `use lib`, no `local::lib`, no install step, and no system-wide site_perl: a Go module is a directory tree with a `go.mod` file at its root, dependencies are declared in that file with exact versions, and `go build` fetches, caches, and verifies them without you running an installer. The practical consequence is that "works on my machine because of what happens to be installed" is not a Go failure mode — the build is reproducible from `go.mod` + `go.sum` alone — but you must unlearn the reflex of reaching for a library for everything (see `small-stdlib-philosophy`).

## The Perl you know

```perl
# Makefile.PL / cpanfile declares deps loosely:
requires 'JSON::MaybeXS' => '1.004';

# runtime resolution walks @INC:
use lib "$FindBin::Bin/../lib";
use My::App::Config;    # found wherever @INC says today
```

Versions are minimums, installation is a separate step (`cpanm`), and two apps on one box can fight over site_perl unless you build per-app `local::lib`s or Carton bundles.

## The Go you write

Real transcript of starting a project:

```
$ mkdir payroll && cd payroll
$ go mod init example.com/payroll
go: creating new go.mod: module example.com/payroll
```

The generated `go.mod`:

```
module example.com/payroll

go 1.26.5
```

Adding a dependency is `go get github.com/some/pkg`, which records an exact version in `go.mod` and a cryptographic checksum in `go.sum`; both files are committed. Imports are always full module paths — `import "github.com/some/pkg"` — never file paths, and the mapping from import path to code is `go.mod`'s job, not a runtime search.

Layout is convention-light but real. A typical service:

```
payroll/
├── go.mod
├── go.sum
├── main.go              # package main for a single-binary project, or:
├── cmd/
│   └── payrolld/
│       └── main.go      # each cmd/<name> builds one binary
├── internal/
│   └── ledger/          # importable ONLY within this module (compiler-enforced)
│       └── ledger.go
└── tax/
    └── tax.go           # package tax; public if the module is published
```

`internal/` is the one magic directory name: packages under it cannot be imported from outside the module, giving you module-private code the way lower-case gives package-private identifiers. There is no `src/`, no `lib/` convention, and small projects are encouraged to be flat — a single `main.go` at the root is idiomatic, not a prototype smell.

## The mismatch

Three CPAN habits to drop. First, version bounds: Perl's `requires '1.004'` means "at least"; `go.mod` records the exact version you got, upgrades happen only when you run `go get -u` or edit the file, and the checksum database detects a republished-but-different tarball — dependency confusion attacks that CPAN trust models struggle with are largely closed off. Second, installation: there is no `make install` for libraries; the module cache (`$GOMODCACHE`) is invisible plumbing you never manage by hand. Third, code loading: nothing resembling `require $computed_name` or runtime plugin discovery via `@INC` hooks exists — the import graph is fixed at compile time. When you need the ecosystem, https://pkg.go.dev is the CPAN search equivalent, with documentation rendered for every public module automatically.

Further reading: https://go.dev/doc/modules/managing-dependencies and https://go.dev/doc/modules/layout

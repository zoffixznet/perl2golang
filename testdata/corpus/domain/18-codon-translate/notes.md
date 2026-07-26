# 18-codon-translate

**Domain:** bioinformatics. **Multi-file entry**: `input.pl` plus
`CodonTable.pm`. Translates CDS FASTA records to protein using
per-record header attributes (`table=`, `strand=`, `frame=`); minus
strand is reverse-complemented first. A record with an unknown codon
table dies inside the module and is caught per record (`eval`), so the
batch completes with exit 1 and a FAILED report line.

## Constructs exercised
- A real Perl module: `package CodonTable`, `Exporter` with
  `@EXPORT_OK`, selective import list in `use CodonTable qw(...)`,
  `FindBin`/`use lib` for script-relative loading, and the `1;` truth
  requirement.
- **Three-level nested data**: `%TABLES{table}{codons}{codon}` (plus
  `starts`/`stops` sets), populated at load time by a triple loop over
  `qw(T C A G)` walking a 64-char amino-acid string with `substr` -- a
  file-scoped `{ ... }` bare block acting as a module initialiser.
- `translate` returns `($protein, \%info)` -- scalar + hashref pair.
- `$info{internal_stops} = () = $body =~ /\*/g` -- the "countof"
  goatse idiom (force list context, then count).
- `(my $rc = reverse $dna) =~ tr/.../.../` copy-reverse-complement.
- Header attribute parsing straight into a hash: `my %attrs = $attr_str
  =~ /(\w+)=(\S+)/g` -- a global match in list context feeding a hash.
- 4-arg `substr($s, 0, 60, '')` (replacement form) in `wrap60`.

## Conversion challenges
- Module -> Go package: exports become capitalised identifiers, the
  loader block becomes `init()` or a `sync.Once`/package-level var --
  a converter must recognise the bare block as one-time initialisation.
- `my %attrs = $str =~ /.../g`: pair-list-to-hash from a regex is a
  dense idiom; Go needs `FindAllStringSubmatch` plus explicit map build.
- `() = $body =~ /\*/g` count idiom -- meaningless if translated
  literally; must become `strings.Count`.
- The `eval { translate(...) }` returning a *list*: on die, `$protein`
  is undef and that undef-ness (not an error variable) is the failure
  test, with `$@` fetched afterwards -- mapping to Go's `(string, Info,
  error)` requires reshaping the API, and the module's `die` messages
  surface verbatim in output.
- 4-arg `substr` destructively consumes the string -- Go strings are
  immutable, so `wrap60` must be restructured (slicing with an index).
- `starts`/`stops` are hash-sets; `$t->{starts}{$codon} ? 1 : 0`
  materialises the boolean -- struct-with-map-sets in Go.

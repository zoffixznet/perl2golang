// Package ai is the local-model integration, kept as a measurement
// instrument rather than as a product feature.
//
// It was built to let a local model improve the conversion: shown the
// original Perl beside the generated Go and the converter's own notes, the
// model proposes repairs, and this package splices, checks, compiles and vets
// each one before anything is kept. Measured over the full corpus with
// qwen2.5-coder:7b (see `make score-ai`), the honest result was zero improved
// entries out of 228, five behavioural corruptions caught only because the
// corpus knows the right output, and eight rewrites that changed nothing
// measurable. On that evidence the user-facing mode was removed; this package
// stays so the same question can be re-asked against the same gates when
// local models are better. Nothing in the perl2golang binary imports it; its
// only driver is `cmd/score -ai`.
//
// Nothing in this package runs unless the caller asks for it. Constructing a
// [Client] opens no socket, starts no process and reads no model: every network
// operation lives behind a method that takes a [context.Context], and the HTTP
// transport itself is built on first use. There is no package-level state that
// talks to anything, and no init function.
//
// The only supported runtime is a locally hosted one speaking the Ollama HTTP
// API, by default on http://localhost:11434. There is no hosted service, no
// account and no API key anywhere in this package, and the endpoint is expected
// to be a loopback address.
//
// The governing rule for everything below is that model output can improve the
// deterministic result or leave it unchanged, never corrupt it:
//
//   - Every Go source string a model produces is spliced by this package, not
//     by the model, and the result must parse ([VerifyGo]), keep the same
//     declarations and exported surface, add no imports beyond what a repair
//     declares from the standard library, and add no panic/unsafe/exit paths
//     before the caller is allowed to see it.
//   - Every prose result must survive the grounding checks in [Client.EnrichDoc]:
//     it may only quote code it was shown, name packages that exist, and it may
//     not claim equivalence for a unit with known deviations.
//   - A failure of any kind - runtime missing, runtime down, timeout, out of
//     memory, malformed output, output that does not parse - is returned as a
//     typed error and the caller's input comes back unchanged. This package
//     never panics on model output and never writes files.
//
// The corpus measurement is the gate these rules cannot provide: structural
// checks prove a repair still builds, and only running the program against
// perl's recorded output proves it still behaves. That check lives in
// internal/score and is the reason the instrument is worth keeping.
//
// Progress text is written only to the [Options.Progress] writer supplied by
// the caller. This package writes nothing to the standard streams.
//
// The model catalogue ([Catalogue], [Lookup], [PreferredModel]) records which
// freely licensed models are worth measuring against and why some are
// excluded, so a future re-run starts from the same footing.
package ai

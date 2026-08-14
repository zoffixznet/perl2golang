// Package ai is the optional local-model integration.
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
//     declarations and exported surface, keep every string and numeric literal,
//     add no imports and add no panic/unsafe/exit paths before the caller is
//     allowed to see it.
//   - Every prose result must survive the grounding checks in [Client.EnrichDoc]:
//     it may only quote code it was shown, name packages that exist, and it may
//     not claim equivalence for a unit with known deviations.
//   - A failure of any kind - runtime missing, runtime down, timeout, out of
//     memory, malformed output, output that does not parse - is returned as a
//     typed error and the caller's input comes back unchanged. This package
//     never panics on model output and never writes files.
//
// Progress text is written only to the [Options.Progress] writer supplied by
// the caller. This package writes nothing to the standard streams.
//
// The hardware probe ([Probe]), the model catalogue ([Catalogue], [Recommend])
// and the setup planner ([PlanSetup]) target Debian-like Linux and degrade
// silently when an optional tool is missing.
package ai

# Pass criteria

- category: `approximate` (per-site report where the lifetime cannot be
  followed) or `refuse-statement`
- diagnostic-must-contain: `DESTROY` or `destructor`, `reference`
- diagnostics reference the guard's construction site (`input.pl:24`)
- generated-code-must: convert everything around the guard and run to the
  last line; the `unlock shared-state` line may be missing from the output,
  and that absence must be what the report describes
- must-not: emit a destructor call at the closing brace or at program exit
  as though the lifetime were block-shaped; must-not use
  runtime.SetFinalizer; must-not drop the DESTROY conversion silently

The sibling shapes, a guard that stays inside its block, dies at an undef,
or lives exactly as long as a sub call, convert exactly. This one escapes
through `remember($g)`: from there on, when the reference count reaches
zero is the count's own business, and the honest outcome is a report entry
at the construction site telling the reader which method to call by hand.

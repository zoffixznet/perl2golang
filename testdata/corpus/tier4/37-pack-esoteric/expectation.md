# Pass criteria

- category: `todo` (a per-call refusal, with the rest of the file converted)
- report entries cite `input.pl:13` (the `%` checksum), `input.pl:18` (the
  `w` BER integer) and `input.pl:21` (the `>` modifier)
- diagnostic-must-contain: `template`, and the offending code spelled out
- must-not: hand the template to the emitted interpreter anyway, which would
  turn a conversion-time answer into a run-time stop; must-not refuse the
  whole file, since every statement around the templates converts

# Pass criteria

- category: `shim` (save / defer-restore)
- report entries cite `input.pl:15` and `input.pl:22`
- diagnostic-must-contain: `local`, `dynamic`, `restore`
- converted program output must match `expected_stdout` byte-for-byte; line 2
  must read `mode=debug` (callee sees the localized value) and line 4 must show
  restoration after die
- must-not: translate `local $mode` into a new lexical variable

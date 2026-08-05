# Pass criteria

- category: `todo` (a per-call refusal; the statements around the forks keep
  converting)
- report entries cite the `fork` calls at `input.pl:12` and `input.pl:27` and
  the `waitpid` calls at `input.pl:21` and `input.pl:32`
- diagnostic-must-contain: `goroutine`, `exec.Command`, and for waitpid the
  waiting counterparts `cmd.Wait` or `WaitGroup`
- the degraded program must stop at the first fork the way a failed fork
  would: the undef stand-in fails the `defined` test and the `die "fork:"`
  branch runs
- must-not: emit a goroutine for the child block, which cannot carry
  `exit 7` into `$? >> 8`; must-not emit a bare panic that makes the
  converted lines above the fork unreachable

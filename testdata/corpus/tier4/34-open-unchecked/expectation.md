# Pass criteria

- category: `convert-verify` with a MANDATORY unchecked-open warning
- converted program must exit 0 and match `expected_stdout` byte-for-byte
  (including the errno text `No such file or directory`), and must not panic
- report-must-contain: `open`
- report-must-contain: `not checked` `unchecked` - either phrase, and a
  recommendation to add error handling
- must-not: insert log.Fatal/os.Exit(1) on open failure; must-not read
  through a nil file handle

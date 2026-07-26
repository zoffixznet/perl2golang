# 07-comparison-operators

## What this exercises
Perl's two parallel families of comparison operators: numeric
(`== != < > <= >= <=>`) and string (`eq ne lt gt le ge cmp`). Which one you
pick, not the operand types, decides the comparison. `"10" == "10.0"` is true
numerically and false stringwise; `2 < 10` is true but `"2" lt "10"` is false.

`<=>` and `cmp` return -1 / 0 / 1 and are the building blocks of `sort`.
`"Zed" cmp "apple"` is negative because comparison is by codepoint and
uppercase sorts before lowercase in ASCII.

## Perl constructs
- both operator families side by side on the same operands
- `<=>` and `cmp` three-way comparison
- default `sort` (string) vs `sort { $a <=> $b }` (numeric)

## Go concepts a converter must teach
- Go has **one** set of comparison operators, dispatched by static type. The
  converter must decide, per comparison site, whether to emit a numeric compare
  (parsing operands if they are strings) or a string compare.
- `<=>` maps to `cmp.Compare(a, b)` for numbers; `cmp` maps to
  `strings.Compare(a, b)`. Both return -1/0/1 like Perl.
- The classic conversion bug lives here: lowering `eq` to `==` is fine, but
  lowering `==` to `==` on two Go strings silently changes semantics
  (`"10" == "10.0"` becomes false). Any `==` on values the converter typed as
  strings must go through `strconv.ParseFloat`.
- Go's `sort.Strings` matches Perl's default `sort`; Perl's default sort is
  *always* stringwise, which surprises people converting numeric data.

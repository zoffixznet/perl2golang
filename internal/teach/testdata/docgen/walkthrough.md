# summarise.pl, line by line

Each section below takes one region of the original, shows the Go it became, and explains the choice. The line numbers refer to the original file, so you can keep it open beside this page. Read the sections in order: later ones assume the earlier ones, and an explanation is given once, at the first region it applies to.

Where a region raises something structural about Go rather than something local to your code, it links to a lesson in [concepts/](concepts/index.md). Those lessons stand alone, so follow them when you want to and skip them when you do not.

## 1. Opening the input file

`summarise.pl`, lines 5 to 8:

```perl
my $file = shift @ARGV or die "usage: summarise.pl FILE\n";

my %count;
open my $fh, '<', $file or die "cannot open $file: $!\n";
```

The Go it became:

```go
f, err := os.Open(os.Args[1])
if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
}
defer f.Close()
```

`open ... or die` is two decisions in one line: do the work, and stop if it failed. Go splits them, because `os.Open` returns the file and an error together and neither one is special. The `defer f.Close()` has no counterpart in the original, where the filehandle closed itself when `$fh` left scope; Go closes nothing on your behalf, so the cleanup is written down at the point the resource is acquired.

Lessons this region introduces: [Errors are return values, not exceptions](concepts/errors-are-values.md) and [nil is not undef, and nothing autovivifies](concepts/nil-vs-undef.md)

---

## 2. Counting by key

`summarise.pl`, lines 9 to 13:

```perl
while (my $line = <$fh>) {
    chomp $line;
    next unless $line =~ /^(\S+)\s+(\d+)$/;
    $count{$1} += $2;
}
```

The Go it became:

```go
scanner := bufio.NewScanner(r)
for scanner.Scan() {
    m := linePattern.FindStringSubmatch(scanner.Text())
    if m == nil {
        continue
    }
    n, err := strconv.Atoi(m[2])
    if err != nil {
        return nil, fmt.Errorf("parsing %q: %w", m[2], err)
    }
    counts[m[1]] += n
}
```

Three things changed. The read loop became a scanner, which strips the newline for you, so `chomp` disappears. The capture variables `$1` and `$2` became a slice of strings, indexed from one because element zero is the whole match. And `+= $2` became an explicit conversion: Go will not add a string to an integer, so the failure that Perl hid behind a zero is now a value you have to do something with.

Lessons this region introduces: [strconv turns strings into numbers, and refuses to guess](concepts/strconv-parsing.md), [FindStringSubmatch replaces $1, and no-match returns nil](concepts/submatch-and-named-groups.md), and [bufio.Scanner reads lines, and gives up on long ones](concepts/bufio-scanner-limit.md)

---

## 3. Sorting the report

`summarise.pl`, lines 16 to 18:

```perl
for my $key (sort { $count{$b} <=> $count{$a} } keys %count) {
    printf "%-20s %6d\n", $key, $count{$key};
}
```

The Go it became:

```go
keys := make([]string, 0, len(counts))
for k := range counts {
    keys = append(keys, k)
}
sort.Slice(keys, func(i, j int) bool { return counts[keys[i]] > counts[keys[j]] })
for _, k := range keys {
    fmt.Printf("%-20s %6d\n", k, counts[k])
}
```

The one-liner became six lines, and the extra lines are the ones that were always there implicitly: collecting the keys, choosing an order for them, then printing. The comparison function returns a bool meaning "i sorts before j" rather than the three-way result of `<=>`, so a descending sort is `>` instead of a reversed pair of operands.

Lessons this region introduces: [Sorting is a function call, and the default is numeric-aware](concepts/sort-slice.md), [Map order is randomised per loop, on purpose](concepts/map-iteration-order.md), [range gives you the index first, and the element is a copy](concepts/range-is-not-foreach.md), and [printf survives the port, but the verbs are typed now](concepts/fmt-and-verbs.md)

---

## Next

Reading a translation is the easy half. The other half is changing it and finding out what the compiler thinks, which is what [the exercises](exercises.md) are for: each one is small, names code that is actually in this directory, and tells you how to check that you got it right.

Written by perl2golang 0.1.0, from your source.

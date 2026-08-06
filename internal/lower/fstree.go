package lower

import (
	"strings"

	"perl2golang/internal/ir"
	"perl2golang/internal/perl/ast"
)

// This file covers the three modules a script reaches for when it builds,
// walks or tears down a directory tree: File::Temp, File::Path and
// File::Find. All three are in the standard library here, under `os` and
// `path/filepath`, with one difference running through all of them: the Go
// call reports an error and nothing else, where the Perl one reported what it
// did.

// tempDirCall lowers File::Temp::tempdir.
//
// The options are named pairs, and the two that matter are DIR, which says
// where, and TEMPLATE or a leading template string, which says what the name
// looks like. CLEANUP has no counterpart at all: os.MkdirTemp removes
// nothing, and a deferred os.RemoveAll is where a Go program says when the
// directory goes.
func (l *Lowerer) tempDirCall(n *ast.Call) ir.Expr {
	parent, pattern, cleanup := l.tempOptions(n)
	out := l.helperCall(hTempDir, ir.TString, parent, pattern)
	if cleanup {
		l.approximate(n, "P2G7581", "tempdir CLEANUP",
			"nothing removes the directory when the program ends",
			"CLEANUP asks File::Temp to delete the tree when the process exits. "+
				"os.MkdirTemp has no such option, and Go's answer is to say so at the "+
				"point the directory is made.",
			"Write `defer os.RemoveAll(dir)` on the line after this one. Inside a "+
				"test, t.TempDir() does both and is what you want there.",
			"defer-timing")
	}
	l.note(out, "os.MkdirTemp puts the random part where a * appears in the pattern, "+
		"or on the end when there is none, and it returns an error because making a "+
		"directory can fail.",
		"errors-are-values", "filepath-and-paths")
	return out
}

// tempFileCall lowers File::Temp::tempfile, which hands back both an open
// handle and the name it chose.
func (l *Lowerer) tempFileCall(n *ast.Call) ir.Expr {
	parent, pattern, cleanup := l.tempOptions(n)
	handle := l.tmp("scratch")
	name := l.tmp("scratchName")
	fileT := ir.NamedType("*os.File", "os")
	st := assign(":=", []ir.Expr{ir.NewIdent(handle, fileT), ir.NewIdent(name, ir.TString)},
		[]ir.Expr{l.helperCall(hTempFile, fileT, parent, pattern)})
	l.setProv(st, n)
	l.note(st, "os.CreateTemp hands back the open file, and the name it chose is "+
		"f.Name() rather than a second result. Both are named here because the Perl "+
		"took them as a pair.",
		"multiple-return-values")
	l.emit(st)
	if cleanup {
		l.approximate(n, "P2G7581", "tempfile UNLINK",
			"nothing removes the file when the program ends",
			"UNLINK asks File::Temp to delete the file when the process exits. "+
				"os.CreateTemp has no such option.",
			"Write `defer os.Remove(f.Name())` after this line, or `defer "+
				"os.RemoveAll(dir)` once for the directory they are all in.",
			"defer-timing")
	}
	return composite(ir.SliceOf(ir.TAny), nil, []ir.Expr{
		l.assignable(ir.NewIdent(handle, fileT), ir.TAny, n),
		ir.NewIdent(name, ir.TString),
	})
}

// tempOptions reads the DIR, TEMPLATE and cleanup settings out of a File::Temp
// call, which takes an optional leading template followed by named pairs.
func (l *Lowerer) tempOptions(n *ast.Call) (parent, pattern ir.Expr, cleanup bool) {
	args := flatten(argList(n))
	parent, pattern = ir.Str(`""`), ir.Str(`""`)
	i := 0
	if len(args)%2 == 1 {
		if text, ok := staticString(args[0]); ok {
			pattern = ir.Str(quote(goPattern(text)))
			i = 1
		}
	}
	for ; i+1 < len(args); i += 2 {
		key, ok := staticString(args[i])
		if !ok {
			continue
		}
		switch strings.ToUpper(key) {
		case "DIR":
			parent = l.toStr(l.expr(args[i+1]), args[i+1])
		case "TEMPLATE":
			if text, ok := staticString(args[i+1]); ok {
				pattern = ir.Str(quote(goPattern(text)))
			}
		case "SUFFIX":
			if text, ok := staticString(args[i+1]); ok {
				pattern = ir.Str(quote("tmp-*" + text))
			}
		case "CLEANUP", "UNLINK":
			cleanup = true
		}
	}
	return parent, pattern, cleanup
}

// goPattern turns a File::Temp template into the pattern os.MkdirTemp takes:
// the run of X characters that says where the random part goes becomes a
// single star.
func goPattern(template string) string {
	if i := strings.Index(template, "XXXX"); i >= 0 {
		j := i
		for j < len(template) && template[j] == 'X' {
			j++
		}
		return template[:i] + "*" + template[j:]
	}
	return template
}

// makePathCall lowers File::Path::make_path, which creates a directory and
// everything above it and reports which levels it had to create.
func (l *Lowerer) makePathCall(n *ast.Call) ir.Expr {
	dirs, spread := l.pathArguments(n)
	out := l.helperCall(hMakeTree, ir.SliceOf(ir.TString), dirs...)
	out.Ellipsis = spread
	l.approximate(n, "P2G7582", "make_path",
		"os.MkdirAll reports an error and not what it created",
		"make_path hands back the list of directories it had to create, which is "+
			"how a script tells a fresh tree from one that was already there. "+
			"os.MkdirAll reports only whether it failed.",
		"Where only success matters, os.MkdirAll on its own is the whole call and "+
			"is idempotent. Where the list matters, look before the call.",
		"errors-are-values", "filepath-and-paths")
	l.note(out, "os.MkdirAll creates every missing level and does nothing when the "+
		"directory is already there, which is `mkdir -p`. The permissions are an "+
		"argument written in octal rather than a global umask setting.",
		"filepath-and-paths")
	return out
}

// removeTreeCall lowers File::Path::remove_tree.
func (l *Lowerer) removeTreeCall(n *ast.Call) ir.Expr {
	dirs, spread := l.pathArguments(n)
	out := l.helperCall(hRemoveTree, ir.TInt, dirs...)
	out.Ellipsis = spread
	l.approximate(n, "P2G7582", "remove_tree",
		"os.RemoveAll reports an error and not a count",
		"remove_tree hands back how many entries it removed. os.RemoveAll reports "+
			"only whether it failed, so counting means walking the tree first.",
		"Where the count is not used, os.RemoveAll on its own is the whole call, "+
			"and it succeeds on a path that was not there.",
		"errors-are-values", "filepath-and-paths")
	return out
}

// pathArguments reads the directory list of a File::Path call, dropping the
// trailing options hash the module allows.
func (l *Lowerer) pathArguments(n *ast.Call) ([]ir.Expr, bool) {
	args := flatten(argList(n))
	var out []ir.Expr
	spread := false
	for _, a := range args {
		if _, isHash := a.(*ast.AnonHash); isHash {
			l.approximate(a, "P2G7582", "make_path options",
				"the options hash was dropped",
				"The options control permissions, verbosity and where errors are "+
					"collected. os.MkdirAll takes permissions as an argument and reports "+
					"errors by returning them.",
				"Pass the mode to os.MkdirAll and check what it returns.",
				"errors-are-values")
			continue
		}
		x := l.expr(a)
		if x == nil {
			continue
		}
		if t := typeOrAny(x); t.Kind == ir.Slice {
			out = append(out, l.strSlice(x, a))
			spread = true
			continue
		}
		out = append(out, l.toStr(x, a))
	}
	if len(out) == 0 {
		out = append(out, ir.Str(`""`))
	}
	// A list argument is spread only when it is the whole argument list;
	// mixing the two would need a concatenation first.
	return out, spread && len(out) == 1
}

// findCall lowers File::Find::find, which walks a tree and runs a block for
// every entry it meets.
//
// filepath.WalkDir is the counterpart, and it is the better shape: no
// globals, the callback is handed the path and a directory entry, returning
// fs.SkipDir prunes a subtree, and any other error stops the walk and comes
// back out of the call. WalkDir also visits in lexical order, so the
// preprocess block that sorted the names has nothing left to do.
func (l *Lowerer) findCall(n *ast.Call) (ir.Expr, bool) {
	args := flatten(argList(n))
	if len(args) < 2 {
		return nil, false
	}
	body, ok := l.wantedBody(args[0])
	if !ok {
		return nil, false
	}
	roots, _ := l.listParts(args[1:])
	if len(roots) == 0 {
		return nil, false
	}

	path := l.tmp("path")
	entry := l.tmp("entry")
	walkErr := l.tmp("walkErr")
	entryT := ir.NamedType("fs.DirEntry", "io/fs")

	saved := l.scope
	l.scope = newScope(saved)
	// find changes into each directory as it walks, which is why $_ holds
	// the bare name inside the callback. WalkDir changes nothing, so $_
	// becomes the entry's name; the file tests are the one place that must
	// keep the full path instead, because there is no chdir to make a bare
	// name reachable, and they resolve it through the walk record below.
	l.topicStack = append(l.topicStack,
		ir.CallOf(selector(ir.NewIdent(entry, entryT), "Name", nil), ir.TString))
	savedFind := l.findWalk
	l.findWalk = &findWalk{path: path, entry: entry, entryType: entryT}
	savedPre := l.pre
	l.pre = nil
	stmts := l.stmts(body)
	stmts = append(l.takePre(), stmts...)
	l.pre = savedPre
	l.findWalk = savedFind
	l.topicStack = l.topicStack[:len(l.topicStack)-1]
	l.scope = saved

	// An error reaching the callback means a directory could not be read,
	// which the Perl walk passed over in silence.
	guard := &ir.If{
		Cond: ir.Bin("!=", ir.NewIdent(walkErr, ir.TError), ir.Nil(ir.TError), ir.TBool),
		Then: &ir.Block{Stmts: []ir.Stmt{&ir.Return{Results: []ir.Expr{ir.Nil(ir.TError)}}}},
	}
	stmts = append([]ir.Stmt{guard}, stmts...)
	stmts = append(stmts, &ir.Return{Results: []ir.Expr{ir.Nil(ir.TError)}})

	fn := funcLit([]ir.Param{
		{Name: path, Type: ir.TString},
		{Name: entry, Type: entryT},
		{Name: walkErr, Type: ir.TError},
	}, []*ir.Type{ir.TError}, &ir.Block{Stmts: stmts})

	var last ir.Expr
	for _, root := range roots {
		walk := call("path/filepath", "filepath", "WalkDir", ir.TError,
			l.toStr(root, n), fn)
		st := exprStmt(walk)
		l.setProv(st, n)
		l.note(st, "filepath.WalkDir hands the callback the full path and a directory "+
			"entry, so nothing has to be read from a global. It visits in lexical "+
			"order, which is what the preprocess block was arranging, and the entry "+
			"answers IsDir without a second call to stat.",
			"filepath-and-paths", "errors-are-values")
		l.emit(st)
		last = walk
	}
	l.approximate(n, "P2G7587", "File::Find::find",
		"the walk keeps no globals and does not change directory",
		"find sets $File::Find::name, $File::Find::dir and $_ before each call, and "+
			"changes into each directory as it goes, which is why $_ is a bare name "+
			"there. WalkDir passes the full path as an argument and changes nothing, "+
			"so $_ is the full path here.",
		"A callback that matched $_ against a pattern meant the last component: "+
			"filepath.Base of the path is that, and the entry's Name method is the "+
			"same thing without the work.",
		"filepath-and-paths", "range-is-not-foreach")
	if last == nil {
		return ir.BoolLit(true), true
	}
	return ir.BoolLit(true), true
}

// wantedBody digs the block out of the first argument of find, which is
// either a sub or a hash naming one.
func (l *Lowerer) wantedBody(e ast.Expr) ([]ast.Stmt, bool) {
	switch n := e.(type) {
	case *ast.AnonSub:
		return n.Body, true
	case *ast.RefGen:
		if c, ok := n.X.(*ast.Call); ok {
			if s, found := l.findSub(c.Name); found && s.Decl != nil {
				return s.Decl.Body, true
			}
		}
	case *ast.AnonHash:
		var flat []ast.Expr
		for _, el := range n.Elems {
			flat = append(flat, flatten(el)...)
		}
		for i := 0; i+1 < len(flat); i += 2 {
			key, ok := staticString(flat[i])
			if !ok {
				continue
			}
			if key == "wanted" {
				return l.wantedBody(flat[i+1])
			}
		}
	}
	return nil, false
}

// findWalk is the state a File::Find callback is lowered against: the names
// the walk gave the entry it is looking at.
type findWalk struct {
	path      string
	entry     string
	entryType *ir.Type
}

// findGlobal answers the File::Find package variables from inside a walk.
func (l *Lowerer) findGlobal(name string) (ir.Expr, bool) {
	if l.findWalk == nil {
		return nil, false
	}
	switch name {
	case "File::Find::name", "File::Find::fullname":
		return ir.NewIdent(l.findWalk.path, ir.TString), true
	case "File::Find::dir":
		return call("path/filepath", "filepath", "Dir", ir.TString,
			ir.NewIdent(l.findWalk.path, ir.TString)), true
	}
	return nil, false
}

// programDir answers $FindBin::Bin and $FindBin::RealBin, which name the
// directory the script was found in.
//
// Go has no such thing to find: imports are resolved when the program is
// compiled and there is no search path at run time. The nearest true answer
// is where the built binary sits, which is what os.Executable reports.
func (l *Lowerer) programDir(name string) (ir.Expr, bool) {
	switch name {
	case "FindBin::Bin", "FindBin::RealBin", "FindBin::Dir":
		out := l.helperCall(hProgramDir, ir.TString)
		l.note(out, "os.Executable reports the built binary's path, which is the "+
			"nearest true answer: there is no script file at run time and no search "+
			"path for one to be added to.",
			"go-mod-vs-cpan", "errors-are-values")
		return out, true
	case "FindBin::Script", "FindBin::RealScript":
		out := call("path/filepath", "filepath", "Base", ir.TString,
			l.helperCall(hProgramPath, ir.TString))
		return out, true
	}
	return nil, false
}

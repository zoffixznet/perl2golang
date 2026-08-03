package parser_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"perl2golang/internal/perl/ast"
	"perl2golang/internal/perl/parser"
)

func corpusRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "..", "..", "testdata", "corpus")
	if _, err := os.Stat(root); err != nil {
		t.Skipf("corpus not present: %v", err)
	}
	return root
}

// walkExprs visits every expression reachable from the program so the
// coverage report can count BadExpr nodes as well as Untranslated statements.
func countBad(prog *ast.Program) (untranslated, bad int) {
	var stmts func([]ast.Stmt)
	var expr func(ast.Expr)
	visitList := func(es []ast.Expr) {
		for _, e := range es {
			expr(e)
		}
	}
	expr = func(e ast.Expr) {
		switch n := e.(type) {
		case nil:
			return
		case *ast.BadExpr:
			bad++
		case *ast.InterpLit:
			visitList(n.Parts)
		case *ast.My:
			visitList(n.Vars)
		case *ast.Assign:
			expr(n.LHS)
			expr(n.RHS)
		case *ast.BinOp:
			expr(n.L)
			expr(n.R)
		case *ast.UnOp:
			expr(n.X)
		case *ast.Ternary:
			expr(n.Cond)
			expr(n.A)
			expr(n.B)
		case *ast.List:
			visitList(n.Elems)
		case *ast.Call:
			visitList(n.Args)
			stmts(n.Block)
		case *ast.MethodCall:
			expr(n.Invocant)
			expr(n.Dynamic)
			visitList(n.Args)
		case *ast.FuncCallRef:
			expr(n.Ref)
			visitList(n.Args)
		case *ast.Index:
			expr(n.Base)
			expr(n.Idx)
		case *ast.HashIndex:
			expr(n.Base)
			expr(n.Key)
		case *ast.Slice:
			expr(n.Base)
			visitList(n.Idx)
		case *ast.Deref:
			expr(n.X)
		case *ast.RefGen:
			expr(n.X)
		case *ast.AnonArray:
			visitList(n.Elems)
		case *ast.AnonHash:
			visitList(n.Elems)
		case *ast.AnonSub:
			stmts(n.Body)
		case *ast.Match:
			expr(n.Bound)
			expr(n.PatternExpr)
		case *ast.Subst:
			expr(n.Bound)
			expr(n.Repl)
		case *ast.Trans:
			expr(n.Bound)
		case *ast.QrExpr:
		case *ast.Readline:
			expr(n.Var)
		case *ast.GlobExpr:
			expr(n.Pattern)
		case *ast.FileTest:
			expr(n.Arg)
		case *ast.BacktickCmd:
			visitList(n.Parts)
		}
	}
	stmts = func(list []ast.Stmt) {
		for _, s := range list {
			switch n := s.(type) {
			case *ast.ExprStmt:
				expr(n.X)
			case *ast.If:
				expr(n.Cond)
				stmts(n.Then)
				for _, ei := range n.ElseIfs {
					expr(ei.Cond)
					stmts(ei.Then)
				}
				stmts(n.Else)
			case *ast.While:
				expr(n.Cond)
				stmts(n.Body)
			case *ast.ForC:
				expr(n.Init)
				expr(n.Cond)
				expr(n.Post)
				stmts(n.Body)
			case *ast.Foreach:
				expr(n.Var)
				visitList(n.List)
				stmts(n.Body)
			case *ast.Block:
				stmts(n.Body)
			case *ast.SubDecl:
				stmts(n.Body)
			case *ast.PackageDecl:
				stmts(n.Body)
			case *ast.Use:
				visitList(n.Args)
			case *ast.Return:
				visitList(n.Exprs)
			case *ast.Untranslated:
				untranslated++
			}
		}
	}
	stmts(prog.Stmts)
	return
}

// TestCorpusParseCoverage reports how much of the corpus parses without
// falling back to an untranslated region. It never fails: it is a coverage
// probe used to steer parser work.
func TestCorpusParseCoverage(t *testing.T) {
	root := corpusRoot(t)
	tiers, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	type stat struct{ clean, total int }
	byTier := map[string]*stat{}
	var problems []string
	for _, tier := range tiers {
		if !tier.IsDir() {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(root, tier.Name()))
		if err != nil {
			t.Fatal(err)
		}
		st := &stat{}
		byTier[tier.Name()] = st
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			src, err := os.ReadFile(filepath.Join(root, tier.Name(), e.Name(), "input.pl"))
			if err != nil {
				continue
			}
			st.total++
			res := parser.Parse(src)
			u, b := countBad(res.Program)
			if u == 0 && b == 0 && len(res.Diags) == 0 {
				st.clean++
			} else {
				msg := ""
				if len(res.Diags) > 0 {
					msg = res.Diags[0].Error()
				}
				problems = append(problems, tier.Name()+"/"+e.Name()+": untranslated="+itoa(u)+" bad="+itoa(b)+" diags="+itoa(len(res.Diags))+" "+msg)
			}
		}
	}
	names := make([]string, 0, len(byTier))
	for k := range byTier {
		names = append(names, k)
	}
	sort.Strings(names)
	totalClean, totalAll := 0, 0
	for _, n := range names {
		st := byTier[n]
		totalClean += st.clean
		totalAll += st.total
		t.Logf("%-8s %3d/%3d clean", n, st.clean, st.total)
	}
	t.Logf("%-8s %3d/%3d clean", "TOTAL", totalClean, totalAll)
	sort.Strings(problems)
	for _, p := range problems {
		t.Logf("  %s", p)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

package gogen

import (
	"strings"
	"testing"

	"perl2go/internal/ir"
)

// stubTodo is the annotation a refusal leaves behind once its stand-in is in
// the code: Spelled, because the stand-in names the code and the wording
// itself.
func stubTodo() ir.Todo {
	return ir.Todo{
		Code:    "P2G7001",
		Short:   "object method calls are not implemented",
		Message: "Perl resolves a method at run time and Go binds it at compile time",
		Perl:    "$obj->name",
		Spelled: true,
	}
}

// stubExpr builds the call a refused expression becomes.
func stubExpr() ir.Expr {
	x := ir.CallOf(ir.NewIdent("notImplemented[any]", nil), ir.TAny,
		ir.Str(`"P2G7001"`), ir.Str(`"object method calls are not implemented"`))
	t := stubTodo()
	ir.MetaOf(x).Todo = &t
	return x
}

// TestARefusedStatementLeavesTheOnesAfterItReachable is the rule in its
// smallest form. Before this, the statement below emitted a panic and the
// assignment after it was dead code that no reader could ever run.
func TestARefusedStatementLeavesTheOnesAfterItReachable(t *testing.T) {
	todo := stubTodo()
	body := &ir.Block{}
	body.Add(
		&ir.TodoStmt{
			Info: todo,
			Stub: ir.CallOf(ir.NewIdent("notImplementedHere", nil), ir.TVoid,
				ir.Str(`"P2G7001"`), ir.Str(`"object method calls are not implemented"`)),
		},
		&ir.Assign{Op: ":=", LHS: []ir.Expr{ir.NewIdent("n", ir.TInt)}, RHS: []ir.Expr{ir.IntLit("1")}},
	)
	f := &ir.File{Name: "main.go", Package: "main",
		Decls: []ir.Decl{&ir.FuncDecl{Name: "main", Body: body}}}

	for _, mode := range []Mode{Clean, Annotated} {
		src, err := New(mode).File(f)
		if err != nil {
			t.Fatalf("mode %d: %v\n%s", mode, err, src)
		}
		got := string(src)
		if strings.Contains(got, "panic(") {
			t.Errorf("mode %d: a refused statement emitted a panic:\n%s", mode, got)
		}
		if !strings.Contains(got, `notImplementedHere("P2G7001", "object method calls are not implemented")`) {
			t.Errorf("mode %d: the stand-in is missing:\n%s", mode, got)
		}
		if !strings.Contains(got, "n := 1") {
			t.Errorf("mode %d: the statement after the refusal was dropped:\n%s", mode, got)
		}
	}
}

// TestARefusedStatementWithNoEffectIsJustAMarker covers the other half: a
// construct that did nothing at run time gets no stand-in, because there is no
// step to stand in for and a line saying so on stderr would be noise.
func TestARefusedStatementWithNoEffectIsJustAMarker(t *testing.T) {
	body := &ir.Block{}
	body.Add(&ir.TodoStmt{Info: ir.Todo{
		Code:  "P2G7010",
		Short: "a second package in one file is not implemented",
	}})
	f := &ir.File{Name: "main.go", Package: "main",
		Decls: []ir.Decl{&ir.FuncDecl{Name: "main", Body: body}}}

	src, err := New(Clean).File(f)
	if err != nil {
		t.Fatalf("%v\n%s", err, src)
	}
	got := string(src)
	if !strings.Contains(got, "// TODO: a second package in one file is not implemented") {
		t.Errorf("the marker is missing:\n%s", got)
	}
	if strings.Contains(got, "notImplemented") {
		t.Errorf("a construct with no run-time effect got a run-time stand-in:\n%s", got)
	}
}

// TestARefusedExpressionsTodoGoesAboveTheStatement pins the readability half.
// The explanation used to be a block comment in the middle of the expression,
// which pushed the code it belonged to off the right of the screen; a line with
// several refusals in it was unreadable.
func TestARefusedExpressionsTodoGoesAboveTheStatement(t *testing.T) {
	body := &ir.Block{}
	body.Add(&ir.Assign{
		Op:  ":=",
		LHS: []ir.Expr{ir.NewIdent("name", ir.TAny)},
		RHS: []ir.Expr{stubExpr()},
	})
	f := &ir.File{Name: "main.go", Package: "main",
		Decls: []ir.Decl{&ir.FuncDecl{Name: "main", Body: body}}}

	for _, mode := range []Mode{Clean, Annotated} {
		src, err := New(mode).File(f)
		if err != nil {
			t.Fatalf("mode %d: %v\n%s", mode, err, src)
		}
		lines := strings.Split(string(src), "\n")
		code, comment := -1, -1
		for i, l := range lines {
			trimmed := strings.TrimSpace(l)
			if strings.HasPrefix(trimmed, "// TODO:") && comment < 0 {
				comment = i
			}
			if strings.HasPrefix(trimmed, "name := ") {
				code = i
			}
		}
		if comment < 0 || code < 0 {
			t.Fatalf("mode %d: expected a TODO above an assignment:\n%s", mode, src)
		}
		if comment > code {
			t.Errorf("mode %d: the TODO is below the code it explains:\n%s", mode, src)
		}
		if strings.Contains(lines[code], "/*") {
			t.Errorf("mode %d: the TODO was written inside the expression:\n%s", mode, lines[code])
		}
	}
}

// TestARepeatedRefusalIsExplainedOnceAndMarkedEveryTime is the balance between
// the two. One refused construct usually recurs all through a file, so the
// prose is written where it first applies; the stand-in is at every one of
// them, so nothing is hidden.
func TestARepeatedRefusalIsExplainedOnceAndMarkedEveryTime(t *testing.T) {
	body := &ir.Block{}
	for _, name := range []string{"a", "b", "c"} {
		body.Add(&ir.Assign{
			Op:  ":=",
			LHS: []ir.Expr{ir.NewIdent(name, ir.TAny)},
			RHS: []ir.Expr{stubExpr()},
		})
	}
	f := &ir.File{Name: "main.go", Package: "main",
		Decls: []ir.Decl{&ir.FuncDecl{Name: "main", Body: body}}}

	for _, mode := range []Mode{Clean, Annotated} {
		src, err := New(mode).File(f)
		if err != nil {
			t.Fatalf("mode %d: %v\n%s", mode, err, src)
		}
		got := string(src)
		if n := strings.Count(got, "// TODO:"); n != 1 {
			t.Errorf("mode %d: the same refusal is explained %d times, want once:\n%s", mode, n, got)
		}
		if n := strings.Count(got, `notImplemented[any]("P2G7001"`); n != 3 {
			t.Errorf("mode %d: %d of the 3 refusals are marked in the code:\n%s", mode, n, got)
		}
	}
}

// TestARefusalInAnExpressionFragmentIsStillMarked covers the one position with
// no statement above it to hoist onto. The walkthrough quotes single
// expressions, and a refusal must not be the thing that goes unmentioned there.
func TestARefusalInAnExpressionFragmentIsStillMarked(t *testing.T) {
	got := RenderExpr(Clean, stubExpr())
	if !strings.Contains(got, "/* TODO:") {
		t.Errorf("a refused expression rendered on its own lost its marker: %q", got)
	}
}

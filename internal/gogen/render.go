package gogen

import (
	"go/format"
	"strings"

	"perl2go/internal/ir"
)

// RenderStmt renders a single statement on its own, formatted where possible.
// It is what the per-file walkthrough quotes next to the Perl it came from.
//
// The import set of the throwaway emitter is discarded: a fragment shown in a
// document does not contribute to any file's import block.
func RenderStmt(mode Mode, s ir.Stmt) string {
	return formatFragment(renderIsolated(mode, func(e *Emitter) { e.stmt(s) }))
}

// RenderDecl renders a single declaration on its own, formatted where
// possible.
func RenderDecl(mode Mode, d ir.Decl) string {
	return formatFragment(renderIsolated(mode, func(e *Emitter) { e.decl(d) }))
}

// RenderExpr renders a single expression on its own, formatted where possible.
func RenderExpr(mode Mode, x ir.Expr) string {
	src := renderIsolated(mode, func(e *Emitter) { e.w(e.expr(x)) })
	if src == "" {
		return ""
	}
	// go/format takes declarations and statements but not a bare expression,
	// so the expression is formatted as the right-hand side of an assignment
	// and the scaffolding is taken back off.
	const prefix = "_ = "
	out, err := format.Source([]byte(prefix + src + "\n"))
	if err != nil {
		return src
	}
	formatted := strings.TrimRight(string(out), "\n")
	if !strings.HasPrefix(formatted, prefix) {
		return src
	}
	return strings.TrimPrefix(formatted, prefix)
}

// renderIsolated runs one emission against a fresh emitter and returns the
// text. A panic yields whatever had been written so far: these renderers feed
// a document, and a half-rendered line is worth more than a crash.
func renderIsolated(mode Mode, emit func(*Emitter)) (out string) {
	e := New(mode)
	defer func() {
		if r := recover(); r != nil {
			out = strings.TrimRight(e.sb.String(), "\n")
		}
	}()
	emit(e)
	return strings.TrimRight(e.sb.String(), "\n")
}

// formatFragment runs gofmt over a statement or declaration list, returning
// the text unchanged when it does not parse on its own. Fragments legitimately
// fail to parse (a lone case body, say), and the caller wants the text either
// way.
func formatFragment(src string) string {
	if strings.TrimSpace(src) == "" {
		return ""
	}
	out, err := format.Source([]byte(src + "\n"))
	if err != nil {
		return src
	}
	return strings.TrimRight(string(out), "\n")
}

package lower

import (
	"strings"
	"testing"

	"perl2golang/internal/runtime"
)

func TestPackTemplateIssue(t *testing.T) {
	tests := []struct {
		template string
		code     byte // 0 means the whole template is supported
	}{
		{"a3 A8 A10", 0},
		{"A4A2A2", 0},
		{"n N v V C c s S l L q Q", 0},
		{"H4 h2 Z* x", 0},
		{"C*", 0},
		{"a3 w2 N", 'w'},
		{"b8", 'b'},
		{"N< a3", '<'},
		{"l!", '!'},
		{"(a3)4", '('},
		{"%16C*", '%'},
		{"n/a*", '/'},
		{"", 0},
	}
	for _, tt := range tests {
		code, _, ok := packTemplateIssue(tt.template)
		switch {
		case tt.code == 0 && !ok:
			t.Errorf("packTemplateIssue(%q) refused %q, want full support", tt.template, code)
		case tt.code != 0 && ok:
			t.Errorf("packTemplateIssue(%q) passed, want a refusal on %q", tt.template, tt.code)
		case tt.code != 0 && code != tt.code:
			t.Errorf("packTemplateIssue(%q) refused %q, want %q", tt.template, code, tt.code)
		}
	}
}

// The set of codes this package checks templates against and the set the
// emitted interpreter accepts have to be the same set, or a template would be
// approved here and refused at run time.
func TestPackTemplateSupportAgrees(t *testing.T) {
	emitted, err := runtime.Emit([]string{"packItems"}, "main")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(emitted), `"`+packSupported+`"`) {
		t.Errorf("the emitted packItems does not accept exactly %q; keep packSupported and the helper's set identical", packSupported)
	}
}

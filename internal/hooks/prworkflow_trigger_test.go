package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gt-2h6: `tap guard pr-workflow` blocks unconditionally in agent context and
// never inspects the command it was invoked for. Any harness template that
// triggers it on `git push` therefore blocks every push in every rig --
// including the refinery's own merge push -- so no merge request can ever land.
//
// The Claude templates are the reference implementation. These tests pin the pi
// and omp templates to that reference so the extra trigger cannot come back.

// referenceTriggers is the set of operations the Claude templates police:
// `gh pr create`, `git checkout -b`, `git switch -c`. Notably NOT `git push` --
// Gas Town workers push directly to main, so a push is the endorsed operation.
var referenceTriggers = []string{
	"gh pr create",
	"git checkout -b",
	"git switch -c",
}

func readTemplate(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("templates", rel))
	if err != nil {
		t.Fatalf("read template %s: %v", rel, err)
	}
	return string(b)
}

// claudePRWorkflowMatchers extracts the Bash matchers that invoke pr-workflow.
func claudePRWorkflowMatchers(t *testing.T, file string) []string {
	t.Helper()
	raw := readTemplate(t, filepath.Join("claude", file))
	var cfg struct {
		Hooks struct {
			PreToolUse []struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"PreToolUse"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	var out []string
	for _, h := range cfg.Hooks.PreToolUse {
		for _, inner := range h.Hooks {
			if strings.Contains(inner.Command, "tap guard pr-workflow") {
				m := strings.TrimSuffix(strings.TrimPrefix(h.Matcher, "Bash("), "*)")
				out = append(out, m)
			}
		}
	}
	return out
}

func TestClaudeTemplatesAreTheReferenceSet(t *testing.T) {
	got := claudePRWorkflowMatchers(t, "settings-autonomous.json")
	if len(got) == 0 {
		t.Fatal("no pr-workflow matchers in claude settings-autonomous.json")
	}
	for _, want := range referenceTriggers {
		found := false
		for _, g := range got {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Errorf("reference is missing %q; got %v", want, got)
		}
	}
	for _, g := range got {
		if g == "git push" {
			t.Error("reference must not police `git push`; pushes are the endorsed path")
		}
	}
}

func TestPiTemplateDoesNotBlockPushes(t *testing.T) {
	src := readTemplate(t, filepath.Join("pi", "gastown-hooks.js"))

	// The trigger must not fire on a plain push.
	if strings.Contains(src, `cmd.includes("git push")`) {
		t.Error("pi template triggers pr-workflow on `git push`; " +
			"that blocks every agent push including the refinery's merge push")
	}
	for _, want := range referenceTriggers {
		if !strings.Contains(src, `cmd.includes("`+want+`")`) {
			t.Errorf("pi template missing trigger for %q", want)
		}
	}
}

func TestOmpTemplateDoesNotBlockPushes(t *testing.T) {
	src := readTemplate(t, filepath.Join("omp", "gastown-hook.ts"))

	if strings.Contains(src, `cmd.includes("git push")`) {
		t.Error("omp template triggers pr-workflow on `git push`; " +
			"that blocks every agent push including the refinery's merge push")
	}
	for _, want := range referenceTriggers {
		if !strings.Contains(src, `cmd.includes("`+want+`")`) {
			t.Errorf("omp template missing trigger for %q", want)
		}
	}
}

// The explanatory comment is load-bearing: the next person to "tidy up" an
// unmatched trigger will otherwise re-add `git push` believing it was omitted
// by accident.
func TestTemplatesExplainWhyPushIsNotATrigger(t *testing.T) {
	for _, f := range []string{
		filepath.Join("pi", "gastown-hooks.js"),
		filepath.Join("omp", "gastown-hook.ts"),
	} {
		src := readTemplate(t, f)
		if !strings.Contains(src, "deliberately NOT here") {
			t.Errorf("%s: missing rationale for the absent `git push` trigger", f)
		}
	}
}

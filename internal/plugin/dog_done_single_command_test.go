package plugin

import (
	"strings"
	"testing"
)

// A dog used to get two commands at the end of its turn: record the run, then
// `gt dog done`. The deacon observed all four dogs hung after their response
// was truncated, sitting at ~CH51% rather than exhausted, which means the
// truncation landed before the state transition and the dog stayed "working"
// forever (hq-wisp-82s43). The same truncation had cut the Deacon's own pane,
// so this is not dog-specific - it is any long turn that runs out of budget.
//
// The fix is to make the post-work step a single command. These pin that the
// pane instructs one command, not two, so the step cannot drift back.

func TestFormatMailBodyInstructsSingleDoneCommand(t *testing.T) {
	p := &Plugin{Name: "compactor-dog", Description: "compacts beads"}
	body := p.FormatMailBody()

	if !strings.Contains(body, "gt dog done --plugin compactor-dog") {
		t.Errorf("pane must tell the dog to pass --plugin to gt dog done:\n%s", body)
	}
	if strings.Contains(body, "gt plugin record-run") {
		t.Errorf("pane still prescribes the separate record-run step; that is the step that gets truncated:\n%s", body)
	}
	if n := strings.Count(body, "gt dog done"); n != 1 {
		t.Errorf("expected exactly one 'gt dog done' in the pane, found %d", n)
	}
}

func TestFormatMailBodyScriptVariantInstructsSingleDoneCommand(t *testing.T) {
	p := &Plugin{
		Name:        "dolt-log-rotate",
		Description: "rotates logs",
		Path:        "/path/to/plugin",
		HasRunScript: true,
	}
	body := p.FormatMailBody()

	if !strings.Contains(body, "gt dog done --plugin dolt-log-rotate") {
		t.Errorf("script pane must fold recording into gt dog done:\n%s", body)
	}
	if strings.Contains(body, "gt plugin record-run") {
		t.Errorf("script pane still prescribes the separate record-run step:\n%s", body)
	}
}

// The old wording told the dog to finish "even if recording fails", which
// implied recording was a step that could fail and block. The new wording has
// to keep that guarantee for the case where the plugin itself failed.
func TestFormatMailBodyKeepsFinishEvenIfPluginFailed(t *testing.T) {
	body := (&Plugin{Name: "x", Description: "y"}).FormatMailBody()
	if !strings.Contains(body, "even if the plugin itself failed") {
		t.Errorf("pane dropped the guarantee that the dog finishes regardless of plugin outcome:\n%s", body)
	}
	if !strings.Contains(body, "truncated") {
		t.Errorf("pane no longer warns about truncation, which is the observed failure mode:\n%s", body)
	}
}
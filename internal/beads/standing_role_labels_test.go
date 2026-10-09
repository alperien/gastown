package beads

import "testing"

// A rig's witness/refinery/crew-user bead is standing: it exists for as long as the rig does, so
// its updated_at only moves when something touches it. The wisp reaper closes on staleness alone,
// which makes such a bead guaranteed to be closed eventually and silently wrong when it is -- the
// role keeps running while its tracking bead reads closed (hq-60a5).
func TestIsStandingRoleID(t *testing.T) {
	standing := []string{
		"xi-xianyumcp-witness",
		"xi-xianyumcp-refinery",
		"xi-xianyumcp-crew-user",
		// the rig part can itself contain a dash, so the role is matched at the end, not by position
		"gtw-gastown_webui-witness",
		"gtw-gastown_webui-refinery",
		"be-beads-witness",
	}
	for _, id := range standing {
		if !IsStandingRoleID(id) {
			t.Errorf("%s is a standing rig role and must be recognised", id)
		}
	}

	ephemeral := []string{
		"gt-gastown-polecat-Toast",
		"gt-gastown-polecap-crash",
		"xi-wisp-pcm",
		// a polecat that happens to be named witness is still a polecat
		"gt-gastown-polecat-witness",
		"gt-gastown-polecat-refinery",
		"",
	}
	for _, id := range ephemeral {
		if IsStandingRoleID(id) {
			t.Errorf("%s is not a standing rig role and must not be given the exemption", id)
		}
	}
}

func TestStandingRoleLabels(t *testing.T) {
	got := standingRoleLabels("xi-xianyumcp-witness")
	for _, want := range []string{"gt:role", "gt:rig"} {
		found := false
		for _, l := range got {
			if l == want {
				found = true
			}
		}
		if !found {
			t.Errorf("standing role must carry %s, got %v", want, got)
		}
	}

	if n := len(standingRoleLabels("gt-gastown-polecat-Toast")); n != 0 {
		t.Errorf("an ephemeral polecat gets no protection labels, got %d", n)
	}
}

// A polecat bead that goes through reset must come back still carrying its labels. This function
// previously kept only gt:agent and safety_stop:*, silently stripping the reaper exemption from any
// role bead that was reset -- so the bead became closable by age again with nothing recording why.
func TestLabelsForAgentBeadReusePreservesProtection(t *testing.T) {
	existing := []string{"gt:agent", "gt:role", "gt:rig", "safety_stop:crash", "random-label"}

	got := labelsForAgentBeadReuse(existing)

	want := map[string]bool{"gt:agent": true, "gt:role": true, "gt:rig": true, "safety_stop:crash": true}
	for _, l := range got {
		delete(want, l)
	}
	if len(want) != 0 {
		t.Errorf("protection labels lost on reuse, still missing %v (got %v)", want, got)
	}

	for _, l := range got {
		if l == "random-label" {
			t.Error("unrelated labels must still be dropped on reuse")
		}
	}
}
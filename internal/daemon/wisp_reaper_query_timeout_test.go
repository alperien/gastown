package daemon

import (
	"testing"
	"time"
)

// The reaper used a hardcoded 10s per-query budget at all four OpenDB call
// sites. On a town-sized hq the anti-join and age queries run longer than
// that, so the reaper aborted mid-cycle and reported 0 reaps instead of
// failing loudly (hq-ddz). These pin the default, the override, and the two
// ways a bad override must not silently restore a budget that cannot work.

func TestWispReaperQueryTimeoutDefaultsWhenUnset(t *testing.T) {
	cases := []struct {
		name   string
		config *DaemonPatrolConfig
	}{
		{"nil config", nil},
		{"nil patrols", &DaemonPatrolConfig{}},
		{"nil wisp_reaper", &DaemonPatrolConfig{Patrols: &PatrolsConfig{}}},
		{"empty string", &DaemonPatrolConfig{Patrols: &PatrolsConfig{
			WispReaper: &WispReaperConfig{QueryTimeoutStr: ""},
		}}},
	}
	for _, tc := range cases {
		if got := wispReaperQueryTimeout(tc.config); got != wispReaperDefaultQueryTimeout {
			t.Errorf("%s: got %v, want %v", tc.name, got, wispReaperDefaultQueryTimeout)
		}
	}
}

func TestWispReaperQueryTimeoutHonoursOverride(t *testing.T) {
	cfg := &DaemonPatrolConfig{Patrols: &PatrolsConfig{
		WispReaper: &WispReaperConfig{QueryTimeoutStr: "120s"},
	}}
	if got := wispReaperQueryTimeout(cfg); got != 120*time.Second {
		t.Errorf("got %v, want 120s", got)
	}
}

// An unparseable or non-positive value must fall back to the default. Falling
// back to zero or to a negative duration would hand the driver a timeout that
// fails every query, which is worse than the bug being fixed.
func TestWispReaperQueryTimeoutRejectsBadOverride(t *testing.T) {
	for _, raw := range []string{"not-a-duration", "0s", "-5s", "0"} {
		cfg := &DaemonPatrolConfig{Patrols: &PatrolsConfig{
			WispReaper: &WispReaperConfig{QueryTimeoutStr: raw},
		}}
		if got := wispReaperQueryTimeout(cfg); got != wispReaperDefaultQueryTimeout {
			t.Errorf("%q: got %v, want the default %v", raw, got, wispReaperDefaultQueryTimeout)
		}
	}
}

// The default has to exceed the 10s that caused hq-ddz. Pin it so a future
// edit cannot quietly lower it back under the cost of a real query.
func TestWispReaperDefaultQueryTimeoutExceedsTheOldHardcodedTen(t *testing.T) {
	if wispReaperDefaultQueryTimeout <= 10*time.Second {
		t.Errorf("default is %v; it must be greater than the 10s that broke hq reaping",
			wispReaperDefaultQueryTimeout)
	}
}

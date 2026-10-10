package daemon

import (
	"testing"
	"time"
)

// The dog dispatch loop walks every eligible plugin and hands each one to the
// next idle dog it finds, with no gap between starts. When the kennel frees up
// together -- daemon restart, orphan cleanup, re-enabling patrols -- all four
// sessions start inside ~90-110s, every turn runs long, and every turn
// truncates before reaching `gt dog done`. Measured twice:
//   alpha/bravo/charlie/delta at 11:47:25 / 11:47:55 / 11:48:27 / 11:48:58
//   alpha/bravo/charlie/delta at 12:03:30 / 12:04:03 / 12:04:47 / 12:05:19
// A provider POST returned 200 with finish_reason "stop" in 2.45s during the
// second burst, so this is a turn-concurrency limit, not an upstream outage.
//
// These pin the rate limit. Without them the loop silently goes back to
// bursting the pack.

func TestDogDispatchRateLimitedAllowsFirstDispatch(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	if dogDispatchRateLimited(time.Time{}, now) {
		t.Error("the first dispatch after startup must not be rate limited")
	}
}

// The observed failing gaps: the whole kennel started inside 110s, and the
// individual starts were ~30-45s apart. Both must be refused.
func TestDogDispatchRateLimitedRefusesObservedBurstGaps(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	for _, gap := range []time.Duration{
		30 * time.Second,  // the "beat" I originally recommended between calls
		45 * time.Second,
		90 * time.Second,  // the width that produced the first burst
		109 * time.Second, // the width that produced the second burst
		119 * time.Second,
	} {
		if !dogDispatchRateLimited(now.Add(-gap), now) {
			t.Errorf("gap of %s must be refused, it is inside the burst window", gap)
		}
	}
}

func TestDogDispatchRateLimitedAllowsGapAtOrAboveMinimum(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	for _, gap := range []time.Duration{
		minDogDispatchInterval,
		minDogDispatchInterval + time.Second,
		3 * time.Minute, // the daemon heartbeat default
	} {
		if dogDispatchRateLimited(now.Add(-gap), now) {
			t.Errorf("gap of %s must be allowed, it is at or above the %s minimum",
				gap, minDogDispatchInterval)
		}
	}
}

// The heartbeat runs every 3 minutes by default, so the whole kennel drains in
// ~9 minutes rather than ~90 seconds. If this ever regresses to something at
// or below the burst window, the rate limit stops being the limiting factor
// and the loop bursts again.
func TestDogDispatchIntervalExceedsHeartbeatCadence(t *testing.T) {
	if minDogDispatchInterval > 3*time.Minute {
		t.Errorf("minimum dispatch interval is %s; with a 3m heartbeat a larger "+
			"value cannot be honoured and dispatch would silently fall back to one "+
			"per heartbeat anyway", minDogDispatchInterval)
	}
}
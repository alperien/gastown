package daemon

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	beadsdk "github.com/steveyegge/beads"
)

// gt-b6r: hasActiveWork() gave the in_progress scan 5 seconds across every rig
// DB and treated ANY query error as "work present". A Dolt query merely slower
// than 5s therefore expired the context and read as work in flight, which made
// a healthy Deacon -- mid patrol cycle, including the mandatory handoff mail --
// look idle and drew spurious unresponsive escalations.
//
// The distinction that matters: a hard error means the store could not be
// reached and we genuinely know nothing, while a deadline means we asked a
// question the server did not finish answering. Neither is evidence of work,
// but only the second was silently masquerading as "busy" on a healthy town.

type timeoutStorage struct {
	beadsdk.Storage
	delay time.Duration
}

func (s *timeoutStorage) SearchIssues(ctx context.Context, _ string, _ beadsdk.IssueFilter) ([]*beadsdk.Issue, error) {
	select {
	case <-time.After(s.delay):
		return nil, nil // answered, and there was nothing
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type errStorage struct {
	beadsdk.Storage
	err error
}

func (s *errStorage) SearchIssues(context.Context, string, beadsdk.IssueFilter) ([]*beadsdk.Issue, error) {
	return nil, s.err
}

type workStorage struct{ beadsdk.Storage }

func (workStorage) SearchIssues(context.Context, string, beadsdk.IssueFilter) ([]*beadsdk.Issue, error) {
	return []*beadsdk.Issue{{ID: "hq-1"}}, nil
}

func TestHasActiveWorkTimeoutIsNotEvidenceOfWork(t *testing.T) {
	d := newTestDaemonWithStores(t, "", map[string]beadsdk.Storage{
		"hq": &timeoutStorage{delay: 2 * time.Second},
	})
	// Would previously have returned true ("conservative: assume work").
	if d.hasActiveWork() {
		t.Error("a query that timed out is not evidence of work; this suppresses " +
			"the idle guard and draws false unresponsive reports")
	}
}

func TestHasActiveWorkStillReportsRealWork(t *testing.T) {
	d := newTestDaemonWithStores(t, "", map[string]beadsdk.Storage{"hq": workStorage{}})
	if !d.hasActiveWork() {
		t.Error("an answered query returning a row is real work and must still report true")
	}
}

// A hard error is a different claim from a slow answer, and the existing
// guard test pins the conservative behaviour for it. Keep that.
func TestHasActiveWorkHardErrorStaysConservative(t *testing.T) {
	d := newTestDaemonWithStores(t, "", map[string]beadsdk.Storage{
		"hq": &errStorage{err: fmt.Errorf("db offline")},
	})
	if !d.hasActiveWork() {
		t.Error("a hard store error must stay conservative: we know nothing, so do not suppress Boot")
	}
}

// One slow store must not mask a store that actually answered with work.
func TestHasActiveWorkSlowStoreDoesNotMaskRealWork(t *testing.T) {
	d := newTestDaemonWithStores(t, "", map[string]beadsdk.Storage{
		"slow":  &timeoutStorage{delay: 2 * time.Second},
		"other": workStorage{},
	})
	if !d.hasActiveWork() {
		t.Error("a second store returned real work; that must not be masked by an unrelated timeout")
	}
}

// Pin the budget above the value that caused gt-b6r.
func TestHasActiveWorkTimeoutExceedsTheOriginalFive(t *testing.T) {
	if hasActiveWorkQueryTimeout <= 5*time.Second {
		t.Errorf("timeout is %s; it must exceed the 5s that produced the false reports",
			hasActiveWorkQueryTimeout)
	}
}

var _ = errors.Is // keep the errors import meaningful if the impl changes
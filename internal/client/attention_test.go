package client

import (
	"testing"
	"time"

	"github.com/weill-labs/amux/internal/config"
	"github.com/weill-labs/amux/internal/proto"
	"github.com/weill-labs/amux/internal/render"
)

func attentionSnapshot(active uint32, idle map[uint32]bool) *rendererSnapshot {
	s := newRendererSnapshot(80, 24, 100)
	s.activePaneID = active
	for id, isIdle := range idle {
		s.paneInfo[id] = proto.PaneSnapshot{ID: id, Idle: isIdle}
	}
	return s
}

func newTestTracker(now *time.Time) *attentionTracker {
	t := newAttentionTracker()
	t.now = func() time.Time { return *now }
	settings, _ := config.ResolveAttention(config.ThemeConfig{})
	settings.DoneFlashDurationMs = 1000
	settings.DoneFlashPulses = 2 // 500ms period, 250ms on / 250ms off
	t.configure(settings)
	return t
}

func TestAttentionBusyToIdleFlashesThenGlows(t *testing.T) {
	t.Parallel()
	now := time.Unix(1000, 0)
	tr := newTestTracker(&now)

	prev := attentionSnapshot(1, map[uint32]bool{1: false, 2: false})
	next := attentionSnapshot(1, map[uint32]bool{1: false, 2: true})
	if !tr.observeLayout(prev, next) {
		t.Fatal("expected state change on busy->idle")
	}

	// t=0: on phase → flash colours.
	att := tr.attentionFor(2, false)
	if att.BorderHex != config.DefaultDoneFlashColor || att.StatusBgHex != config.DefaultDoneFlashBg {
		t.Fatalf("expected flash tint at t=0, got %+v", att)
	}
	deadline, ok := tr.nextDeadline()
	if !ok || deadline.Sub(now) != 250*time.Millisecond {
		t.Fatalf("expected next edge at +250ms, got %v ok=%v", deadline.Sub(now), ok)
	}

	// t=300ms: off phase → plain.
	now = now.Add(300 * time.Millisecond)
	if att := tr.attentionFor(2, false); att != attentionPlain() {
		t.Fatalf("expected plain during off phase, got %+v", att)
	}

	// t=600ms: second pulse on.
	now = now.Add(300 * time.Millisecond)
	if att := tr.attentionFor(2, false); att.BorderHex != config.DefaultDoneFlashColor {
		t.Fatalf("expected second pulse on, got %+v", att)
	}

	// t=1.2s: flash over → steady needs-input glow, no more deadlines.
	now = now.Add(600 * time.Millisecond)
	att = tr.attentionFor(2, false)
	if att.BorderHex != config.DefaultNeedsInputColor || att.StatusBgHex != config.DefaultNeedsInputBg {
		t.Fatalf("expected glow after flash, got %+v", att)
	}
	if _, ok := tr.nextDeadline(); ok {
		t.Fatal("expected no deadline while glowing")
	}

	// Focusing the pane acknowledges it.
	focused := attentionSnapshot(2, map[uint32]bool{1: false, 2: true})
	tr.observeLayout(next, focused)
	if att := tr.attentionFor(2, true); att != attentionPlain() {
		t.Fatalf("focused pane must never be tinted, got %+v", att)
	}
	if att := tr.attentionFor(2, false); att != attentionPlain() {
		t.Fatalf("expected acknowledged pane to be plain, got %+v", att)
	}
}

func TestAttentionFocusedPaneDoesNotFlash(t *testing.T) {
	t.Parallel()
	now := time.Unix(1000, 0)
	tr := newTestTracker(&now)
	prev := attentionSnapshot(1, map[uint32]bool{1: false})
	next := attentionSnapshot(1, map[uint32]bool{1: true})
	tr.observeLayout(prev, next)
	if att := tr.attentionFor(1, false); att != attentionPlain() {
		t.Fatalf("focused pane going idle must not flash, got %+v", att)
	}
}

func TestAttentionAlreadyIdlePanesStayQuiet(t *testing.T) {
	t.Parallel()
	now := time.Unix(1000, 0)
	tr := newTestTracker(&now)
	// First snapshot: pane 2 already idle. No prior state → no flash.
	tr.observeLayout(nil, attentionSnapshot(1, map[uint32]bool{1: false, 2: true}))
	if att := tr.attentionFor(2, false); att != attentionPlain() {
		t.Fatalf("already-idle pane must not flash on first snapshot, got %+v", att)
	}
}

func TestAttentionBusyAgainClearsAndKeystrokeAcks(t *testing.T) {
	t.Parallel()
	now := time.Unix(1000, 0)
	tr := newTestTracker(&now)
	prev := attentionSnapshot(1, map[uint32]bool{1: false, 2: false})
	idle := attentionSnapshot(1, map[uint32]bool{1: false, 2: true})
	tr.observeLayout(prev, idle)
	if att := tr.attentionFor(2, false); att == attentionPlain() {
		t.Fatal("expected flash")
	}
	busy := attentionSnapshot(1, map[uint32]bool{1: false, 2: false})
	tr.observeLayout(idle, busy)
	if att := tr.attentionFor(2, false); att != attentionPlain() {
		t.Fatalf("busy again should clear attention, got %+v", att)
	}

	tr.observeLayout(busy, idle)
	if !tr.acknowledge(2) {
		t.Fatal("expected acknowledge to clear state")
	}
	if tr.acknowledge(2) {
		t.Fatal("second acknowledge should be a no-op")
	}
}

func TestAttentionDisabledFlashStillGlows(t *testing.T) {
	t.Parallel()
	now := time.Unix(1000, 0)
	tr := newTestTracker(&now)
	settings := tr.settings
	settings.DoneFlash = false
	tr.configure(settings)
	prev := attentionSnapshot(1, map[uint32]bool{2: false})
	next := attentionSnapshot(1, map[uint32]bool{2: true})
	tr.observeLayout(prev, next)
	att := tr.attentionFor(2, false)
	if att.BorderHex != config.DefaultNeedsInputColor {
		t.Fatalf("expected immediate glow with flash disabled, got %+v", att)
	}
	if _, ok := tr.nextDeadline(); ok {
		t.Fatal("no deadline expected with flash disabled")
	}
}

func TestAttentionAllDisabledIsInert(t *testing.T) {
	t.Parallel()
	now := time.Unix(1000, 0)
	tr := newTestTracker(&now)
	tr.configure(config.AttentionSettings{})
	prev := attentionSnapshot(1, map[uint32]bool{2: false})
	next := attentionSnapshot(1, map[uint32]bool{2: true})
	if tr.observeLayout(prev, next) {
		t.Fatal("disabled tracker must not change state")
	}
	if att := tr.attentionFor(2, false); att != attentionPlain() {
		t.Fatalf("expected plain, got %+v", att)
	}
}

func attentionPlain() render.PaneAttention { return render.PaneAttention{} }

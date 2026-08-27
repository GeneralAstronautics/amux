package client

import (
	"sync"
	"time"

	"github.com/weill-labs/amux/internal/config"
	"github.com/weill-labs/amux/internal/render"
)

// attentionTracker owns the client-side "done-flash" and "needs-input glow"
// state. It observes idle/busy transitions from pushed layout snapshots and
// decides per pane, per frame, which border/title tint (if any) to draw.
//
// Semantics:
//   - A pane whose agent goes busy -> idle while NOT focused starts a flash
//     (DoneFlashPulses pulses over DoneFlashDurationMs) and becomes
//     "unacknowledged".
//   - After the flash, an unacknowledged pane shows the steady needs-input glow
//     until it is focused, typed into, or its agent goes busy again.
//   - The focused pane never flashes or glows.
type attentionTracker struct {
	mu       sync.Mutex
	settings config.AttentionSettings
	panes    map[uint32]*paneAttentionState
	now      func() time.Time
}

type paneAttentionState struct {
	flashStart time.Time // zero when not flashing
	unacked    bool      // finished while unfocused and not yet visited
}

func newAttentionTracker() *attentionTracker {
	settings, _ := config.ResolveAttention(config.ThemeConfig{})
	return &attentionTracker{
		settings: settings,
		panes:    make(map[uint32]*paneAttentionState),
		now:      time.Now,
	}
}

func (t *attentionTracker) configure(settings config.AttentionSettings) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.settings = settings
	if !settings.DoneFlash && !settings.NeedsInputGlow {
		t.panes = make(map[uint32]*paneAttentionState)
	}
}

func (t *attentionTracker) enabled() bool {
	return t.settings.DoneFlash || t.settings.NeedsInputGlow
}

func (t *attentionTracker) flashDuration() time.Duration {
	return time.Duration(t.settings.DoneFlashDurationMs) * time.Millisecond
}

func (t *attentionTracker) pulsePeriod() time.Duration {
	pulses := t.settings.DoneFlashPulses
	if pulses < 1 {
		pulses = 1
	}
	return t.flashDuration() / time.Duration(pulses)
}

// observeLayout compares consecutive snapshots and updates per-pane state.
// It returns true when the visible attention state may have changed.
func (t *attentionTracker) observeLayout(prev, next *rendererSnapshot) bool {
	if t == nil || next == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.enabled() {
		return false
	}
	now := t.now()
	changed := false

	// Drop state for panes that no longer exist.
	for id := range t.panes {
		if _, ok := next.paneInfo[id]; !ok {
			delete(t.panes, id)
			changed = true
		}
	}

	for id, ps := range next.paneInfo {
		var prevIdle, hadPrev bool
		if prev != nil {
			if p, ok := prev.paneInfo[id]; ok {
				prevIdle, hadPrev = p.Idle, true
			}
		}
		st := t.panes[id]

		if id == next.activePaneID {
			// Focus acknowledges everything.
			if st != nil {
				delete(t.panes, id)
				changed = true
			}
			continue
		}

		if !ps.Idle {
			// Busy: nothing to show; clear any stale state.
			if st != nil {
				delete(t.panes, id)
				changed = true
			}
			continue
		}

		// Idle now. Only a busy -> idle transition (for a pane we saw busy
		// before) starts a flash; panes that were already idle stay quiet.
		if hadPrev && !prevIdle && st == nil {
			st = &paneAttentionState{unacked: true}
			if t.settings.DoneFlash {
				st.flashStart = now
			}
			t.panes[id] = st
			changed = true
		}
	}
	return changed
}

// acknowledge clears attention for a pane (focus / keystroke). Returns true
// when state changed.
func (t *attentionTracker) acknowledge(paneID uint32) bool {
	if t == nil || paneID == 0 {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.panes[paneID]; !ok {
		return false
	}
	delete(t.panes, paneID)
	return true
}

// attentionFor returns the tint to draw for paneID at the current time.
func (t *attentionTracker) attentionFor(paneID uint32, isActive bool) render.PaneAttention {
	if t == nil || isActive {
		return render.PaneAttention{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	st, ok := t.panes[paneID]
	if !ok {
		return render.PaneAttention{}
	}
	now := t.now()
	if t.settings.DoneFlash && !st.flashStart.IsZero() {
		elapsed := now.Sub(st.flashStart)
		if elapsed < t.flashDuration() {
			period := t.pulsePeriod()
			if period > 0 && elapsed%period < period/2 {
				return render.PaneAttention{
					BorderHex:   t.settings.DoneFlashColor,
					StatusBgHex: t.settings.DoneFlashBg,
				}
			}
			// Off phase of a pulse: draw the plain (non-glow) look so the
			// pulse is visible against the steady glow that follows.
			return render.PaneAttention{}
		}
		st.flashStart = time.Time{}
	}
	if t.settings.NeedsInputGlow && st.unacked {
		return render.PaneAttention{
			BorderHex:   t.settings.NeedsInputColor,
			StatusBgHex: t.settings.NeedsInputBg,
		}
	}
	return render.PaneAttention{}
}

// nextDeadline reports when the next visual change is due (a pulse edge or
// the end of a flash). ok is false when nothing is animating.
func (t *attentionTracker) nextDeadline() (time.Time, bool) {
	if t == nil {
		return time.Time{}, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.settings.DoneFlash {
		return time.Time{}, false
	}
	now := t.now()
	var best time.Time
	found := false
	period := t.pulsePeriod()
	half := period / 2
	if half <= 0 {
		half = time.Millisecond
	}
	for _, st := range t.panes {
		if st.flashStart.IsZero() {
			continue
		}
		end := st.flashStart.Add(t.flashDuration())
		if !now.Before(end) {
			continue
		}
		elapsed := now.Sub(st.flashStart)
		// Next half-period boundary.
		next := st.flashStart.Add((elapsed/half + 1) * half)
		if next.After(end) {
			next = end
		}
		if !found || next.Before(best) {
			best = next
			found = true
		}
	}
	return best, found
}

// --- ClientRenderer / Renderer plumbing ---

// ConfigureAttention installs done-flash / needs-input settings.
func (cr *ClientRenderer) ConfigureAttention(settings config.AttentionSettings) {
	if cr == nil || cr.renderer == nil {
		return
	}
	cr.renderer.attention.configure(settings)
}

// AcknowledgeActivePane clears attention state for the focused pane (called on
// local keystrokes). Returns true if a redraw is needed.
func (cr *ClientRenderer) AcknowledgeActivePane() bool {
	if cr == nil || cr.renderer == nil {
		return false
	}
	if cr.renderer.attention.acknowledge(cr.ActivePaneID()) {
		cr.RequestFullRedraw()
		return true
	}
	return false
}

func (cr *ClientRenderer) attentionDeadline() (time.Time, bool) {
	if cr == nil || cr.renderer == nil {
		return time.Time{}, false
	}
	return cr.renderer.attention.nextDeadline()
}

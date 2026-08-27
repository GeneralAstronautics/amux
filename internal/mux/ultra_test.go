package mux

import (
	"testing"
)

func ultraTestWindow(t *testing.T, n int) *Window {
	t.Helper()
	w := NewWindow(fakePaneID(1), 120, 40)
	for i := 2; i <= n; i++ {
		if _, err := w.SplitRoot(SplitVertical, fakePaneID(uint32(i))); err != nil {
			t.Fatalf("split %d: %v", i, err)
		}
	}
	return w
}

func leafByID(t *testing.T, w *Window, id uint32) *LayoutCell {
	t.Helper()
	cell := w.Root.FindPane(id)
	if cell == nil {
		t.Fatalf("pane %d not in visible tree", id)
	}
	return cell
}

func TestUltraEnableAssignsLeadAndSlots(t *testing.T) {
	t.Parallel()
	w := ultraTestWindow(t, 6)
	if err := w.SetLead(1); err != nil {
		t.Fatal(err)
	}
	if err := w.EnableUltra(2, 3, true); err != nil {
		t.Fatal(err)
	}
	if !w.IsUltra() || w.Ultra.PageCount() != 1 {
		t.Fatalf("expected single page ultra, got %+v", w.Ultra)
	}
	lead := leafByID(t, w, 1)
	if lead.X != 0 || lead.Y != 0 || !lead.LeadSlot {
		t.Fatalf("lead not top-left: %+v", lead)
	}
	// All six panes visible; 2 rows x 3 cols.
	leaves := 0
	w.Root.Walk(func(c *LayoutCell) { leaves++ })
	if leaves != 6 {
		t.Fatalf("expected 6 leaves, got %d", leaves)
	}
	if got := w.Ultra.SlotIDs(); len(got) != 5 || got[0] != 2 || got[4] != 6 {
		t.Fatalf("slots = %v", got)
	}
	// Equal-ish cells.
	c2, c3 := leafByID(t, w, 2), leafByID(t, w, 3)
	if c2.Y != 0 || c3.Y != 0 || c2.X <= lead.X || c3.X <= c2.X {
		t.Fatalf("row 0 order wrong: lead=%+v c2=%+v c3=%+v", lead, c2, c3)
	}
	if c4 := leafByID(t, w, 4); c4.Y == 0 || c4.X != 0 {
		t.Fatalf("pane 4 should start row 1 at x=0: %+v", c4)
	}
	if len(w.Panes()) != 6 || w.PaneCount() != 6 {
		t.Fatalf("Panes()=%d PaneCount=%d", len(w.Panes()), w.PaneCount())
	}
}

func TestUltraCloseLeavesEmptySlotAndNewPaneTakesLowest(t *testing.T) {
	t.Parallel()
	w := ultraTestWindow(t, 4)
	_ = w.SetLead(1)
	if err := w.EnableUltra(2, 3, true); err != nil {
		t.Fatal(err)
	}
	if err := w.ClosePane(3); err != nil {
		t.Fatal(err)
	}
	if got := w.Ultra.SlotIDs(); len(got) != 3 || got[0] != 2 || got[1] != 0 || got[2] != 4 {
		t.Fatalf("slots after close = %v", got)
	}
	// Empty leaf exists at slot 2 with its slot number.
	var empty *LayoutCell
	w.Root.Walk(func(c *LayoutCell) {
		if c.Pane == nil && c.Slot == 2 {
			empty = c
		}
	})
	if empty == nil {
		t.Fatal("expected an empty leaf tagged slot 2")
	}
	if w.HasPane(3) {
		t.Fatal("closed pane still reported")
	}
	// Pane 4 kept its slot (stable), new pane fills slot 2.
	if _, err := w.SplitRoot(SplitVertical, fakePaneID(9)); err != nil {
		t.Fatal(err)
	}
	if got := w.Ultra.SlotIDs(); got[1] != 9 || got[2] != 4 {
		t.Fatalf("slots after spawn = %v", got)
	}
	if w.ActivePane == nil || w.ActivePane.ID != 9 {
		t.Fatalf("new pane should be active, got %v", w.ActivePane)
	}
}

func TestUltraKeepFocusSpawnDoesNotChangeActive(t *testing.T) {
	t.Parallel()
	w := ultraTestWindow(t, 2)
	_ = w.SetLead(1)
	_ = w.EnableUltra(2, 3, true)
	w.FocusPane(w.Panes()[0])
	if _, err := w.SplitRootWithOptions(SplitVertical, fakePaneID(7), SplitOptions{KeepFocus: true}); err != nil {
		t.Fatal(err)
	}
	if w.ActivePane.ID != 1 {
		t.Fatalf("active changed to %d", w.ActivePane.ID)
	}
}

func TestUltraPagination(t *testing.T) {
	t.Parallel()
	w := ultraTestWindow(t, 9) // lead + 8 agents -> 2 pages of 5
	_ = w.SetLead(1)
	w.FocusPane(w.Panes()[0])
	if err := w.EnableUltra(2, 3, true); err != nil {
		t.Fatal(err)
	}
	u := w.Ultra
	if u.PageCount() != 2 {
		t.Fatalf("pages = %d", u.PageCount())
	}
	if !w.HasPane(9) || w.Root.FindPane(9) != nil || !w.UltraPaneHidden(9) {
		t.Fatalf("pane 9 should be hidden but owned")
	}
	if cols, rows, ok := w.PaneContentSize(9); !ok || cols <= 0 || rows <= 0 {
		t.Fatalf("hidden pane size = %d,%d,%v", cols, rows, ok)
	}
	if len(w.Panes()) != 9 {
		t.Fatalf("Panes() should include hidden panes, got %d", len(w.Panes()))
	}
	if err := w.UltraStepPage(1); err != nil {
		t.Fatal(err)
	}
	if u.Page != 1 || w.Root.FindPane(9) == nil || w.Root.FindPane(2) != nil {
		t.Fatalf("page 2 wrong: page=%d", u.Page)
	}
	// Lead visible on every page.
	leafByID(t, w, 1)
	// Active pane must be on the visible page.
	if !w.ultraVisible(w.ActivePane.ID) {
		t.Fatalf("active pane %d not visible", w.ActivePane.ID)
	}
	// Wrap.
	if err := w.UltraStepPage(1); err != nil {
		t.Fatal(err)
	}
	if u.Page != 0 {
		t.Fatalf("expected wrap to page 0, got %d", u.Page)
	}
	if err := w.UltraSetPage(5); err == nil {
		t.Fatal("expected out-of-range error")
	}
	// Focusing a hidden pane switches page.
	w.FocusPane(w.Panes()[8])
	if u.Page != 1 || w.ActivePane.ID != 9 {
		t.Fatalf("focus hidden: page=%d active=%d", u.Page, w.ActivePane.ID)
	}
	// Closing everything on page 2 drops the page.
	for _, id := range []uint32{7, 8, 9} {
		if err := w.ClosePane(id); err != nil {
			t.Fatal(err)
		}
	}
	if u.PageCount() != 1 || u.Page != 0 {
		t.Fatalf("expected trailing page trimmed: pages=%d page=%d", u.PageCount(), u.Page)
	}
}

func TestUltraPromoteSwapsWithBusyVisiblePane(t *testing.T) {
	t.Parallel()
	w := ultraTestWindow(t, 8) // lead + 7 agents: slots 2..6 page 0, 7,8 page 1
	_ = w.SetLead(1)
	w.FocusPane(w.Panes()[0]) // lead active
	_ = w.EnableUltra(2, 3, true)
	busy := map[uint32]bool{2: false, 3: true, 4: true, 5: true, 6: true}
	changed := w.UltraPromote(8, func(id uint32) bool { return busy[id] })
	if !changed {
		t.Fatal("expected promotion")
	}
	ids := w.Ultra.SlotIDs()
	if ids[1] != 8 {
		t.Fatalf("expected pane 8 in slot 2 (pane 3 was busy, pane 2 idle stays): %v", ids)
	}
	if ids[6] != 3 {
		t.Fatalf("expected demoted pane 3 in slot 7: %v", ids)
	}
	if w.Root.FindPane(8) == nil || w.UltraPaneHidden(8) {
		t.Fatal("promoted pane not visible")
	}
	// Active pane is never demoted.
	w.FocusPane(w.Panes()[3]) // pane 4
	changed = w.UltraPromote(7, func(id uint32) bool { return true })
	if !changed {
		t.Fatal("expected promotion of 7")
	}
	if w.UltraPaneHidden(4) {
		t.Fatal("active pane was demoted")
	}
	// Nothing busy on the visible page -> no swap.
	w = ultraTestWindow(t, 7)
	_ = w.SetLead(1)
	w.FocusPane(w.Panes()[0])
	_ = w.EnableUltra(2, 3, true)
	if w.UltraPromote(7, func(uint32) bool { return false }) {
		t.Fatal("promotion should not demote idle panes")
	}
	// Visible pane not hidden -> no-op.
	if w.UltraPromote(2, func(uint32) bool { return true }) {
		t.Fatal("visible pane promotion should be a no-op")
	}
}

func TestUltraPromoteFillsEmptyVisibleSlotFirst(t *testing.T) {
	t.Parallel()
	w := ultraTestWindow(t, 7)
	_ = w.SetLead(1)
	w.FocusPane(w.Panes()[0])
	_ = w.EnableUltra(2, 3, true)
	_ = w.ClosePane(4) // slot 3 empty on page 0
	if !w.UltraPromote(7, func(uint32) bool { return true }) {
		t.Fatal("expected promotion into empty slot")
	}
	ids := w.Ultra.SlotIDs()
	if ids[2] != 7 {
		t.Fatalf("expected pane 7 in slot 3: %v", ids)
	}
	if w.Ultra.PageCount() != 1 {
		t.Fatalf("page 2 should be trimmed, pages=%d", w.Ultra.PageCount())
	}
}

func TestUltraLeadChangesAndDisable(t *testing.T) {
	t.Parallel()
	w := ultraTestWindow(t, 4)
	if err := w.EnableUltra(2, 3, true); err != nil {
		t.Fatal(err)
	}
	// No lead: cell 0 is an empty lead slot, all 4 panes are agents.
	if got := w.Ultra.SlotIDs(); len(got) != 4 {
		t.Fatalf("slots = %v", got)
	}
	var leadCell *LayoutCell
	w.Root.Walk(func(c *LayoutCell) {
		if c.LeadSlot {
			leadCell = c
		}
	})
	if leadCell == nil || leadCell.Pane != nil {
		t.Fatalf("expected empty lead cell, got %+v", leadCell)
	}
	if err := w.SetLead(3); err != nil {
		t.Fatal(err)
	}
	if got := w.Ultra.SlotIDs(); got[2] != 0 || w.LeadPaneID != 3 {
		t.Fatalf("after set-lead slots=%v lead=%d", got, w.LeadPaneID)
	}
	if c := leafByID(t, w, 3); !c.LeadSlot {
		t.Fatalf("pane 3 not in lead cell")
	}
	// Switching lead: old lead takes lowest empty slot (slot 3).
	if err := w.SetLead(4); err != nil {
		t.Fatal(err)
	}
	if got := w.Ultra.SlotIDs(); got[2] != 3 || got[3] != 0 {
		t.Fatalf("after switch lead slots=%v", got)
	}
	if err := w.UnsetLead(); err != nil {
		t.Fatal(err)
	}
	if got := w.Ultra.SlotIDs(); got[3] != 4 || w.LeadPaneID != 0 {
		t.Fatalf("after unset slots=%v lead=%d", got, w.LeadPaneID)
	}
	_ = w.SetLead(1)
	if err := w.DisableUltra(); err != nil {
		t.Fatal(err)
	}
	if w.IsUltra() || len(w.Panes()) != 4 || w.LeadPaneID != 1 {
		t.Fatalf("disable: ultra=%v panes=%d lead=%d", w.IsUltra(), len(w.Panes()), w.LeadPaneID)
	}
	w.Root.Walk(func(c *LayoutCell) {
		if c.Pane == nil {
			t.Fatalf("empty leaf survived disable: %+v", c)
		}
	})
	if c := leafByID(t, w, 1); c.X != 0 || c.H != w.Height {
		t.Fatalf("lead should be a full-height left column after disable: %+v", c)
	}
}

func TestUltraSnapshotRoundTrip(t *testing.T) {
	t.Parallel()
	w := ultraTestWindow(t, 8)
	_ = w.SetLead(1)
	_ = w.EnableUltra(2, 3, false)
	_ = w.ClosePane(3)
	_ = w.UltraSetPage(1)
	w.ID = 7
	snap := w.SnapshotWindow(1)
	if snap.Ultra == nil || snap.Ultra.Page != 1 || snap.Ultra.Pages != 2 || snap.Ultra.Rows != 2 || snap.Ultra.Cols != 3 {
		t.Fatalf("snapshot ultra = %+v", snap.Ultra)
	}
	hidden := 0
	for _, ps := range snap.Panes {
		if ps.Hidden {
			hidden++
		}
	}
	if hidden != 4 { // page 0 agents 2,4,5,6 hidden while on page 1
		t.Fatalf("hidden panes = %d", hidden)
	}
	paneMap := map[uint32]*Pane{}
	for _, p := range w.Panes() {
		paneMap[p.ID] = p
	}
	restored := RebuildWindowFromSnapshot(snap, 120, 40, paneMap)
	if restored.Ultra == nil || restored.Ultra.Page != 1 || restored.LeadPaneID != 1 {
		t.Fatalf("restored ultra = %+v lead=%d", restored.Ultra, restored.LeadPaneID)
	}
	if got, want := restored.Ultra.SlotIDs(), w.Ultra.SlotIDs(); len(got) != len(want) {
		t.Fatalf("slots %v != %v", got, want)
	} else {
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("slots %v != %v", got, want)
			}
		}
	}
	if restored.Root.FindPane(7) == nil || restored.Root.FindPane(2) != nil {
		t.Fatal("restored tree not on page 2")
	}
	if len(restored.Panes()) != 7 {
		t.Fatalf("restored panes = %d", len(restored.Panes()))
	}
}

func TestUltraResizeRebuildsGrid(t *testing.T) {
	t.Parallel()
	w := ultraTestWindow(t, 6)
	_ = w.SetLead(1)
	_ = w.EnableUltra(2, 3, true)
	w.Resize(200, 60)
	if w.Root.W != 200 || w.Root.H != 60 {
		t.Fatalf("root = %dx%d", w.Root.W, w.Root.H)
	}
	c := leafByID(t, w, 6)
	if c.X+c.W != 200 || c.Y+c.H != 60 {
		t.Fatalf("bottom-right cell %+v does not reach the corner", c)
	}
	// Tree mutations that don't fit the slot model are refused.
	if err := w.MovePane(2, 3, true); err == nil {
		t.Fatal("expected MovePane to be refused in ultra")
	}
	if w.ResizePane(2, "left", 2) {
		t.Fatal("ResizePane should be a no-op in ultra")
	}
	if err := w.SwapPanes(2, 6); err != nil {
		t.Fatal(err)
	}
	if got := w.Ultra.SlotIDs(); got[0] != 6 || got[4] != 2 {
		t.Fatalf("swap slots = %v", got)
	}
}

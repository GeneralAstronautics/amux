package mux

import (
	"fmt"
	"sort"
)

// Default ultra grid: 3 columns x 2 rows = 6 cells (lead + 5 agent slots).
const (
	DefaultUltraRows = 2
	DefaultUltraCols = 3
	MaxUltraRows     = 8
	MaxUltraCols     = 8
)

// UltraLayout is a fixed-slot, paginated grid ("ultra" layout).
//
// Cell 0 (top-left) is reserved for the window's lead pane and appears on every
// page. The remaining Rows*Cols-1 cells of each page show agent slots. Slots
// are stable: a pane keeps its slot until it exits, an exited pane leaves an
// empty slot, and new panes take the lowest empty slot. When more agent slots
// exist than fit on one page, the extra slots live on further pages; panes on
// hidden pages keep running and stay addressable (capture, send-keys, ...) but
// are not in the window's layout tree.
type UltraLayout struct {
	Rows int
	Cols int
	// Page is the 0-based visible page.
	Page int
	// Slots holds agent slots in order; nil means an empty slot. Slot i lives on
	// page i/AgentSlotsPerPage() at cell (i%AgentSlotsPerPage())+1.
	Slots []*Pane
	// AutoPromote moves a pane that goes idle on a hidden page onto the visible
	// page (swapping with a busy visible pane or filling an empty visible slot).
	AutoPromote bool
}

// AgentSlotsPerPage returns the number of agent cells on one page.
func (u *UltraLayout) AgentSlotsPerPage() int {
	n := u.Rows*u.Cols - 1
	if n < 1 {
		return 1
	}
	return n
}

// PageCount returns the number of pages (always >= 1).
func (u *UltraLayout) PageCount() int {
	per := u.AgentSlotsPerPage()
	pages := (len(u.Slots) + per - 1) / per
	if pages < 1 {
		pages = 1
	}
	return pages
}

// SlotOf returns the agent slot index holding paneID, or -1.
func (u *UltraLayout) SlotOf(paneID uint32) int {
	for i, p := range u.Slots {
		if p != nil && p.ID == paneID {
			return i
		}
	}
	return -1
}

// PageOfSlot returns the page a slot index lives on.
func (u *UltraLayout) PageOfSlot(slot int) int {
	return slot / u.AgentSlotsPerPage()
}

// SlotIDs returns the slot table as pane IDs (0 = empty).
func (u *UltraLayout) SlotIDs() []uint32 {
	ids := make([]uint32, len(u.Slots))
	for i, p := range u.Slots {
		if p != nil {
			ids[i] = p.ID
		}
	}
	return ids
}

func (u *UltraLayout) clampPage() {
	if u.Page < 0 {
		u.Page = 0
	}
	if last := u.PageCount() - 1; u.Page > last {
		u.Page = last
	}
}

// trimEmptyPages drops whole trailing pages that hold no panes. Page 0 always
// survives, and partially filled pages keep their empty slots.
func (u *UltraLayout) trimEmptyPages() {
	per := u.AgentSlotsPerPage()
	for len(u.Slots) > per {
		start := ((len(u.Slots) - 1) / per) * per
		empty := true
		for _, p := range u.Slots[start:] {
			if p != nil {
				empty = false
				break
			}
		}
		if !empty {
			break
		}
		u.Slots = u.Slots[:start]
	}
	u.clampPage()
}

// insert places pane in the lowest empty slot (appending when full) and
// returns the slot index.
func (u *UltraLayout) insert(pane *Pane) int {
	for i, p := range u.Slots {
		if p == nil {
			u.Slots[i] = pane
			return i
		}
	}
	u.Slots = append(u.Slots, pane)
	return len(u.Slots) - 1
}

// ValidateUltraGrid checks rows/cols bounds.
func ValidateUltraGrid(rows, cols int) error {
	if rows < 1 || rows > MaxUltraRows {
		return fmt.Errorf("ultra rows must be 1..%d (got %d)", MaxUltraRows, rows)
	}
	if cols < 1 || cols > MaxUltraCols {
		return fmt.Errorf("ultra cols must be 1..%d (got %d)", MaxUltraCols, cols)
	}
	if rows*cols < 2 {
		return fmt.Errorf("ultra grid needs at least 2 cells (rows*cols >= 2)")
	}
	return nil
}

// IsUltra reports whether the window is in ultra layout.
func (w *Window) IsUltra() bool {
	return w != nil && w.Ultra != nil
}

// EnableUltra switches the window to the ultra grid. Existing panes keep their
// current walk order: the lead pane (if any) takes cell 0 and the others fill
// slots 0..n-1. Re-enabling with different dimensions re-flows the same slot
// order onto the new page size.
func (w *Window) EnableUltra(rows, cols int, autoPromote bool) error {
	w.assertOwner("EnableUltra")
	if err := ValidateUltraGrid(rows, cols); err != nil {
		return err
	}
	if w.ZoomedPaneID != 0 {
		if err := w.Unzoom(); err != nil {
			return err
		}
	}
	if w.Ultra != nil {
		w.Ultra.Rows = rows
		w.Ultra.Cols = cols
		w.Ultra.AutoPromote = autoPromote
		w.Ultra.trimEmptyPages()
		w.ultraShowPaneIfActive()
		w.rebuildUltraTree()
		return nil
	}

	u := &UltraLayout{Rows: rows, Cols: cols, AutoPromote: autoPromote}
	w.ultraLead = nil
	if w.LeadPaneID != 0 {
		if cell := w.Root.FindPane(w.LeadPaneID); cell != nil {
			w.ultraLead = cell.Pane
		} else {
			w.LeadPaneID = 0
		}
	}
	for _, p := range w.Panes() {
		if w.LeadPaneID != 0 && p.ID == w.LeadPaneID {
			continue
		}
		u.Slots = append(u.Slots, p)
	}
	// Initial slot order follows pane ID (spawn order) so the grid is
	// predictable regardless of how the previous split tree was arranged.
	sort.SliceStable(u.Slots, func(a, b int) bool { return u.Slots[a].ID < u.Slots[b].ID })
	w.Ultra = u
	w.ultraShowPaneIfActive()
	w.rebuildUltraTree()
	return nil
}

// DisableUltra leaves the ultra grid and rebuilds a plain split layout with
// every pane visible: the lead pane (if any) anchored as a full-height left
// column and the agent panes in a row-major grid on the right.
func (w *Window) DisableUltra() error {
	w.assertOwner("DisableUltra")
	if w.Ultra == nil {
		return fmt.Errorf("window is not in ultra layout")
	}
	u := w.Ultra
	var agents []*Pane
	for _, p := range u.Slots {
		if p != nil {
			agents = append(agents, p)
		}
	}
	lead := w.leadPane()
	w.Ultra = nil

	var root *LayoutCell
	switch {
	case lead == nil && len(agents) == 0:
		return fmt.Errorf("window has no panes")
	case lead == nil:
		root = buildGridTree(agents, u.Cols, 0, 0, w.Width, w.Height)
	case len(agents) == 0:
		root = NewLeaf(lead, 0, 0, w.Width, w.Height)
	default:
		leadW := max(20, w.Width/3)
		if leadW > w.Width*4/5 {
			leadW = w.Width * 4 / 5
		}
		rightW := w.Width - leadW - 1
		leadLeaf := NewLeaf(lead, 0, 0, leadW, w.Height)
		grid := buildGridTree(agents, u.Cols, leadW+1, 0, rightW, w.Height)
		root = &LayoutCell{X: 0, Y: 0, W: w.Width, H: w.Height, Dir: SplitVertical, Children: []*LayoutCell{leadLeaf, grid}}
		leadLeaf.Parent = root
		grid.Parent = root
	}
	w.Root = root
	w.Root.FixOffsets()
	if w.ActivePane == nil || w.Root.FindPane(w.ActivePane.ID) == nil {
		if p := firstPaneInSubtree(w.Root); p != nil {
			w.setActive(p)
		}
	}
	w.resizePTYs()
	return nil
}

// buildGridTree lays panes out row-major in a grid with the given number of
// columns. Rows are added as needed; the last row may be short. Nil panes are
// allowed and become empty leaves.
func buildGridTree(panes []*Pane, cols, x, y, width, height int) *LayoutCell {
	if cols < 1 {
		cols = 1
	}
	rows := (len(panes) + cols - 1) / cols
	if rows < 1 {
		rows = 1
	}
	if len(panes) == 1 {
		return NewLeaf(panes[0], x, y, width, height)
	}
	rowH := equalSplitSizes(height, rows)
	root := &LayoutCell{X: x, Y: y, W: width, H: height, Dir: SplitHorizontal}
	if rows == 1 {
		root = nil
	}
	yoff := y
	var rowCells []*LayoutCell
	for r := 0; r < rows; r++ {
		rowPanes := panes[r*cols:]
		if len(rowPanes) > cols {
			rowPanes = rowPanes[:cols]
		}
		h := height
		if rows > 1 {
			h = rowH[r]
		}
		var rowCell *LayoutCell
		if len(rowPanes) == 1 {
			rowCell = NewLeaf(rowPanes[0], x, yoff, width, h)
		} else {
			colW := equalSplitSizes(width, len(rowPanes))
			rowCell = &LayoutCell{X: x, Y: yoff, W: width, H: h, Dir: SplitVertical}
			xoff := x
			for c, p := range rowPanes {
				leaf := NewLeaf(p, xoff, yoff, colW[c], h)
				leaf.Parent = rowCell
				rowCell.Children = append(rowCell.Children, leaf)
				xoff += colW[c] + 1
			}
		}
		if root == nil {
			return rowCell
		}
		rowCell.Parent = root
		rowCells = append(rowCells, rowCell)
		yoff += h + 1
	}
	root.Children = rowCells
	return root
}

func (w *Window) leadPane() *Pane {
	if w.LeadPaneID == 0 {
		return nil
	}
	if w.Ultra != nil {
		return w.ultraLead
	}
	if cell := w.Root.FindPane(w.LeadPaneID); cell != nil {
		return cell.Pane
	}
	return nil
}

// rebuildUltraTree regenerates w.Root from the ultra slot table for the
// visible page. Every cell of the grid is a leaf; cells without a pane are
// empty leaves (Pane == nil) tagged with their slot number. Hidden panes are
// resized to the agent-cell size so paging is instant.
func (w *Window) rebuildUltraTree() {
	u := w.Ultra
	if u == nil {
		return
	}
	u.clampPage()
	per := u.AgentSlotsPerPage()
	start := u.Page * per

	lead := w.ultraLead
	if w.LeadPaneID == 0 {
		lead = nil
	}
	cells := make([]*Pane, u.Rows*u.Cols)
	slotNums := make([]int, u.Rows*u.Cols)
	cells[0] = lead
	slotNums[0] = -1
	for k := 1; k < len(cells); k++ {
		idx := start + k - 1
		slotNums[k] = idx + 1
		if idx < len(u.Slots) {
			cells[k] = u.Slots[idx]
		}
	}

	rowH := equalSplitSizes(w.Height, u.Rows)
	colW := equalSplitSizes(w.Width, u.Cols)
	for i := range rowH {
		if rowH[i] < PaneMinSize {
			rowH[i] = PaneMinSize
		}
	}
	for i := range colW {
		if colW[i] < PaneMinSize {
			colW[i] = PaneMinSize
		}
	}

	var root *LayoutCell
	var rows []*LayoutCell
	yoff := 0
	for r := 0; r < u.Rows; r++ {
		var rowCell *LayoutCell
		if u.Cols == 1 {
			k := r
			rowCell = NewLeaf(cells[k], 0, yoff, w.Width, rowH[r])
			rowCell.Slot = slotNums[k]
			rowCell.LeadSlot = k == 0
		} else {
			rowCell = &LayoutCell{X: 0, Y: yoff, W: w.Width, H: rowH[r], Dir: SplitVertical}
			xoff := 0
			for c := 0; c < u.Cols; c++ {
				k := r*u.Cols + c
				leaf := NewLeaf(cells[k], xoff, yoff, colW[c], rowH[r])
				leaf.Slot = slotNums[k]
				leaf.LeadSlot = k == 0
				leaf.Parent = rowCell
				rowCell.Children = append(rowCell.Children, leaf)
				xoff += colW[c] + 1
			}
		}
		rows = append(rows, rowCell)
		yoff += rowH[r] + 1
	}
	if u.Rows == 1 {
		root = rows[0]
	} else {
		root = &LayoutCell{X: 0, Y: 0, W: w.Width, H: w.Height, Dir: SplitHorizontal, Children: rows}
		for _, rc := range rows {
			rc.Parent = root
		}
	}
	root.Parent = nil
	w.Root = root
	w.Root.FixOffsets()

	// Keep the active pane on the visible page.
	if w.ActivePane == nil || !w.ultraVisible(w.ActivePane.ID) {
		if lead != nil {
			w.setActive(lead)
		} else if p := firstPaneInSubtree(w.Root); p != nil {
			w.setActive(p)
		} else if len(u.Slots) > 0 {
			for _, p := range u.Slots {
				if p != nil {
					w.setActive(p)
					break
				}
			}
		}
	}

	w.resizePTYsPreservingZoomedPaneSize()
	// Hidden panes: size them like an agent cell so they render correctly the
	// moment they are paged in.
	if len(cells) > 1 {
		agentCell := w.Root.FindLeafAt(0, 0)
		if u.Cols > 1 {
			agentCell = w.Root.FindLeafAt(colW[0]+1, 0)
		} else if u.Rows > 1 {
			agentCell = w.Root.FindLeafAt(0, rowH[0]+1)
		}
		if agentCell != nil {
			for i, p := range u.Slots {
				if p == nil || (i >= start && i < start+per) {
					continue
				}
				_ = p.Resize(agentCell.W, PaneContentHeight(agentCell.H))
			}
		}
	}
}

// ultraVisible reports whether paneID is on the visible page (or is the lead).
func (w *Window) ultraVisible(paneID uint32) bool {
	u := w.Ultra
	if u == nil {
		return true
	}
	if w.LeadPaneID != 0 && paneID == w.LeadPaneID {
		return true
	}
	idx := u.SlotOf(paneID)
	if idx < 0 {
		return false
	}
	return u.PageOfSlot(idx) == u.Page
}

// UltraPaneHidden reports whether the pane exists in this window but is on a
// hidden page.
func (w *Window) UltraPaneHidden(paneID uint32) bool {
	if w.Ultra == nil {
		return false
	}
	return w.Ultra.SlotOf(paneID) >= 0 && !w.ultraVisible(paneID)
}

// ultraShowPane switches to the page holding paneID (no-op when visible).
func (w *Window) ultraShowPane(paneID uint32) bool {
	u := w.Ultra
	if u == nil || w.ultraVisible(paneID) {
		return false
	}
	idx := u.SlotOf(paneID)
	if idx < 0 {
		return false
	}
	u.Page = u.PageOfSlot(idx)
	w.rebuildUltraTree()
	return true
}

func (w *Window) ultraShowPaneIfActive() {
	if w.Ultra == nil || w.ActivePane == nil {
		return
	}
	if idx := w.Ultra.SlotOf(w.ActivePane.ID); idx >= 0 {
		w.Ultra.Page = w.Ultra.PageOfSlot(idx)
	}
}

// UltraSetPage shows the given 0-based page.
func (w *Window) UltraSetPage(page int) error {
	w.assertOwner("UltraSetPage")
	if w.Ultra == nil {
		return fmt.Errorf("window is not in ultra layout")
	}
	if page < 0 || page >= w.Ultra.PageCount() {
		return fmt.Errorf("page %d out of range (1..%d)", page+1, w.Ultra.PageCount())
	}
	if w.ZoomedPaneID != 0 {
		if err := w.Unzoom(); err != nil {
			return err
		}
	}
	w.Ultra.Page = page
	w.rebuildUltraTree()
	return nil
}

// UltraStepPage moves delta pages, wrapping around.
func (w *Window) UltraStepPage(delta int) error {
	w.assertOwner("UltraStepPage")
	if w.Ultra == nil {
		return fmt.Errorf("window is not in ultra layout")
	}
	n := w.Ultra.PageCount()
	page := ((w.Ultra.Page+delta)%n + n) % n
	return w.UltraSetPage(page)
}

// ultraInsert assigns a new pane to the lowest empty slot.
func (w *Window) ultraInsert(pane *Pane, opts SplitOptions) (*Pane, error) {
	u := w.Ultra
	idx := u.insert(pane)
	if !opts.KeepFocus {
		u.Page = u.PageOfSlot(idx)
	}
	w.rebuildUltraTree()
	if !opts.KeepFocus {
		w.setActive(pane)
	}
	return pane, nil
}

// ultraClose removes a pane from the slot table (leaving the slot empty) or
// clears the lead cell.
func (w *Window) ultraClose(paneID uint32) error {
	u := w.Ultra
	total := w.PaneCount()
	if total <= 1 {
		return fmt.Errorf("cannot close last pane")
	}
	if w.ZoomedPaneID == paneID {
		w.ZoomedPaneID = 0
	}
	wasActive := w.ActivePane != nil && w.ActivePane.ID == paneID
	if w.LeadPaneID == paneID {
		w.LeadPaneID = 0
		w.ultraLead = nil
	} else {
		idx := u.SlotOf(paneID)
		if idx < 0 {
			return fmt.Errorf("pane %d not found in layout", paneID)
		}
		u.Slots[idx] = nil
		u.trimEmptyPages()
	}
	if wasActive {
		w.ActivePane = nil
	}
	w.rebuildUltraTree()
	return nil
}

// ultraSetLead moves paneID into the lead cell; a previous lead (if any)
// takes the lowest empty agent slot.
func (w *Window) ultraSetLead(paneID uint32) error {
	u := w.Ultra
	if w.LeadPaneID == paneID {
		return nil
	}
	idx := u.SlotOf(paneID)
	if idx < 0 {
		return fmt.Errorf("pane %d not found", paneID)
	}
	pane := u.Slots[idx]
	u.Slots[idx] = nil
	if prev := w.ultraLead; prev != nil && w.LeadPaneID != 0 {
		u.insert(prev)
	}
	w.LeadPaneID = paneID
	w.ultraLead = pane
	u.trimEmptyPages()
	w.rebuildUltraTree()
	return nil
}

func (w *Window) ultraUnsetLead() error {
	u := w.Ultra
	if w.LeadPaneID == 0 {
		return fmt.Errorf("no lead pane set")
	}
	if w.ultraLead != nil {
		u.insert(w.ultraLead)
	}
	w.LeadPaneID = 0
	w.ultraLead = nil
	w.rebuildUltraTree()
	return nil
}

// ultraSwap exchanges two agent slots.
func (w *Window) ultraSwap(id1, id2 uint32) error {
	u := w.Ultra
	i, j := u.SlotOf(id1), u.SlotOf(id2)
	if i < 0 {
		return fmt.Errorf("pane %d not found in layout", id1)
	}
	if j < 0 {
		return fmt.Errorf("pane %d not found in layout", id2)
	}
	u.Slots[i], u.Slots[j] = u.Slots[j], u.Slots[i]
	w.rebuildUltraTree()
	return nil
}

// UltraPromote brings a hidden pane onto the visible page. It swaps with a
// busy pane on the visible page (never the active pane, never the lead), or
// moves into an empty visible slot. Returns true when the layout changed.
// busy reports whether a pane is currently busy (not idle).
func (w *Window) UltraPromote(paneID uint32, busy func(uint32) bool) bool {
	w.assertOwner("UltraPromote")
	u := w.Ultra
	if u == nil || !w.UltraPaneHidden(paneID) {
		return false
	}
	from := u.SlotOf(paneID)
	per := u.AgentSlotsPerPage()
	start := u.Page * per
	end := min(start+per, len(u.Slots))

	activeID := uint32(0)
	if w.ActivePane != nil {
		activeID = w.ActivePane.ID
	}

	// Prefer an empty visible slot.
	for i := start; i < end; i++ {
		if u.Slots[i] == nil {
			u.Slots[i], u.Slots[from] = u.Slots[from], nil
			u.trimEmptyPages()
			w.rebuildUltraTree()
			return true
		}
	}
	// Otherwise demote the busy visible pane that has been idle-checked
	// least recently active (lowest ActivePoint) to keep the swap predictable.
	var candidates []int
	for i := start; i < end; i++ {
		p := u.Slots[i]
		if p == nil || p.ID == activeID {
			continue
		}
		if busy != nil && !busy(p.ID) {
			continue
		}
		candidates = append(candidates, i)
	}
	if len(candidates) == 0 {
		return false
	}
	sort.SliceStable(candidates, func(a, b int) bool {
		return u.Slots[candidates[a]].ActivePoint < u.Slots[candidates[b]].ActivePoint
	})
	to := candidates[0]
	u.Slots[to], u.Slots[from] = u.Slots[from], u.Slots[to]
	w.rebuildUltraTree()
	return true
}

// ultraPanes returns lead + all slot panes (visible and hidden) in slot order.
func (w *Window) ultraPanes() []*Pane {
	var panes []*Pane
	if w.LeadPaneID != 0 && w.ultraLead != nil {
		panes = append(panes, w.ultraLead)
	}
	for _, p := range w.Ultra.Slots {
		if p != nil {
			panes = append(panes, p)
		}
	}
	return panes
}

// HasPane reports whether the pane belongs to this window, including panes on
// hidden ultra pages.
func (w *Window) HasPane(paneID uint32) bool {
	if w == nil || w.Root == nil {
		return false
	}
	if w.Ultra != nil {
		if w.LeadPaneID != 0 && paneID == w.LeadPaneID && w.ultraLead != nil {
			return true
		}
		return w.Ultra.SlotOf(paneID) >= 0
	}
	return w.Root.FindPane(paneID) != nil
}

// PaneContentSize returns the PTY size (cols, rows) a pane in this window is
// laid out at, including hidden ultra panes (sized like an agent cell).
func (w *Window) PaneContentSize(paneID uint32) (cols, rows int, ok bool) {
	if w == nil || w.Root == nil {
		return 0, 0, false
	}
	if cell := w.Root.FindPane(paneID); cell != nil {
		return cell.W, PaneContentHeight(cell.H), true
	}
	if w.Ultra != nil && w.Ultra.SlotOf(paneID) >= 0 {
		var agentCell *LayoutCell
		w.Root.Walk(func(c *LayoutCell) {
			if agentCell == nil && !c.LeadSlot {
				agentCell = c
			}
		})
		if agentCell != nil {
			return agentCell.W, PaneContentHeight(agentCell.H), true
		}
	}
	return 0, 0, false
}

// ultraReplacePane swaps pane pointers in the slot table / lead cell.
func (w *Window) ultraReplacePane(oldPaneID uint32, replacement *Pane) bool {
	u := w.Ultra
	if w.LeadPaneID == oldPaneID && w.ultraLead != nil {
		w.ultraLead = replacement
		w.LeadPaneID = replacement.ID
		return true
	}
	if idx := u.SlotOf(oldPaneID); idx >= 0 {
		u.Slots[idx] = replacement
		return true
	}
	return false
}

var errUltraLayout = fmt.Errorf("not supported in ultra layout (use `amux layout off` first)")

// ultraUnsplice reverts a takeover in ultra mode: the first proxy pane for the
// host becomes the replacement; any other proxy panes for the host free their
// slots.
func (w *Window) ultraUnsplice(hostName string, replacement *Pane) error {
	u := w.Ultra
	replaced := false
	if w.ultraLead != nil && w.ultraLead.IsProxy() && w.ultraLead.Meta.Host == hostName {
		w.ultraLead = replacement
		w.LeadPaneID = replacement.ID
		replaced = true
	}
	for i, p := range u.Slots {
		if p == nil || !p.IsProxy() || p.Meta.Host != hostName {
			continue
		}
		if !replaced {
			u.Slots[i] = replacement
			replaced = true
		} else {
			u.Slots[i] = nil
		}
	}
	if !replaced {
		return fmt.Errorf("no spliced panes found for host %q", hostName)
	}
	if w.ActivePane == nil || w.ActivePane.Meta.Host == hostName || !w.HasPane(w.ActivePane.ID) {
		w.ActivePane = replacement
	}
	u.trimEmptyPages()
	w.rebuildUltraTree()
	return nil
}

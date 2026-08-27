package render

import (
	"sort"

	"github.com/weill-labs/amux/internal/config"
	"github.com/weill-labs/amux/internal/mux"
)

// PaneAttention describes transient client-side highlighting for a pane:
// a done-flash pulse or a needs-input glow. Empty fields mean "no override".
type PaneAttention struct {
	BorderHex   string // border color override
	StatusBgHex string // pane status-line background override
}

// PaneAttentionProvider is an optional PaneData extension. Client-side pane
// data implements it; capture/server pane data do not, so captures and the
// server-side full renderer never see attention tints.
type PaneAttentionProvider interface {
	Attention() PaneAttention
}

func paneAttentionFor(pd PaneData) PaneAttention {
	if provider, ok := pd.(PaneAttentionProvider); ok {
		return provider.Attention()
	}
	return PaneAttention{}
}

// paneStatusBgHex returns the status-line background for a pane, honouring
// pressed state and any attention override.
func paneStatusBgHex(pd PaneData, pressed bool) string {
	if att := paneAttentionFor(pd); att.StatusBgHex != "" {
		return att.StatusBgHex
	}
	return statusBarBaseBgHex(pressed)
}

// paneTintParity assigns each leaf pane a checkerboard parity based on its
// column rank (distinct X positions, left to right) and its row rank within
// panes sharing that X. Returns the set of pane IDs with odd parity.
func paneTintParity(root *mux.LayoutCell) map[uint32]struct{} {
	if root == nil {
		return nil
	}
	type leaf struct {
		id   uint32
		x, y int
	}
	var leaves []leaf
	root.Walk(func(cell *mux.LayoutCell) {
		if pid := cell.CellPaneID(); pid != 0 {
			leaves = append(leaves, leaf{id: pid, x: cell.X, y: cell.Y})
		}
	})
	if len(leaves) < 2 {
		return nil
	}
	xs := make([]int, 0, len(leaves))
	seen := make(map[int]struct{})
	for _, l := range leaves {
		if _, ok := seen[l.x]; !ok {
			seen[l.x] = struct{}{}
			xs = append(xs, l.x)
		}
	}
	sort.Ints(xs)
	colRank := make(map[int]int, len(xs))
	for i, x := range xs {
		colRank[x] = i
	}
	sort.Slice(leaves, func(i, j int) bool {
		if leaves[i].x != leaves[j].x {
			return leaves[i].x < leaves[j].x
		}
		return leaves[i].y < leaves[j].y
	})
	odd := make(map[uint32]struct{})
	rowRank := 0
	lastX := -1
	for _, l := range leaves {
		if l.x != lastX {
			rowRank = 0
			lastX = l.x
		}
		if (colRank[l.x]+rowRank)%2 == 1 {
			odd[l.id] = struct{}{}
		}
		rowRank++
	}
	return odd
}

// applyPaneContentBackground fills the default (unset) background of a pane's
// visible content cells with bg.
func applyPaneContentBackground(g *ScreenGrid, cell *mux.LayoutCell, contentH int, bgHex string) {
	if bgHex == "" || contentH <= 0 {
		return
	}
	bg := hexToColor(bgHex)
	if bg == nil {
		return
	}
	for row := 0; row < contentH; row++ {
		y := cell.Y + mux.StatusLineRows + row
		for col := 0; col < cell.W; col++ {
			dst := g.cellForWrite(cell.X+col, y)
			if dst == nil {
				continue
			}
			if dst.Style.Bg == nil {
				dst.Style.Bg = bg
			}
		}
	}
}

// accentHex returns the render-time accent for a pane, applying palette
// softening.
func accentHex(hex string) string {
	return config.RenderAccent(hex)
}

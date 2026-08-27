package render

import (
	"strconv"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/weill-labs/amux/internal/config"
	"github.com/weill-labs/amux/internal/mux"
)

// emptySlotLabel returns the placeholder text for an empty ultra cell.
func emptySlotLabel(cell *mux.LayoutCell) string {
	if cell.LeadSlot {
		return "lead · empty (amux lead <pane>)"
	}
	if cell.Slot > 0 {
		return "slot " + strconv.Itoa(cell.Slot) + " · empty"
	}
	return ""
}

// buildEmptySlotCells draws a dim centred label into every ultra cell that has
// no pane so empty slots read as deliberate rather than as a rendering hole.
func buildEmptySlotCells(g *ScreenGrid, root *mux.LayoutCell, layoutHeight int) {
	if root == nil {
		return
	}
	style := uv.Style{Fg: hexToColor(config.DimColorHex)}
	root.Walk(func(cell *mux.LayoutCell) {
		if cell.CellPaneID() != 0 {
			return
		}
		label := emptySlotLabel(cell)
		if label == "" || cell.W < 2 || cell.H < 1 {
			return
		}
		if len([]rune(label)) > cell.W {
			label = truncateRunes(label, cell.W)
		}
		y := cell.Y + cell.H/2
		if y >= layoutHeight || y >= cell.Y+cell.H {
			return
		}
		x := cell.X + (cell.W-len([]rune(label)))/2
		for i, r := range []rune(label) {
			if x+i >= cell.X+cell.W {
				break
			}
			g.Set(x+i, y, ScreenCell{Char: string(r), Style: style, Width: 1})
		}
	})
}

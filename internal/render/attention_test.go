package render

import (
	"testing"

	"github.com/weill-labs/amux/internal/mux"
)

type attentionFakePane struct {
	fakePaneData
	att PaneAttention
}

func (p *attentionFakePane) Attention() PaneAttention { return p.att }

func TestGridAppliesAttentionAndAlternateTints(t *testing.T) {
	t.Parallel()
	// Two side-by-side panes: pane 1 (x=0..9) active, pane 2 (x=11..20) flashing.
	left := mux.NewLeafByID(1, 0, 0, 10, 5)
	right := mux.NewLeafByID(2, 11, 0, 10, 5)
	root := &mux.LayoutCell{Dir: mux.SplitVertical, W: 21, H: 5, Children: []*mux.LayoutCell{left, right}}
	left.Parent, right.Parent = root, root

	flash := PaneAttention{BorderHex: "11ff11", StatusBgHex: "225522"}
	panes := map[uint32]PaneData{
		1: &attentionFakePane{fakePaneData: fakePaneData{id: 1, name: "a", color: "89b4fa", screen: "hello"}},
		2: &attentionFakePane{fakePaneData: fakePaneData{id: 2, name: "b", color: "a6e3a1", screen: "world"}, att: flash},
	}
	lookup := func(id uint32) PaneData { return panes[id] }

	comp := NewCompositor(21, 6, "s")
	g, _ := comp.buildGridWithOverlay(root, 1, lookup, OverlayState{})

	// Shared border column x=10 is adjacent to pane 2 → flash border color wins.
	if got := g.Get(10, 2).Style.Fg; got != hexToColor("11ff11") {
		t.Fatalf("expected flash border fg, got %v", got)
	}
	// Pane 2 status line uses the flash background.
	if got := g.Get(11, 0).Style.Bg; got != hexToColor("225522") {
		t.Fatalf("expected flash status bg, got %v", got)
	}
	// Pane 1 status line keeps the default surface background.
	if got := g.Get(0, 0).Style.Bg; got != hexToColor("313244") {
		t.Fatalf("expected default status bg, got %v", got)
	}
}

package render

import (
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/weill-labs/amux/internal/mux"
)

func TestPaneTintParityCheckerboard(t *testing.T) {
	t.Parallel()
	// Two columns; left column split into two rows.
	left := mux.NewLeafByID(1, 0, 0, 40, 10)
	leftBottom := mux.NewLeafByID(2, 0, 11, 40, 10)
	right := mux.NewLeafByID(3, 41, 0, 40, 21)
	root := &mux.LayoutCell{Dir: mux.SplitVertical, Children: []*mux.LayoutCell{
		{Dir: mux.SplitHorizontal, Children: []*mux.LayoutCell{left, leftBottom}},
		right,
	}}
	odd := paneTintParity(root)
	if _, ok := odd[1]; ok {
		t.Fatal("pane 1 (col 0,row 0) should be even")
	}
	if _, ok := odd[2]; !ok {
		t.Fatal("pane 2 (col 0,row 1) should be odd")
	}
	if _, ok := odd[3]; !ok {
		t.Fatal("pane 3 (col 1,row 0) should be odd")
	}
	if paneTintParity(mux.NewLeafByID(9, 0, 0, 10, 10)) != nil {
		t.Fatal("single pane should get no tint")
	}
}

func TestApplyPaneContentBackgroundOnlyFillsDefaultBg(t *testing.T) {
	t.Parallel()
	g := NewScreenGrid(10, 5)
	cell := mux.NewLeafByID(1, 0, 0, 10, 5)
	explicit := hexToColor("ff0000")
	g.Set(2, 1, ScreenCell{Char: "x", Width: 1, Style: uv.Style{Bg: explicit}})
	applyPaneContentBackground(g, cell, 3, "222222")
	if got := g.Get(0, 1).Style.Bg; got == nil {
		t.Fatal("default-bg cell should be tinted")
	}
	if got := g.Get(2, 1).Style.Bg; got != explicit {
		t.Fatalf("explicit bg must be preserved, got %v", got)
	}
	if got := g.Get(0, 0).Style.Bg; got != nil {
		t.Fatal("status row must not be tinted")
	}
	if got := g.Get(0, 4).Style.Bg; got != nil {
		t.Fatal("rows beyond contentH must not be tinted")
	}
}

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
	comp.SetAlternateTintBg("232323")
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
	// Alternating tint: pane 1 (col 0) untinted, pane 2 (col 1) tinted.
	if got := g.Get(0, 1).Style.Bg; got != nil {
		t.Fatalf("pane 1 content should have default bg, got %v", got)
	}
	if got := g.Get(11, 1).Style.Bg; got != hexToColor("232323") {
		t.Fatalf("pane 2 content should be tinted, got %v", got)
	}

	// Disabling alternate tints removes the content background.
	comp.SetAlternateTintBg("")
	g, _ = comp.buildGridWithOverlay(root, 1, lookup, OverlayState{})
	if got := g.Get(11, 1).Style.Bg; got != nil {
		t.Fatalf("expected no tint after disabling, got %v", got)
	}
}

package render

import (
	"github.com/weill-labs/amux/internal/config"
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

// accentHex returns the render-time accent for a pane, applying palette
// softening.
func accentHex(hex string) string {
	return config.RenderAccent(hex)
}

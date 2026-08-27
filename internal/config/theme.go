package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Theme presets select a base palette for borders, status lines, and the
// global bar. Individual colors can be overridden in [theme.colors].
const (
	ThemePresetDefault = "default"
	ThemePresetSoft    = "soft"
)

// Palette is the set of named UI colors used by the renderer. Every value is a
// 6-digit lowercase hex string without a leading '#'.
type Palette struct {
	Dim       string // inactive/dim borders
	Text      string // status text foreground
	Surface0  string // status bar background
	Surface1  string // pressed status bar background
	Blue      string // focused window tab / default active border
	Mauve     string // modal accent
	FooterKey string // modal footer key names
	Green     string // success indicators
	Yellow    string // warning indicators
	Peach     string // reconnecting indicators
	Red       string // error indicators

	// AccentSoften blends per-pane accent colors toward their own grey level
	// when rendering borders and titles. 0 keeps the server-assigned accent
	// unchanged; 1 is fully desaturated. Capture output is never affected.
	AccentSoften float64
}

// DefaultPalette is the Catppuccin Mocha palette amux has always used.
func DefaultPalette() Palette {
	return Palette{
		Dim:       "6c7086",
		Text:      "cdd6f4",
		Surface0:  "313244",
		Surface1:  "45475a",
		Blue:      "89b4fa",
		Mauve:     "cba6f7",
		FooterKey: "858392",
		Green:     "a6e3a1",
		Yellow:    "f9e2af",
		Peach:     "fab387",
		Red:       "f38ba8",
	}
}

// SoftPalette is a lower-contrast, desaturated variant intended for terminals
// such as Ghostty where the default palette reads as harsh next to many panes.
func SoftPalette() Palette {
	return Palette{
		Dim:          "4a4d5e",
		Text:         "b9bdd0",
		Surface0:     "262634",
		Surface1:     "343548",
		Blue:         "8ea6d4",
		Mauve:        "b3a0d6",
		FooterKey:    "7a7890",
		Green:        "9cc5a2",
		Yellow:       "d8c89f",
		Peach:        "d9a98a",
		Red:          "d4919f",
		AccentSoften: 0.3,
	}
}

// PaletteForPreset returns the base palette for a theme preset name.
func PaletteForPreset(preset string) (Palette, error) {
	switch preset {
	case "", ThemePresetDefault:
		return DefaultPalette(), nil
	case ThemePresetSoft:
		return SoftPalette(), nil
	default:
		return Palette{}, fmt.Errorf(`theme.preset must be one of "default" or "soft"`)
	}
}

// ThemeColors holds per-role palette overrides from [theme.colors]. Empty
// values inherit from the preset.
type ThemeColors struct {
	Dim          string   `toml:"dim"`
	Text         string   `toml:"text"`
	Surface0     string   `toml:"surface0"`
	Surface1     string   `toml:"surface1"`
	Blue         string   `toml:"blue"`
	Mauve        string   `toml:"mauve"`
	FooterKey    string   `toml:"footer_key"`
	Green        string   `toml:"green"`
	Yellow       string   `toml:"yellow"`
	Peach        string   `toml:"peach"`
	Red          string   `toml:"red"`
	AccentSoften *float64 `toml:"accent_soften"`
}

// ResolvePalette builds the effective palette from a preset and overrides.
func ResolvePalette(preset string, overrides ThemeColors) (Palette, error) {
	p, err := PaletteForPreset(preset)
	if err != nil {
		return Palette{}, err
	}
	apply := func(dst *string, key, value string) error {
		if value == "" {
			return nil
		}
		hex, err := NormalizeHexColor("theme.colors."+key, value)
		if err != nil {
			return err
		}
		*dst = hex
		return nil
	}
	fields := []struct {
		dst   *string
		key   string
		value string
	}{
		{&p.Dim, "dim", overrides.Dim},
		{&p.Text, "text", overrides.Text},
		{&p.Surface0, "surface0", overrides.Surface0},
		{&p.Surface1, "surface1", overrides.Surface1},
		{&p.Blue, "blue", overrides.Blue},
		{&p.Mauve, "mauve", overrides.Mauve},
		{&p.FooterKey, "footer_key", overrides.FooterKey},
		{&p.Green, "green", overrides.Green},
		{&p.Yellow, "yellow", overrides.Yellow},
		{&p.Peach, "peach", overrides.Peach},
		{&p.Red, "red", overrides.Red},
	}
	for _, f := range fields {
		if err := apply(f.dst, f.key, f.value); err != nil {
			return Palette{}, err
		}
	}
	if overrides.AccentSoften != nil {
		v := *overrides.AccentSoften
		if v < 0 || v > 1 {
			return Palette{}, fmt.Errorf("theme.colors.accent_soften must be between 0 and 1")
		}
		p.AccentSoften = v
	}
	return p, nil
}

// EffectivePalette returns the palette for this config, falling back to the
// default palette on any error.
func (c *Config) EffectivePalette() Palette {
	if c == nil {
		return DefaultPalette()
	}
	p, err := ResolvePalette(c.Theme.Preset, c.Theme.Colors)
	if err != nil {
		return DefaultPalette()
	}
	return p
}

// ApplyPalette installs p as the process-wide renderer palette. It is intended
// to be called once at client startup before any rendering happens.
func ApplyPalette(p Palette) {
	DimColorHex = p.Dim
	TextColorHex = p.Text
	Surface0Hex = p.Surface0
	Surface1Hex = p.Surface1
	BlueHex = p.Blue
	MauveHex = p.Mauve
	FooterKeyHex = p.FooterKey
	GreenHex = p.Green
	YellowHex = p.Yellow
	PeachHex = p.Peach
	RedHex = p.Red
	accentSoften = p.AccentSoften
}

// ResetPalette restores the built-in default palette.
func ResetPalette() { ApplyPalette(DefaultPalette()) }

var accentSoften float64

// RenderAccent returns the accent hex color to use when drawing a pane's
// border or title, applying the palette's accent softening. The server-assigned
// accent (used by capture --colors) is never changed.
func RenderAccent(hex string) string {
	if accentSoften <= 0 || len(hex) != 6 {
		return hex
	}
	return SoftenHex(hex, accentSoften)
}

// SoftenHex blends a hex color toward its own luminance grey by amount (0..1).
func SoftenHex(hex string, amount float64) string {
	r, g, b, ok := parseHexRGB(hex)
	if !ok {
		return hex
	}
	if amount < 0 {
		amount = 0
	}
	if amount > 1 {
		amount = 1
	}
	luma := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	mix := func(c uint8) uint8 {
		v := float64(c)*(1-amount) + luma*amount
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		return uint8(v + 0.5)
	}
	return fmt.Sprintf("%02x%02x%02x", mix(r), mix(g), mix(b))
}

func parseHexRGB(hex string) (r, g, b uint8, ok bool) {
	if len(hex) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v), true
}

// NormalizeHexColor validates a user-supplied color ("#rrggbb" or "rrggbb")
// and returns the lowercase 6-digit form without '#'.
func NormalizeHexColor(field, value string) (string, error) {
	v := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "#"))
	if _, _, _, ok := parseHexRGB(strings.ToLower(v)); !ok {
		return "", fmt.Errorf("%s must be a 6-digit hex color such as \"#a6e3a1\"", field)
	}
	return strings.ToLower(v), nil
}

// Attention (done-flash / needs-input glow) defaults.
const (
	DefaultDoneFlashColor      = "a6e3a1" // green border while flashing
	DefaultDoneFlashBg         = "3a5a45" // muted green title background while flashing
	DefaultDoneFlashDurationMs = 1500
	DefaultDoneFlashPulses     = 3
	DefaultNeedsInputColor     = "b4befe" // lavender border
	DefaultNeedsInputBg        = "3b3b5e" // muted indigo title background
	DefaultAlternateTintBg     = "23232e" // subtle content background for odd panes
)

// AttentionSettings is the resolved done-flash / needs-input configuration.
type AttentionSettings struct {
	DoneFlash           bool
	DoneFlashColor      string
	DoneFlashBg         string
	DoneFlashDurationMs int
	DoneFlashPulses     int
	NeedsInputGlow      bool
	NeedsInputColor     string
	NeedsInputBg        string
}

// AlternateTintSettings is the resolved alternating-pane-tint configuration.
type AlternateTintSettings struct {
	Enabled bool
	Bg      string
}

func boolOr(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

func hexOr(field, value, def string) (string, error) {
	if value == "" {
		return def, nil
	}
	return NormalizeHexColor(field, value)
}

// ResolveAttention validates and resolves the attention settings.
func ResolveAttention(t ThemeConfig) (AttentionSettings, error) {
	var out AttentionSettings
	var err error
	out.DoneFlash = boolOr(t.DoneFlash, true)
	if out.DoneFlashColor, err = hexOr("theme.done_flash_color", t.DoneFlashColor, DefaultDoneFlashColor); err != nil {
		return out, err
	}
	if t.DoneFlashBg != nil {
		if *t.DoneFlashBg == "" {
			out.DoneFlashBg = ""
		} else if out.DoneFlashBg, err = NormalizeHexColor("theme.done_flash_bg", *t.DoneFlashBg); err != nil {
			return out, err
		}
	} else {
		out.DoneFlashBg = DefaultDoneFlashBg
	}
	out.DoneFlashDurationMs = DefaultDoneFlashDurationMs
	if t.DoneFlashDurationMs != nil {
		if *t.DoneFlashDurationMs < 100 || *t.DoneFlashDurationMs > 10000 {
			return out, fmt.Errorf("theme.done_flash_duration_ms must be between 100 and 10000")
		}
		out.DoneFlashDurationMs = *t.DoneFlashDurationMs
	}
	out.DoneFlashPulses = DefaultDoneFlashPulses
	if t.DoneFlashPulses != nil {
		if *t.DoneFlashPulses < 1 || *t.DoneFlashPulses > 10 {
			return out, fmt.Errorf("theme.done_flash_pulses must be between 1 and 10")
		}
		out.DoneFlashPulses = *t.DoneFlashPulses
	}
	out.NeedsInputGlow = boolOr(t.NeedsInputGlow, true)
	if out.NeedsInputColor, err = hexOr("theme.needs_input_color", t.NeedsInputColor, DefaultNeedsInputColor); err != nil {
		return out, err
	}
	if t.NeedsInputBg != nil {
		if *t.NeedsInputBg == "" {
			out.NeedsInputBg = ""
		} else if out.NeedsInputBg, err = NormalizeHexColor("theme.needs_input_bg", *t.NeedsInputBg); err != nil {
			return out, err
		}
	} else {
		out.NeedsInputBg = DefaultNeedsInputBg
	}
	return out, nil
}

// ResolveAlternateTint validates and resolves the alternating tint settings.
func ResolveAlternateTint(t ThemeConfig) (AlternateTintSettings, error) {
	out := AlternateTintSettings{Enabled: boolOr(t.AlternateTints, true)}
	bg, err := hexOr("theme.alternate_tint_bg", t.AlternateTintBg, DefaultAlternateTintBg)
	if err != nil {
		return out, err
	}
	out.Bg = bg
	return out, nil
}

// EffectiveAttention returns the attention settings, falling back to defaults.
func (c *Config) EffectiveAttention() AttentionSettings {
	if c == nil {
		s, _ := ResolveAttention(ThemeConfig{})
		return s
	}
	s, err := ResolveAttention(c.Theme)
	if err != nil {
		s, _ = ResolveAttention(ThemeConfig{})
	}
	return s
}

// EffectiveAlternateTint returns the alternating tint settings, falling back
// to defaults.
func (c *Config) EffectiveAlternateTint() AlternateTintSettings {
	if c == nil {
		s, _ := ResolveAlternateTint(ThemeConfig{})
		return s
	}
	s, err := ResolveAlternateTint(c.Theme)
	if err != nil {
		s, _ = ResolveAlternateTint(ThemeConfig{})
	}
	return s
}

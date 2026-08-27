package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/weill-labs/amux/internal/proto"
)

// catppuccinMocha is the accent palette in official order (catppuccin.com/palette).
// Unexported to prevent mutation; access via AccentColor/NumAccentColors.
var catppuccinMocha = [...]string{
	"f5e0dc", // Rosewater
	"f2cdcd", // Flamingo
	"f5c2e7", // Pink
	"cba6f7", // Mauve
	"f38ba8", // Red
	"eba0ac", // Maroon
	"fab387", // Peach
	"f9e2af", // Yellow
	"a6e3a1", // Green
	"94e2d5", // Teal
	"89dceb", // Sky
	"74c7ec", // Sapphire
	"89b4fa", // Blue
	"b4befe", // Lavender
}

// NumAccentColors is the number of colors in the Catppuccin Mocha accent palette.
const NumAccentColors = 14

// AccentColor returns the hex color at index i (mod palette size).
func AccentColor(i uint32) string {
	return catppuccinMocha[i%NumAccentColors]
}

// AccentColors returns a copy of the full palette.
func AccentColors() []string {
	out := make([]string, NumAccentColors)
	copy(out, catppuccinMocha[:])
	return out
}

// catppuccinLetters maps each hex color to a single-letter abbreviation
// for use in color map output (e.g., `amux capture --colors`).
var catppuccinLetters = map[string]byte{
	"f5e0dc": 'R', // Rosewater
	"f2cdcd": 'F', // Flamingo
	"f5c2e7": 'P', // Pink
	"cba6f7": 'M', // Mauve
	"f38ba8": 'E', // Red
	"eba0ac": 'A', // Maroon
	"fab387": 'H', // Peach
	"f9e2af": 'Y', // Yellow
	"a6e3a1": 'G', // Green
	"94e2d5": 'T', // Teal
	"89dceb": 'S', // Sky
	"74c7ec": 'B', // Sapphire
	"89b4fa": 'U', // Blue
	"b4befe": 'L', // Lavender
}

// AccentColorLetter returns the single-letter abbreviation for a hex color,
// or 0 if the color is not in the palette.
func AccentColorLetter(hex string) (byte, bool) {
	l, ok := catppuccinLetters[hex]
	return l, ok
}

// Named hex UI colors. These default to the Catppuccin Mocha palette and are
// replaced at client startup by ApplyPalette when [theme] preset/colors are
// configured. See theme.go.
var (
	DimColorHex  = "6c7086" // Overlay 0 — inactive/dim borders
	TextColorHex = "cdd6f4" // Text foreground
	Surface0Hex  = "313244" // Surface 0 — status bar background
	Surface1Hex  = "45475a" // Surface 1 — pressed status bar background
	BlueHex      = "89b4fa" // Blue — active tab highlight
	MauveHex     = "cba6f7" // Mauve — modal accent (selection bar, borders)
	FooterKeyHex = "858392" // muted gray-mauve — modal footer key names (matches crush)
	GreenHex     = "a6e3a1" // Green — status indicator
	YellowHex    = "f9e2af" // Yellow — status indicator
	PeachHex     = "fab387" // Peach — reconnecting mirror indicator
	RedHex       = "f38ba8" // Red — status indicator
)

type DebugConfig struct {
	Pprof bool `toml:"pprof"`
}

type ClientConfig struct {
	LocalEcho      string `toml:"local_echo"`
	LocalEchoStyle string `toml:"local_echo_style"`
	// MouseFocusClick controls what happens when a mouse button is pressed on
	// a pane that is not the active pane while that pane has application
	// mouse tracking enabled. "focus-only" (default) focuses the pane and
	// swallows the whole press/drag/release gesture so the app never sees a
	// click it did not "own". "forward" focuses and also forwards the click.
	MouseFocusClick string `toml:"mouse_focus_click"`
}

const (
	MouseFocusClickFocusOnly = "focus-only"
	MouseFocusClickForward   = "forward"
)

// ResolveMouseFocusClick validates client.mouse_focus_click.
func ResolveMouseFocusClick(mode string) (string, error) {
	switch mode {
	case "", MouseFocusClickFocusOnly:
		return MouseFocusClickFocusOnly, nil
	case MouseFocusClickForward:
		return mode, nil
	default:
		return "", fmt.Errorf(`client.mouse_focus_click must be one of "focus-only" or "forward"`)
	}
}

// EffectiveMouseFocusClick returns the resolved mouse_focus_click mode.
func (c *Config) EffectiveMouseFocusClick() string {
	if c == nil {
		return MouseFocusClickFocusOnly
	}
	mode, err := ResolveMouseFocusClick(c.Client.MouseFocusClick)
	if err != nil {
		return MouseFocusClickFocusOnly
	}
	return mode
}

const (
	StatusStyleCompact   = "compact"
	StatusStylePlain     = "plain"
	StatusStylePowerline = "powerline"

	ThemeIconsASCII   = "ascii"
	ThemeIconsUnicode = "unicode"
	ThemeIconsNerd    = "nerd"
)

// ThemeConfig controls client-side presentation.
//
// Example:
//
//	[theme]
//	icons = "unicode" # ascii | unicode | nerd
type ThemeConfig struct {
	StatusStyle string  `toml:"status_style"`
	Icons       *string `toml:"icons"`

	// Palette preset ("default" | "soft") and per-role overrides.
	Preset string      `toml:"preset"`
	Colors ThemeColors `toml:"colors"`

	// Done-flash: pulse a pane's border/title when its agent goes busy -> idle
	// while the pane is not focused. Defaults on.
	DoneFlash           *bool   `toml:"done_flash"`
	DoneFlashColor      string  `toml:"done_flash_color"`
	DoneFlashBg         *string `toml:"done_flash_bg"`
	DoneFlashDurationMs *int    `toml:"done_flash_duration_ms"`
	DoneFlashPulses     *int    `toml:"done_flash_pulses"`

	// Needs-input glow: steady tint on a pane that finished (went idle) and
	// has not been focused or typed into since. Defaults on.
	NeedsInputGlow  *bool   `toml:"needs_input_glow"`
	NeedsInputColor string  `toml:"needs_input_color"`
	NeedsInputBg    *string `toml:"needs_input_bg"`

	// Alternating tints: checkerboard background across adjacent panes.
	// Defaults on.
	AlternateTints  *bool  `toml:"alternate_tints"`
	AlternateTintBg string `toml:"alternate_tint_bg"`
}

type RemoteConfig struct {
	Hosts map[string]Host `toml:"hosts"`
}

type Host struct {
	SSH        string `toml:"ssh"`
	Session    string `toml:"session"`
	SocketPath string `toml:"socket_path"`
}

// LayoutConfig holds named-layout settings.
//
//	[layout.ultra]
//	rows = 2            # grid rows (default 2)
//	cols = 3            # grid columns (default 3) -> 6 cells: lead + 5 agent slots
//	auto_promote = true # bump idle hidden panes onto the visible page
type LayoutConfig struct {
	Ultra UltraConfig `toml:"ultra"`
}

// UltraConfig configures the ultra fixed-slot grid layout.
type UltraConfig struct {
	Rows        *int  `toml:"rows"`
	Cols        *int  `toml:"cols"`
	AutoPromote *bool `toml:"auto_promote"`
}

// Config is the top-level amux configuration.
type Config struct {
	ScrollbackLines *int         `toml:"scrollback_lines"`
	Debug           DebugConfig  `toml:"debug"`
	Client          ClientConfig `toml:"client"`
	Theme           ThemeConfig  `toml:"theme"`
	Layout          LayoutConfig `toml:"layout"`
	Remote          RemoteConfig `toml:"remote"`
}

// ResolveUltra validates [layout.ultra] and returns effective rows, cols and
// auto_promote (defaults 2, 3, true).
func ResolveUltra(u UltraConfig) (rows, cols int, autoPromote bool, err error) {
	rows, cols, autoPromote = 2, 3, true
	if u.Rows != nil {
		rows = *u.Rows
	}
	if u.Cols != nil {
		cols = *u.Cols
	}
	if u.AutoPromote != nil {
		autoPromote = *u.AutoPromote
	}
	if rows < 1 || rows > 8 {
		return 0, 0, false, fmt.Errorf("layout.ultra.rows must be 1..8")
	}
	if cols < 1 || cols > 8 {
		return 0, 0, false, fmt.Errorf("layout.ultra.cols must be 1..8")
	}
	if rows*cols < 2 {
		return 0, 0, false, fmt.Errorf("layout.ultra needs rows*cols >= 2")
	}
	return rows, cols, autoPromote, nil
}

// EffectiveUltra returns the resolved ultra grid settings.
func (c *Config) EffectiveUltra() (rows, cols int, autoPromote bool) {
	if c == nil {
		return 2, 3, true
	}
	rows, cols, autoPromote, err := ResolveUltra(c.Layout.Ultra)
	if err != nil {
		return 2, 3, true
	}
	return rows, cols, autoPromote
}

// DefaultPath returns the default config file path.
// Checks AMUX_CONFIG env var first, then ~/.config/amux/config.toml.
func DefaultPath() string {
	if p := os.Getenv("AMUX_CONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "amux", "config.toml")
}

// Load reads the config from the given path. Returns an empty config if the file doesn't exist.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	return parseConfig(data)
}

// Save writes cfg to path as TOML, creating the parent directory when needed.
func Save(path string, cfg *Config) error {
	if cfg == nil {
		cfg = &Config{}
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.toml")
	if err != nil {
		return fmt.Errorf("creating config temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing config temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing config temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replacing config: %w", err)
	}
	return nil
}

func parseConfig(data []byte) (*Config, error) {
	cfg := &Config{}

	md, err := toml.Decode(string(data), cfg)
	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if hasLegacyKeysConfig(md.Undecoded()) {
		return nil, fmt.Errorf(`unsupported config section "keys"`)
	}

	if _, err := ResolveScrollbackLines(cfg.ScrollbackLines); err != nil {
		return nil, err
	}
	if _, err := ResolveLocalEchoMode(cfg.Client.LocalEcho); err != nil {
		return nil, err
	}
	if _, err := ResolveLocalEchoStyle(cfg.Client.LocalEchoStyle); err != nil {
		return nil, err
	}
	if _, err := ResolveMouseFocusClick(cfg.Client.MouseFocusClick); err != nil {
		return nil, err
	}
	if _, err := ResolveStatusStyle(cfg.Theme.StatusStyle); err != nil {
		return nil, err
	}
	if _, err := ResolveThemeIcons(cfg.Theme.Icons); err != nil {
		return nil, err
	}
	if _, err := ResolvePalette(cfg.Theme.Preset, cfg.Theme.Colors); err != nil {
		return nil, err
	}
	if _, err := ResolveAttention(cfg.Theme); err != nil {
		return nil, err
	}
	if _, err := ResolveAlternateTint(cfg.Theme); err != nil {
		return nil, err
	}
	if _, _, _, err := ResolveUltra(cfg.Layout.Ultra); err != nil {
		return nil, err
	}
	if err := ValidateRemoteHosts(cfg.Remote.Hosts); err != nil {
		return nil, err
	}

	return cfg, nil
}

func hasLegacyKeysConfig(undecoded []toml.Key) bool {
	for _, key := range undecoded {
		if len(key) > 0 && key[0] == "keys" {
			return true
		}
	}
	return false
}

// ResolveScrollbackLines validates a configured scrollback limit and returns
// the effective value. Zero means "use the built-in default".
func ResolveScrollbackLines(lines *int) (int, error) {
	return resolveScrollbackLinesField("scrollback_lines", lines)
}

func resolveScrollbackLinesField(field string, lines *int) (int, error) {
	switch {
	case lines == nil:
		return proto.DefaultScrollbackLines, nil
	case *lines < 1:
		return 0, fmt.Errorf("%s must be >= 1", field)
	default:
		return *lines, nil
	}
}

// EffectiveScrollbackLines returns the resolved retained scrollback limit,
// falling back to the built-in default for nil configs or unset values.
func (c *Config) EffectiveScrollbackLines() int {
	if c == nil {
		return proto.DefaultScrollbackLines
	}
	lines, err := ResolveScrollbackLines(c.ScrollbackLines)
	if err != nil {
		return proto.DefaultScrollbackLines
	}
	return lines
}

func (c *Config) PprofEnabled() bool {
	return c != nil && c.Debug.Pprof
}

func ResolveLocalEchoMode(mode string) (string, error) {
	switch mode {
	case "", "auto":
		return "auto", nil
	case "off", "always":
		return mode, nil
	default:
		return "", fmt.Errorf(`local_echo must be one of "auto", "off", or "always"`)
	}
}

func ResolveLocalEchoStyle(style string) (string, error) {
	switch style {
	case "", "dim":
		return "dim", nil
	case "underline", "none":
		return style, nil
	default:
		return "", fmt.Errorf(`local_echo_style must be one of "dim", "underline", or "none"`)
	}
}

func ResolveStatusStyle(style string) (string, error) {
	switch style {
	case "", StatusStyleCompact:
		return StatusStyleCompact, nil
	case StatusStylePlain, StatusStylePowerline:
		return style, nil
	default:
		return "", fmt.Errorf(`status_style must be one of "compact", "plain", or "powerline"`)
	}
}

func (c *Config) EffectiveLocalEchoMode() string {
	if c == nil {
		return "auto"
	}
	mode, err := ResolveLocalEchoMode(c.Client.LocalEcho)
	if err != nil {
		return "auto"
	}
	return mode
}

func (c *Config) EffectiveLocalEchoStyle() string {
	if c == nil {
		return "dim"
	}
	style, err := ResolveLocalEchoStyle(c.Client.LocalEchoStyle)
	if err != nil {
		return "dim"
	}
	return style
}

func (c *Config) EffectiveStatusStyle() string {
	if c == nil {
		return StatusStyleCompact
	}
	style, err := ResolveStatusStyle(c.Theme.StatusStyle)
	if err != nil {
		return StatusStyleCompact
	}
	return style
}

func ResolveThemeIcons(icons *string) (string, error) {
	if icons == nil {
		return ThemeIconsUnicode, nil
	}
	switch *icons {
	case ThemeIconsASCII, ThemeIconsUnicode, ThemeIconsNerd:
		return *icons, nil
	default:
		return "", fmt.Errorf(`theme.icons must be one of "ascii", "unicode", or "nerd"`)
	}
}

func (c *Config) EffectiveThemeIcons() string {
	if c == nil {
		return ThemeIconsUnicode
	}
	icons, err := ResolveThemeIcons(c.Theme.Icons)
	if err != nil {
		return ThemeIconsUnicode
	}
	return icons
}

func ValidateRemoteHosts(hosts map[string]Host) error {
	names := make([]string, 0, len(hosts))
	for name := range hosts {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		host := hosts[name]
		ssh := strings.TrimSpace(host.SSH)
		socketPath := strings.TrimSpace(host.SocketPath)
		if ssh == "" {
			return fmt.Errorf("remote.hosts.%s.ssh is required", name)
		}
		if strings.HasPrefix(ssh, "-") {
			return fmt.Errorf("remote.hosts.%s.ssh must not start with '-'", name)
		}
		if strings.TrimSpace(host.Session) == "" {
			return fmt.Errorf("remote.hosts.%s.session is required", name)
		}
		if socketPath == "" {
			return fmt.Errorf("remote.hosts.%s.socket_path is required", name)
		}
		if !filepath.IsAbs(socketPath) {
			return fmt.Errorf("remote.hosts.%s.socket_path must be absolute", name)
		}
	}
	return nil
}

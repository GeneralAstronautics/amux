package config

import (
	"strings"
	"testing"
)

func TestParseConfigThemeDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := parseConfig([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	att := cfg.EffectiveAttention()
	if !att.DoneFlash || !att.NeedsInputGlow {
		t.Fatalf("expected done_flash and needs_input_glow on by default: %+v", att)
	}
	if att.DoneFlashColor != DefaultDoneFlashColor || att.DoneFlashPulses != DefaultDoneFlashPulses {
		t.Fatalf("unexpected defaults: %+v", att)
	}
	alt := cfg.EffectiveAlternateTint()
	if !alt.Enabled || alt.Bg != DefaultAlternateTintBg {
		t.Fatalf("unexpected alternate tint defaults: %+v", alt)
	}
	if cfg.EffectivePalette() != DefaultPalette() {
		t.Fatalf("expected default palette")
	}
}

func TestParseConfigThemeSoftPresetWithOverrides(t *testing.T) {
	t.Parallel()
	cfg, err := parseConfig([]byte(`
[theme]
preset = "soft"
done_flash = false
needs_input_color = "#FFAA00"
needs_input_bg = ""
alternate_tints = false
done_flash_pulses = 2
done_flash_duration_ms = 800

[theme.colors]
dim = "#101010"
accent_soften = 0.5
`))
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.EffectivePalette()
	if p.Dim != "101010" {
		t.Fatalf("dim override not applied: %+v", p)
	}
	if p.Surface0 != SoftPalette().Surface0 {
		t.Fatalf("soft preset not applied: %+v", p)
	}
	if p.AccentSoften != 0.5 {
		t.Fatalf("accent_soften override not applied: %+v", p)
	}
	att := cfg.EffectiveAttention()
	if att.DoneFlash || att.NeedsInputColor != "ffaa00" || att.NeedsInputBg != "" {
		t.Fatalf("unexpected attention settings: %+v", att)
	}
	if att.DoneFlashPulses != 2 || att.DoneFlashDurationMs != 800 {
		t.Fatalf("unexpected flash timing: %+v", att)
	}
	if cfg.EffectiveAlternateTint().Enabled {
		t.Fatalf("alternate_tints=false not honoured")
	}
}

func TestParseConfigThemeRejectsBadValues(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"preset":   "[theme]\npreset = \"neon\"\n",
		"color":    "[theme]\ndone_flash_color = \"red\"\n",
		"pulses":   "[theme]\ndone_flash_pulses = 0\n",
		"duration": "[theme]\ndone_flash_duration_ms = 50\n",
		"soften":   "[theme.colors]\naccent_soften = 2\n",
		"override": "[theme.colors]\nblue = \"#12345\"\n",
	}
	for name, body := range cases {
		if _, err := parseConfig([]byte(body)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestSoftenHex(t *testing.T) {
	t.Parallel()
	if got := SoftenHex("ff0000", 0); got != "ff0000" {
		t.Fatalf("amount 0 should be identity, got %s", got)
	}
	got := SoftenHex("ff0000", 1)
	if got != "4c4c4c" {
		t.Fatalf("fully softened pure red should be its luma grey, got %s", got)
	}
	half := SoftenHex("ff0000", 0.5)
	if !strings.HasPrefix(half, "a6") {
		t.Fatalf("unexpected half soften: %s", half)
	}
}

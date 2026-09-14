package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// useTheme applies a theme for one test and puts the default back afterwards,
// since styles are process-wide.
func useTheme(t *testing.T, name string, overrides map[string]string) error {
	t.Helper()
	t.Cleanup(func() {
		if err := ApplyTheme("default", nil); err != nil {
			t.Fatalf("restoring the default theme: %v", err)
		}
	})
	return ApplyTheme(name, overrides)
}

func TestApplyThemeUsesABuiltInPalette(t *testing.T) {
	if err := useTheme(t, "dracula", nil); err != nil {
		t.Fatalf("ApplyTheme: %v", err)
	}

	if got := currentPalette.Accent; got != lipgloss.Color("#bd93f9") {
		t.Errorf("accent = %v, want dracula's purple", got)
	}
	if got := currentPalette.Added; got != lipgloss.Color("#50fa7b") {
		t.Errorf("added = %v, want dracula's green", got)
	}
	// The styles themselves have to be rebuilt, not just the palette recorded.
	if !strings.Contains(styleBranch.Render("main"), "main") {
		t.Error("styles were not rebuilt")
	}
}

func TestApplyThemeWithNoNameKeepsTheDefault(t *testing.T) {
	if err := useTheme(t, "", nil); err != nil {
		t.Fatalf("ApplyTheme: %v", err)
	}
	if currentPalette.Accent != defaultPalette().Accent {
		t.Error("an empty theme name should leave the default in place")
	}
}

func TestApplyThemeOverridesIndividualColours(t *testing.T) {
	err := useTheme(t, "nord", map[string]string{
		"accent":  "#ff79c6",
		"warning": "#FFB86C",
	})
	if err != nil {
		t.Fatalf("ApplyTheme: %v", err)
	}

	if got := currentPalette.Accent; got != lipgloss.Color("#ff79c6") {
		t.Errorf("accent = %v, want the override", got)
	}
	if got := currentPalette.Warning; got != lipgloss.Color("#FFB86C") {
		t.Errorf("warning = %v, want the override", got)
	}
	// Roles that were not overridden keep the theme's own colour.
	if got := currentPalette.Added; got != lipgloss.Color("#a3be8c") {
		t.Errorf("added = %v, want nord's green", got)
	}
}

func TestApplyThemeRejectsAnUnknownTheme(t *testing.T) {
	err := useTheme(t, "solarized", nil)
	if err == nil {
		t.Fatal("an unknown theme should be an error")
	}
	// The message has to say what the user can pick instead.
	for _, want := range ThemeNames() {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to list %q", err, want)
		}
	}
}

func TestApplyThemeRejectsBadColours(t *testing.T) {
	cases := map[string]map[string]string{
		"a colour name":   {"accent": "blue"},
		"a palette index": {"accent": "39"},
		"a short hex":     {"accent": "#fff"},
		"an unknown role": {"highlight": "#ffffff"},
	}

	for name, overrides := range cases {
		t.Run(name, func(t *testing.T) {
			if err := useTheme(t, "default", overrides); err == nil {
				t.Errorf("%v should have been rejected", overrides)
			}
		})
	}
}

// A rejected theme must not leave the styles half-applied.
func TestApplyThemeLeavesStylesAloneOnError(t *testing.T) {
	if err := useTheme(t, "gruvbox", nil); err != nil {
		t.Fatalf("ApplyTheme: %v", err)
	}
	before := currentPalette

	if err := ApplyTheme("gruvbox", map[string]string{"accent": "not a colour"}); err == nil {
		t.Fatal("a bad colour should be an error")
	}
	if currentPalette != before {
		t.Error("a failed ApplyTheme changed the palette anyway")
	}
}

func TestThemeNamesAreStable(t *testing.T) {
	names := ThemeNames()
	if len(names) < 2 {
		t.Fatalf("ThemeNames() = %v, want several", names)
	}
	if names[0] != "default" {
		t.Errorf("ThemeNames() starts with %q; default should sort first", names[0])
	}
}

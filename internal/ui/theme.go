package ui

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Palette is the small set of colour roles every style is built from. Keeping
// it this short is deliberate: a theme should be a handful of decisions, not a
// mapping for each element on screen.
type Palette struct {
	// Text is ordinary foreground text.
	Text lipgloss.TerminalColor
	// Muted is secondary text: labels, hints, metadata.
	Muted lipgloss.TerminalColor
	// Border is the line around an unfocused pane.
	Border lipgloss.TerminalColor
	// Accent marks the current branch, the focused pane and the selected row.
	Accent lipgloss.TerminalColor
	// Added is an added line, a new file, a branch that is ahead.
	Added lipgloss.TerminalColor
	// Removed is a deleted line, a deleted file, an error.
	Removed lipgloss.TerminalColor
	// Warning is a dirty tree, a branch that is behind, an unfinished merge.
	Warning lipgloss.TerminalColor
	// Inverse is text drawn on top of Accent, Removed or Warning.
	Inverse lipgloss.TerminalColor
}

// defaultPalette adapts to the terminal's own background, which is why it is
// the default: it looks deliberate on a light and a dark terminal alike.
func defaultPalette() Palette {
	return Palette{
		Text:    lipgloss.AdaptiveColor{Light: "236", Dark: "252"},
		Muted:   lipgloss.AdaptiveColor{Light: "245", Dark: "243"},
		Border:  lipgloss.AdaptiveColor{Light: "252", Dark: "238"},
		Accent:  lipgloss.AdaptiveColor{Light: "25", Dark: "39"},
		Added:   lipgloss.AdaptiveColor{Light: "28", Dark: "78"},
		Removed: lipgloss.AdaptiveColor{Light: "124", Dark: "203"},
		Warning: lipgloss.AdaptiveColor{Light: "130", Dark: "215"},
		Inverse: lipgloss.AdaptiveColor{Light: "255", Dark: "235"},
	}
}

// themes are the palettes that ship with tuigy. They commit to a single set of
// colours rather than adapting, because that is the point of choosing one.
var themes = map[string]Palette{
	"default": defaultPalette(),

	"dracula": {
		Text:    lipgloss.Color("#f8f8f2"),
		Muted:   lipgloss.Color("#6272a4"),
		Border:  lipgloss.Color("#44475a"),
		Accent:  lipgloss.Color("#bd93f9"),
		Added:   lipgloss.Color("#50fa7b"),
		Removed: lipgloss.Color("#ff5555"),
		Warning: lipgloss.Color("#ffb86c"),
		Inverse: lipgloss.Color("#282a36"),
	},

	"nord": {
		Text:    lipgloss.Color("#d8dee9"),
		Muted:   lipgloss.Color("#4c566a"),
		Border:  lipgloss.Color("#3b4252"),
		Accent:  lipgloss.Color("#88c0d0"),
		Added:   lipgloss.Color("#a3be8c"),
		Removed: lipgloss.Color("#bf616a"),
		Warning: lipgloss.Color("#ebcb8b"),
		Inverse: lipgloss.Color("#2e3440"),
	},

	"gruvbox": {
		Text:    lipgloss.Color("#ebdbb2"),
		Muted:   lipgloss.Color("#928374"),
		Border:  lipgloss.Color("#504945"),
		Accent:  lipgloss.Color("#83a598"),
		Added:   lipgloss.Color("#b8bb26"),
		Removed: lipgloss.Color("#fb4934"),
		Warning: lipgloss.Color("#fabd2f"),
		Inverse: lipgloss.Color("#282828"),
	},
}

// ThemeNames lists the built-in themes, sorted so help text is stable.
func ThemeNames() []string {
	names := make([]string, 0, len(themes))
	for name := range themes {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// colourNames are the roles a config file may override, in the order they are
// reported back to the user.
var colourNames = []string{"text", "muted", "border", "accent", "added", "removed", "warning", "inverse"}

// hexColour is the only colour spelling accepted from a config file. Terminal
// palette indexes are deliberately not accepted: they mean something different
// in every terminal, which is exactly what a theme is trying to pin down.
var hexColour = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// ApplyTheme rebuilds every style from a named theme and any overrides on top.
//
// It is called once at startup, before anything is rendered.
func ApplyTheme(name string, overrides map[string]string) error {
	if name == "" {
		name = "default"
	}

	palette, ok := themes[name]
	if !ok {
		return fmt.Errorf("unknown theme %q; available: %s", name, strings.Join(ThemeNames(), ", "))
	}

	for role, value := range overrides {
		if !hexColour.MatchString(value) {
			return fmt.Errorf("colour %s: %q is not a hex colour such as #bd93f9", role, value)
		}

		colour := lipgloss.Color(value)
		switch role {
		case "text":
			palette.Text = colour
		case "muted":
			palette.Muted = colour
		case "border":
			palette.Border = colour
		case "accent":
			palette.Accent = colour
		case "added":
			palette.Added = colour
		case "removed":
			palette.Removed = colour
		case "warning":
			palette.Warning = colour
		case "inverse":
			palette.Inverse = colour
		default:
			return fmt.Errorf("unknown colour %q; available: %s", role, strings.Join(colourNames, ", "))
		}
	}

	applyPalette(palette)
	return nil
}

// Every style in the package. They are rebuilt as a set so that a theme is
// applied atomically and no style is ever left over from the previous one.
var (
	styleBase lipgloss.Style
	styleDim  lipgloss.Style

	styleRepoName lipgloss.Style
	styleBranch   lipgloss.Style
	styleAhead    lipgloss.Style
	styleBehind   lipgloss.Style
	styleDirty    lipgloss.Style
	styleOpState  lipgloss.Style

	styleSection  lipgloss.Style
	styleSelected lipgloss.Style

	styleAdded   lipgloss.Style
	styleRemoved lipgloss.Style
	styleHunk    lipgloss.Style
	styleMeta    lipgloss.Style

	styleError lipgloss.Style
	styleFlash lipgloss.Style

	styleKey  lipgloss.Style
	styleDesc lipgloss.Style

	styleBorder      lipgloss.Style
	styleBorderFocus lipgloss.Style
	styleModal       lipgloss.Style
	styleTitle       lipgloss.Style

	styleTab       lipgloss.Style
	styleTabActive lipgloss.Style
)

func init() { applyPalette(defaultPalette()) }

// currentPalette is what the styles were last built from. Reading it is how the
// tests check a theme without having to inspect rendered escape sequences.
var currentPalette Palette

func applyPalette(p Palette) {
	currentPalette = p

	styleBase = lipgloss.NewStyle().Foreground(p.Text)
	styleDim = lipgloss.NewStyle().Foreground(p.Muted)

	styleRepoName = lipgloss.NewStyle().Foreground(p.Text).Bold(true)
	styleBranch = lipgloss.NewStyle().Foreground(p.Accent).Bold(true)
	styleAhead = lipgloss.NewStyle().Foreground(p.Added)
	styleBehind = lipgloss.NewStyle().Foreground(p.Warning)
	styleDirty = lipgloss.NewStyle().Foreground(p.Warning)
	styleOpState = lipgloss.NewStyle().Foreground(p.Inverse).Background(p.Warning).Bold(true).Padding(0, 1)

	styleSection = lipgloss.NewStyle().Foreground(p.Muted).Bold(true)
	styleSelected = lipgloss.NewStyle().Foreground(p.Inverse).Background(p.Accent)

	styleAdded = lipgloss.NewStyle().Foreground(p.Added)
	styleRemoved = lipgloss.NewStyle().Foreground(p.Removed)
	styleHunk = lipgloss.NewStyle().Foreground(p.Accent)
	styleMeta = lipgloss.NewStyle().Foreground(p.Muted)

	styleError = lipgloss.NewStyle().Foreground(p.Inverse).Background(p.Removed).Bold(true).Padding(0, 1)
	styleFlash = lipgloss.NewStyle().Foreground(p.Added)

	styleKey = lipgloss.NewStyle().Foreground(p.Accent).Bold(true)
	styleDesc = lipgloss.NewStyle().Foreground(p.Muted)

	// The focused pane's border is accented; focus must always be visible.
	styleBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.Border)
	styleBorderFocus = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.Accent)

	styleModal = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.Accent).Padding(1, 2)
	styleTitle = lipgloss.NewStyle().Foreground(p.Text).Bold(true)

	styleTab = lipgloss.NewStyle().Foreground(p.Muted).Padding(0, 1)
	styleTabActive = lipgloss.NewStyle().Foreground(p.Inverse).Background(p.Accent).Bold(true).Padding(0, 1)
}

func paneStyle(focused bool) lipgloss.Style {
	if focused {
		return styleBorderFocus
	}
	return styleBorder
}

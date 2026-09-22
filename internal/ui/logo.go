package ui

import "github.com/charmbracelet/lipgloss"

// The wordmark, drawn in half blocks.
//
// It is the same mark as docs/logo.png rather than a second drawing of it: the
// rows below were sampled from that file, two vertical pixels to a character,
// which is what makes a wide pixel font legible in a grid of terminal cells.
// Redrawing it by hand would have produced something that drifts from the logo
// the moment either is touched.
//
// It is deliberately not in a colour. Themes here are a palette of roles rather
// than a set of colours — see docs/decisions/0009 — so a brand purple written
// into the art would be the one thing on screen that ignores the theme.
const logoArt = `` +
	" ▄███████████████▄\n" +
	"██▀ ▄▄▄▄ ▄▄     ▀██    ▄▄              ▄▄▄\n" +
	"██  ▀  ▀ ▀▀      ██  ▄▄██▄▄▄ ▄▄▄   ▄▄  ▀▀▀   ▄▄▄▄▄▄▄ ▄▄   ▄▄\n" +
	"██  █▄           ██  ▀▀██▀▀▀ ███   ██ ▀███  ███▀▀███ ██   ██\n" +
	"██   ██▄         ██    ██    ███   ██  ███  ███▄▄███ ██▄▄▄██\n" +
	"██  ██▀ ██████   ██    ▀████ ▀██████▀ █████  ▀▀▀▀███  ▀▀▀▀██\n" +
	"▀█▄▄▄▄▄▄▄▄▄▄▄▄▄▄██▀     ▀▀▀▀  ▀▀▀▀▀▀  ▀▀▀▀▀ ▄██████▀  █████▀\n" +
	"  ▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀                           ▀▀▀▀▀▀    ▀▀▀▀"

const (
	// logoWidth and logoHeight are what the art needs. Below either, whatever
	// would be drawn is a smear rather than a wordmark, and the name in plain
	// text says more.
	logoWidth  = 60
	logoHeight = 8
)

// logoView is the wordmark in the theme's title role.
//
// Whether there is room for it at all is each caller's question, because what
// to show instead differs: the splash has a line of its own to fall back to and
// the help screen has a heading. Nothing here is worth a broken layout.
func logoView() string { return styleTitle.Render(logoArt) }

// splashView is the screen tuigy shows while it is reading the repository for
// the first time, which is the one moment it has nothing else to say.
//
// It is the first paint only. Switching projects empties the same fields, and a
// mark thrown over the interface every time someone changes repository would be
// a splash screen in the way people rightly complain about.
func (m Model) splashView() string {
	if m.width < logoWidth || m.height < logoHeight+4 {
		return "loading tuigy…"
	}

	caption := styleDim.Render("reading the repository…")

	// Centred as one block, so the caption sits under the middle of the mark
	// rather than under the middle of the screen.
	block := lipgloss.JoinVertical(lipgloss.Center, logoView(), "", caption)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, block)
}

// helpHeading is the wordmark at the top of the help screen, or the title it
// replaces on a terminal too narrow to draw it.
//
// The help screen is the one place in tuigy that is opened to be read rather
// than worked in, and it scrolls, so the rows cost nothing that matters.
func (m Model) helpHeading() string {
	if m.help.Width < logoWidth {
		return styleTitle.Render("Shortcuts")
	}
	return logoView()
}

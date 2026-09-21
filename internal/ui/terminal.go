package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/mustafakarakulak/tuigy/internal/term"
)

// The terminal is a band across the bottom of the body, under both panes rather
// than in place of either: running something here is worth doing because what it
// does to the files already on screen is visible in the same frame.
//
// It is also the one place in tuigy where a keystroke does not mean what the key
// map says it means. While the shell has focus every key belongs to the child,
// ctrl+c included, because interrupting what is running there is the entire point
// of having it. Exactly one keystroke is reserved — Detach — and the footer shows
// nothing else for as long as the shell is focused.
//
// See docs/decisions/0016-the-terminal-is-a-band.md.

type (
	// termStartedMsg reports a shell that has been launched, or why it was not.
	termStartedMsg struct {
		session *term.Session
		err     error
	}
	// termOutputMsg says the screen has changed. It carries the session it came
	// from so that a signal from one the user has already closed is dropped
	// rather than redrawing a pane that is gone.
	termOutputMsg struct{ session *term.Session }
	termExitedMsg struct{ session *term.Session }
)

// openTerminal shows the shell pane and gives it the keyboard.
//
// A session already running is only refocused: the point of the pane is that
// what you left running is still there when you come back to it.
func (m Model) openTerminal() (tea.Model, tea.Cmd) {
	if m.shell != nil {
		m.termFocus = true
		return m, nil
	}

	// Started at the size it will be drawn at, so the shell's first prompt is
	// already laid out for the pane rather than reflowed a moment later.
	bodyH := max(m.height-headerHeight-footerHeight, 3)
	_, box := splitBody(bodyH, true)

	root, w, h := m.repo.Root, max(m.width-2, 1), max(box-2, 1)
	return m, func() tea.Msg {
		session, err := term.Start(root, w, h)
		return termStartedMsg{session: session, err: err}
	}
}

// waitForTerminal blocks until the shell has something new to show or has left.
func waitForTerminal(s *term.Session) tea.Cmd {
	return func() tea.Msg {
		select {
		case <-s.Updates():
			return termOutputMsg{session: s}
		case <-s.Done():
			return termExitedMsg{session: s}
		}
	}
}

// closeTerminal ends the session and gives its rows back to the panes above.
func (m *Model) closeTerminal() {
	if m.shell == nil {
		return
	}
	session := m.shell
	m.shell, m.termFocus = nil, false
	// The panes above take back the rows the shell was drawn in.
	m.layout()

	// Closed in the background: the shell is given a moment to leave on its own
	// and the interface has no reason to wait for it.
	go func() { _ = session.Close() }()
}

// Shutdown ends anything the model started that outlives the interface.
//
// The shell would be hung up anyway when the process exits and the
// pseudo-terminal is closed, but waiting for it means tuigy does not hand the
// terminal back before its child has let go of it.
func (m Model) Shutdown() {
	if m.shell != nil {
		_ = m.shell.Close()
	}
}

// handleTerminalKey routes a keystroke to the shell, or takes the keyboard back.
func (m Model) handleTerminalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Detach) {
		m.termFocus = false
		return m, nil
	}

	// A shell that has exited is about to take the band with it; until that
	// message arrives there is nothing to send a keystroke to.
	if m.shell.Exited() {
		return m, nil
	}

	if text, ok := literalText(msg); ok {
		m.shell.SendText(text)
		return m, nil
	}
	if event, ok := keyEvent(msg); ok {
		m.shell.SendKey(event)
	}
	return m, nil
}

// literalText is the text a keypress types, for the keys that type something.
//
// Runes go through as text rather than as key events so that a pasted line
// arrives in one piece and a multi-byte character is never split.
func literalText(msg tea.KeyMsg) (string, bool) {
	if msg.Alt {
		return "", false // alt is a modifier, not something that types
	}
	switch msg.Type {
	case tea.KeyRunes:
		return string(msg.Runes), true
	case tea.KeySpace:
		return " ", true
	default:
		return "", false
	}
}

// keyEvent translates a bubbletea keypress into the event the emulator encodes.
//
// The encoding is left to the emulator because it depends on modes the child
// sets: an arrow key is "\x1b[A" normally and "\x1bOA" to a program that has
// asked for application cursor keys, and getting that wrong breaks the arrow
// keys in vim and in a shell's own line editor.
func keyEvent(msg tea.KeyMsg) (uv.KeyPressEvent, bool) {
	var pressed uv.Key

	switch {
	case msg.Type == tea.KeyRunes:
		if len(msg.Runes) == 0 {
			return uv.KeyPressEvent{}, false
		}
		pressed = uv.Key{Code: msg.Runes[0], Text: string(msg.Runes)}

	case msg.Type == tea.KeySpace:
		pressed = uv.Key{Code: uv.KeySpace, Text: " "}

	default:
		named, ok := namedKeys[msg.Type]
		if ok {
			pressed = named
			break
		}
		// Everything left is a C0 control byte, whose value is already what
		// the wire carries: 0x01 is ctrl+a, 0x1c is ctrl+backslash.
		code, ok := controlKey(msg.Type)
		if !ok {
			return uv.KeyPressEvent{}, false
		}
		pressed = uv.Key{Code: code, Mod: uv.ModCtrl}
	}

	if msg.Alt {
		pressed.Mod |= uv.ModAlt
	}
	return uv.KeyPressEvent(pressed), true
}

// controlKey turns a C0 control byte into the key that produces it.
func controlKey(t tea.KeyType) (rune, bool) {
	b := rune(t)
	switch {
	case b == 0:
		return uv.KeySpace, true // ctrl+@, which is NUL
	case b >= 1 && b <= 26:
		return 'a' + b - 1, true
	case b >= 28 && b <= 31:
		// ctrl+\ ] ^ _ sit four below their printable characters.
		return b + 64, true
	default:
		return 0, false
	}
}

// namedKeys are the keys that are not a character and not a control byte.
// Written out rather than derived, for the same reason the key bindings are.
var namedKeys = map[tea.KeyType]uv.Key{
	tea.KeyEnter:     {Code: uv.KeyEnter},
	tea.KeyTab:       {Code: uv.KeyTab},
	tea.KeyShiftTab:  {Code: uv.KeyTab, Mod: uv.ModShift},
	tea.KeyBackspace: {Code: uv.KeyBackspace},
	tea.KeyDelete:    {Code: uv.KeyDelete},
	tea.KeyEsc:       {Code: uv.KeyEscape},
	tea.KeyInsert:    {Code: uv.KeyInsert},

	tea.KeyUp:     {Code: uv.KeyUp},
	tea.KeyDown:   {Code: uv.KeyDown},
	tea.KeyLeft:   {Code: uv.KeyLeft},
	tea.KeyRight:  {Code: uv.KeyRight},
	tea.KeyHome:   {Code: uv.KeyHome},
	tea.KeyEnd:    {Code: uv.KeyEnd},
	tea.KeyPgUp:   {Code: uv.KeyPgUp},
	tea.KeyPgDown: {Code: uv.KeyPgDown},

	tea.KeyCtrlUp:     {Code: uv.KeyUp, Mod: uv.ModCtrl},
	tea.KeyCtrlDown:   {Code: uv.KeyDown, Mod: uv.ModCtrl},
	tea.KeyCtrlLeft:   {Code: uv.KeyLeft, Mod: uv.ModCtrl},
	tea.KeyCtrlRight:  {Code: uv.KeyRight, Mod: uv.ModCtrl},
	tea.KeyCtrlHome:   {Code: uv.KeyHome, Mod: uv.ModCtrl},
	tea.KeyCtrlEnd:    {Code: uv.KeyEnd, Mod: uv.ModCtrl},
	tea.KeyCtrlPgUp:   {Code: uv.KeyPgUp, Mod: uv.ModCtrl},
	tea.KeyCtrlPgDown: {Code: uv.KeyPgDown, Mod: uv.ModCtrl},

	tea.KeyShiftUp:    {Code: uv.KeyUp, Mod: uv.ModShift},
	tea.KeyShiftDown:  {Code: uv.KeyDown, Mod: uv.ModShift},
	tea.KeyShiftLeft:  {Code: uv.KeyLeft, Mod: uv.ModShift},
	tea.KeyShiftRight: {Code: uv.KeyRight, Mod: uv.ModShift},
	tea.KeyShiftHome:  {Code: uv.KeyHome, Mod: uv.ModShift},
	tea.KeyShiftEnd:   {Code: uv.KeyEnd, Mod: uv.ModShift},

	tea.KeyCtrlShiftUp:    {Code: uv.KeyUp, Mod: uv.ModCtrl | uv.ModShift},
	tea.KeyCtrlShiftDown:  {Code: uv.KeyDown, Mod: uv.ModCtrl | uv.ModShift},
	tea.KeyCtrlShiftLeft:  {Code: uv.KeyLeft, Mod: uv.ModCtrl | uv.ModShift},
	tea.KeyCtrlShiftRight: {Code: uv.KeyRight, Mod: uv.ModCtrl | uv.ModShift},
	tea.KeyCtrlShiftHome:  {Code: uv.KeyHome, Mod: uv.ModCtrl | uv.ModShift},
	tea.KeyCtrlShiftEnd:   {Code: uv.KeyEnd, Mod: uv.ModCtrl | uv.ModShift},

	tea.KeyF1:  {Code: uv.KeyF1},
	tea.KeyF2:  {Code: uv.KeyF2},
	tea.KeyF3:  {Code: uv.KeyF3},
	tea.KeyF4:  {Code: uv.KeyF4},
	tea.KeyF5:  {Code: uv.KeyF5},
	tea.KeyF6:  {Code: uv.KeyF6},
	tea.KeyF7:  {Code: uv.KeyF7},
	tea.KeyF8:  {Code: uv.KeyF8},
	tea.KeyF9:  {Code: uv.KeyF9},
	tea.KeyF10: {Code: uv.KeyF10},
	tea.KeyF11: {Code: uv.KeyF11},
	tea.KeyF12: {Code: uv.KeyF12},
}

// ---------------------------------------------------------------- view

// terminalPane draws the shell screen inside the detail pane.
func (m Model) terminalPane() string {
	if m.shell == nil {
		return ""
	}

	screen := m.shell.Read()
	lines := make([]string, 0, len(screen.Lines))
	for y, line := range screen.Lines {
		if m.termFocus && screen.CursorVisible && y == screen.CursorY {
			line = withCursor(line, screen.CursorX, m.termW)
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// withCursor draws a block cursor over one cell of a rendered row.
//
// The row already carries the child's own colours, so the cell is cut out by
// visible width rather than by byte — cutting a row of ANSI text at a rune
// offset would slice an escape sequence in half.
func withCursor(line string, x, width int) string {
	if x < 0 || x >= width {
		return line
	}

	// Past the end of what was printed there is nothing to invert, so the row
	// is padded out to the cursor first.
	if w := ansi.StringWidth(line); w < x+1 {
		line += strings.Repeat(" ", x+1-w)
	}

	cell := ansi.Cut(line, x, x+1)
	if ansi.Strip(cell) == "" {
		cell = " "
	}
	return ansi.Truncate(line, x, "") + styleSelected.Render(ansi.Strip(cell)) + ansi.Cut(line, x+1, width)
}

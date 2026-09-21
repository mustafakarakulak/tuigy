package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mustafakarakulak/tuigy/internal/config"
	"github.com/mustafakarakulak/tuigy/internal/keys"
)

// Settings has two pages, because there turned out to be two things worth
// changing and only one of them was reachable.
//
// The theme page applies as the cursor moves: a colour scheme is not something
// anyone can judge from its name. The keys page is the opposite — a binding is
// judged by reading it — so it changes only what you ask it to, one action at a
// time, and writes each change to the configuration file as it is made.

type settingsPage int

const (
	pageTheme settingsPage = iota
	pageKeys
)

var settingsPages = []settingsPage{pageTheme, pageKeys}

func (p settingsPage) title() string {
	if p == pageTheme {
		return "Theme"
	}
	return "Key bindings"
}

// keysSavedMsg reports a binding written to the configuration file.
type keysSavedMsg struct {
	action string
	path   string
	err    error
}

// keyRow is one action as the settings list shows it.
type keyRow struct {
	action  string
	stroke  string
	desc    string
	changed bool
}

func (m Model) openSettings() (tea.Model, tea.Cmd) {
	m.err = nil
	m.themeCursor = max(slices.Index(ThemeNames(), m.theme), 0)
	m.settingsPage = pageTheme
	m.keyCursor, m.keyOff = 0, 0
	m.keyFilter, m.keyFiltering, m.capturing = "", false, false
	m.keyNote = ""
	m.modal = modalSettings
	return m, nil
}

func (m Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Capturing comes first: the whole point is that the next keystroke is
	// taken as a keystroke rather than as whatever it usually does.
	if m.capturing {
		return m.captureBinding(msg)
	}
	if m.keyFiltering {
		return m.handleKeyFilter(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Cancel):
		// Leaving puts back the theme the user actually had. A binding is not
		// reverted, because it was already written when it was chosen.
		m.modal = modalNone
		return m, m.previewTheme(m.theme)

	case key.Matches(msg, m.keys.NextPane):
		m.settingsPage = nextPage(m.settingsPage, 1)
		m.keyNote = ""
		return m, nil

	case key.Matches(msg, m.keys.PrevPane):
		m.settingsPage = nextPage(m.settingsPage, -1)
		m.keyNote = ""
		return m, nil
	}

	if m.settingsPage == pageKeys {
		return m.handleKeysPage(msg)
	}
	return m.handleThemePage(msg)
}

func nextPage(p settingsPage, delta int) settingsPage {
	return settingsPages[(int(p)+delta+len(settingsPages))%len(settingsPages)]
}

// ---------------------------------------------------------------- theme page

func (m Model) handleThemePage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	names := ThemeNames()

	switch {
	case key.Matches(msg, m.keys.Up):
		m.themeCursor = max(m.themeCursor-1, 0)
		return m, m.previewTheme(names[m.themeCursor])

	case key.Matches(msg, m.keys.Down):
		m.themeCursor = min(m.themeCursor+1, len(names)-1)
		return m, m.previewTheme(names[m.themeCursor])

	case key.Matches(msg, m.keys.Confirm):
		chosen := names[m.themeCursor]
		m.theme = chosen
		m.modal = modalNone

		return m, func() tea.Msg {
			path, err := config.SetTheme(chosen)
			return themeSavedMsg{path: path, err: err}
		}
	}

	return m, nil
}

// previewTheme repaints everything in a theme without recording it as the
// choice, so moving the cursor shows what a theme actually looks like.
func (m Model) previewTheme(name string) tea.Cmd {
	if err := ApplyTheme(name, nil); err != nil {
		return func() tea.Msg { return errMsg{err} }
	}
	return nil
}

// ---------------------------------------------------------------- keys page

func (m Model) handleKeysPage(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.keyRows()

	switch {
	case key.Matches(msg, m.keys.Up):
		return m.moveKeyCursor(rows, -1), nil

	case key.Matches(msg, m.keys.Down):
		return m.moveKeyCursor(rows, 1), nil

	case key.Matches(msg, m.keys.PageUp):
		return m.moveKeyCursor(rows, -keysPerPage), nil

	case key.Matches(msg, m.keys.PageDown):
		return m.moveKeyCursor(rows, keysPerPage), nil

	case key.Matches(msg, m.keys.Filter):
		m.keyFiltering = true
		m.keyNote = ""
		return m, nil

	case key.Matches(msg, m.keys.Confirm):
		if len(rows) == 0 {
			return m, nil
		}
		m.capturing = true
		m.keyNote = ""
		return m, nil

	case key.Matches(msg, resetBindingKey()):
		return m.resetBinding(rows)
	}

	return m, nil
}

func (m Model) moveKeyCursor(rows []keyRow, delta int) Model {
	if len(rows) == 0 {
		return m
	}
	m.keyCursor = clamp(m.keyCursor+delta, 0, len(rows)-1)
	m.keyOff = scrollTo(m.keyCursor, m.keyOff, keysPerPage)
	m.keyNote = ""
	return m
}

func (m Model) handleKeyFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.keyFilter, m.keyFiltering = "", false
		m.keyCursor, m.keyOff = 0, 0
		return m, nil

	case key.Matches(msg, m.keys.Confirm):
		m.keyFiltering = false
		return m, nil
	}

	switch msg.Type {
	case tea.KeyBackspace:
		runes := []rune(m.keyFilter)
		if len(runes) > 0 {
			m.keyFilter = string(runes[:len(runes)-1])
		}
	case tea.KeySpace:
		m.keyFilter += " "
	case tea.KeyRunes:
		m.keyFilter += string(msg.Runes)
	default:
		return m, nil
	}

	m.keyCursor, m.keyOff = 0, 0
	return m, nil
}

// captureBinding takes the keystroke the user just pressed and gives it to the
// selected action.
//
// Escape is the one keystroke that cannot be captured, because it is the only
// way out of a mode where every other key is being swallowed. The configuration
// file can still rebind it.
func (m Model) captureBinding(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.capturing = false

	if key.Matches(msg, m.keys.Cancel) || msg.Type == tea.KeyEsc {
		return m, nil
	}

	rows := m.keyRows()
	if m.keyCursor < 0 || m.keyCursor >= len(rows) {
		return m, nil
	}
	action := rows[m.keyCursor].action

	stroke := keyName(msg)
	if stroke == "ctrl+c" {
		// ctrl+c quits before the key map is consulted, so binding it here
		// would produce a shortcut that silently never fires.
		m.keyNote = "ctrl+c always quits, and cannot be given to anything else"
		return m, nil
	}

	// Applied to the running key map first, so the footer and the help screen
	// describe the new binding immediately rather than after a restart.
	updated := m.keys
	if err := updated.Set(action, []string{stroke}); err != nil {
		m.err = err
		return m, nil
	}
	m.keys = updated

	m.keyNote = conflictNote(&m.keys, stroke, action)
	m.err = nil

	return m, func() tea.Msg {
		path, err := config.SetKeyBinding(action, []string{stroke})
		return keysSavedMsg{action: action, path: path, err: err}
	}
}

// resetBindingKey belongs to this dialog rather than to the key map: "r" has
// nothing else to do here, and an action for it would itself be one more thing
// to bind.
func resetBindingKey() key.Binding {
	return key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "back to the default key"))
}

// resetBinding puts the selected action back to the key it ships with.
func (m Model) resetBinding(rows []keyRow) (tea.Model, tea.Cmd) {
	if m.keyCursor < 0 || m.keyCursor >= len(rows) {
		return m, nil
	}
	row := rows[m.keyCursor]
	if !row.changed {
		m.keyNote = row.action + " is already on its default key"
		return m, nil
	}

	updated := m.keys
	if err := updated.Reset(row.action); err != nil {
		m.err = err
		return m, nil
	}
	m.keys = updated
	m.keyNote, m.err = "", nil

	return m, func() tea.Msg {
		path, err := config.ResetKeyBinding(row.action)
		return keysSavedMsg{action: row.action, path: path, err: err}
	}
}

// keyName is how a keystroke is written in a configuration file.
func keyName(msg tea.KeyMsg) string {
	if stroke := msg.String(); stroke != " " {
		return stroke
	}
	return "space"
}

// conflictNote names the other actions a keystroke is already on.
//
// Sharing is not an error: enter checks out a branch, pops a stash and confirms
// a dialog, and no two of those are ever on screen together. Saying so is still
// worth doing, because the one case that is a mistake looks identical until you
// are told.
func conflictNote(km *keys.Map, stroke, action string) string {
	others := km.Conflicts(stroke, action)
	if len(others) == 0 {
		return ""
	}
	if len(others) > 3 {
		others = append(others[:3], fmt.Sprintf("and %d more", len(others)-3))
	}
	return stroke + " is also " + strings.Join(others, ", ")
}

// keyRows lists the actions the settings page is showing.
func (m Model) keyRows() []keyRow {
	km := m.keys
	rows := make([]keyRow, 0, len(keys.ActionNames()))

	for _, action := range keys.ActionNames() {
		binding, ok := km.Get(action)
		if !ok {
			continue
		}
		desc := binding.Help().Desc
		if !matchesFilter(m.keyFilter, action) && !matchesFilter(m.keyFilter, desc) {
			continue
		}

		rows = append(rows, keyRow{
			action:  action,
			stroke:  strings.Join(displayStrokes(binding.Keys()), ", "),
			desc:    desc,
			changed: !km.IsDefault(action),
		})
	}
	return rows
}

// displayStrokes writes keystrokes the way the help text does, where a literal
// space would be invisible.
func displayStrokes(strokes []string) []string {
	out := make([]string, 0, len(strokes))
	for _, s := range strokes {
		if s == " " {
			s = "space"
		}
		out = append(out, s)
	}
	return out
}

// settingsHints is what the footer offers, which depends on the page and on
// whether a keystroke is being waited for.
func (m Model) settingsHints() hintSet {
	if m.capturing {
		// Every key is about to be swallowed; the only one worth advertising is
		// the one that is not.
		return hintSet{nil, []key.Binding{m.keys.Cancel}}
	}
	if m.keyFiltering {
		return hintSet{[]key.Binding{m.keys.Confirm}, []key.Binding{m.keys.Cancel}}
	}

	page := key.NewBinding(
		key.WithKeys(m.keys.NextPane.Keys()...),
		key.WithHelp(m.keys.NextPane.Help().Key, nextPage(m.settingsPage, 1).title()),
	)

	if m.settingsPage == pageKeys {
		rebind := key.NewBinding(
			key.WithKeys(m.keys.Confirm.Keys()...),
			key.WithHelp(m.keys.Confirm.Help().Key, "change this key"),
		)
		return hintSet{
			[]key.Binding{rebind, resetBindingKey(), m.keys.Filter,
				m.keys.Down, m.keys.Up, page},
			[]key.Binding{m.keys.Cancel},
		}
	}

	use := key.NewBinding(
		key.WithKeys(m.keys.Confirm.Keys()...),
		key.WithHelp(m.keys.Confirm.Help().Key, "use this theme"),
	)
	return hintSet{[]key.Binding{m.keys.Down, m.keys.Up, use, page},
		[]key.Binding{m.keys.Cancel}}
}

// ---------------------------------------------------------------- view

// keysPerPage is how many actions the list shows at once. It is fixed rather
// than derived from the height so that the cursor does not jump when the dialog
// is drawn on a different terminal.
const keysPerPage = 12

func (m Model) settingsBox() string {
	lines := []string{
		styleTitle.Render("Settings"),
		m.settingsTabs(),
		"",
	}

	if m.settingsPage == pageKeys {
		return strings.Join(append(lines, m.keysPage()...), "\n")
	}
	return strings.Join(append(lines, m.themePage()...), "\n")
}

func (m Model) settingsTabs() string {
	parts := make([]string, 0, len(settingsPages))
	for _, p := range settingsPages {
		style := styleTab
		if p == m.settingsPage {
			style = styleTabActive
		}
		parts = append(parts, style.Render(p.title()))
	}
	return strings.Join(parts, " ") + "  " + styleDim.Render("tab")
}

func (m Model) themePage() []string {
	lines := []string{styleDim.Render("theme")}

	for i, name := range ThemeNames() {
		if i == m.themeCursor {
			lines = append(lines, styleSelected.Render(" ▸ "+name+" "))
			continue
		}
		lines = append(lines, "   "+styleBase.Render(name))
	}

	path := "~/.config/tuigy/config.yml"
	if p, err := config.Path(); err == nil {
		path = config.ShortPath(p)
	}

	// The path can be long, so it is shortened from the front: the file name is
	// the part that has to stay readable.
	width := max(m.width-10, 20)

	return append(lines,
		"",
		styleDim.Render("Everything is repainted as you move, so you can see each one."),
		"",
		styleSection.Render("EVERYTHING ELSE"),
		styleDim.Render("Individual colours, the editor and the commit message agent"),
		styleDim.Render("live in this file, beside what is set here:"),
		"  "+styleBase.Render(truncateLeft(path, width)),
		styleDim.Render("Run tuigy --init-config to write a documented one."),
	)
}

func (m Model) keysPage() []string {
	rows := m.keyRows()

	query := "/" + m.keyFilter
	if m.keyFiltering {
		query += "▏"
	}
	lines := []string{styleKey.Render(query) + "  " +
		styleDim.Render(fmt.Sprintf("%d of %d actions", len(rows), len(keys.ActionNames())))}

	if len(rows) == 0 {
		return append(lines, "", styleDim.Render("nothing matches"))
	}

	// Columns: the action, the keys it is on, and what it does with whatever is
	// left over.
	nameW, strokeW := 18, 14
	descW := max(m.width-6-nameW-strokeW-6, 10)

	start := clamp(m.keyOff, 0, max(len(rows)-keysPerPage, 0))
	if start > 0 {
		lines = append(lines, styleDim.Render("   ↑ more"))
	} else {
		lines = append(lines, "")
	}

	for i := start; i < min(start+keysPerPage, len(rows)); i++ {
		r := rows[i]
		mark := " "
		if r.changed {
			mark = "•"
		}
		// The description is padded out rather than merely clipped, so that the
		// dialog keeps the same width as the filter narrows the list.
		text := padRight(r.action, nameW) + padRight(r.stroke, strokeW) +
			padRight(clipLine(r.desc, descW), descW)

		if i == m.keyCursor {
			// The three columns before the name are shared: the cursor and the
			// changed mark sit in them together rather than hiding each other.
			lines = append(lines, styleSelected.Render(mark+"▸ "+text))
			continue
		}
		lines = append(lines, styleAdded.Render(mark)+styleBase.Render("  "+padRight(r.action, nameW))+
			styleKey.Render(padRight(r.stroke, strokeW))+
			styleDesc.Render(padRight(clipLine(r.desc, descW), descW)))
	}

	if start+keysPerPage < len(rows) {
		lines = append(lines, styleDim.Render("   ↓ more"))
	} else {
		lines = append(lines, "")
	}

	lines = append(lines, "", m.keysFootnote())
	return lines
}

// keysFootnote is the line under the list: what is being waited for, what just
// went wrong, or what the marks mean.
func (m Model) keysFootnote() string {
	switch {
	case m.capturing:
		return styleFlash.Render("press the key you want  ") + styleDim.Render("esc cancels")
	case m.keyNote != "":
		return styleDirty.Render(clipLine(m.keyNote, max(m.width-10, 20)))
	default:
		return styleDim.Render("• marks a binding you have changed; each change is saved as you make it")
	}
}

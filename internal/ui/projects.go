package ui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/mustafakarakulak/tuigy/internal/config"
	"github.com/mustafakarakulak/tuigy/internal/git"
)

// The project switcher is a list of the repositories tuigy has been opened in,
// most recent first.
//
// Every repository is remembered on the way in, without being asked: a switcher
// whose list you have to curate is one more thing to maintain, and the list it
// would hold is the same one.

type (
	projectsMsg struct{ projects []config.Project }
	// repoOpenedMsg carries a repository the switcher opened, or why it could
	// not be opened — a remembered path can have been moved or deleted.
	repoOpenedMsg struct {
		project config.Project
		repo    *git.Repo
		err     error
	}
)

// rememberProject records the current repository and reads the list back.
func (m Model) rememberProject() tea.Cmd {
	root, name := m.repo.Root, m.repo.Name()
	return func() tea.Msg {
		projects, err := config.RememberProject(root, name)
		if err != nil {
			return errMsg{err}
		}
		return projectsMsg{projects: projects}
	}
}

// openProjects shows the switcher.
func (m Model) openProjects() (tea.Model, tea.Cmd) {
	m.err = nil
	m.modal = modalProjects
	m.projectFilter = ""
	m.projectCursor = 0

	// The list is re-read rather than trusted: another tuigy in another
	// repository has been writing to the same file.
	return m, func() tea.Msg {
		projects, err := config.LoadProjects()
		if err != nil {
			return errMsg{err}
		}
		return projectsMsg{projects: projects}
	}
}

func (m Model) handleProjectsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	matches := m.matchingProjects()

	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.modal = modalNone
		return m, nil

	case key.Matches(msg, m.keys.Up):
		m.projectCursor = max(m.projectCursor-1, 0)
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.projectCursor = min(m.projectCursor+1, max(len(matches)-1, 0))
		return m, nil

	case key.Matches(msg, m.keys.Confirm):
		if m.projectCursor < 0 || m.projectCursor >= len(matches) {
			return m, nil
		}
		return m, openRepo(matches[m.projectCursor])

	case key.Matches(msg, m.keys.DeleteRef):
		if m.projectCursor < 0 || m.projectCursor >= len(matches) {
			return m, nil
		}
		return m, m.forgetProject(matches[m.projectCursor])
	}

	// Everything else types into the filter: a switcher is reached for when you
	// already know which repository you want.
	switch msg.Type {
	case tea.KeyBackspace:
		runes := []rune(m.projectFilter)
		if len(runes) > 0 {
			m.projectFilter = string(runes[:len(runes)-1])
			m.projectCursor = 0
		}
	case tea.KeySpace:
		m.projectFilter += " "
		m.projectCursor = 0
	case tea.KeyRunes:
		m.projectFilter += string(msg.Runes)
		m.projectCursor = 0
	}
	return m, nil
}

// matchingProjects is the list as filtered, with the repository already open
// left out: switching to where you are is not a thing anyone means to do.
func (m Model) matchingProjects() []config.Project {
	out := make([]config.Project, 0, len(m.projects))
	for _, p := range m.projects {
		if p.Path == m.repo.Root {
			continue
		}
		if matchesFilter(m.projectFilter, p.Name) || matchesFilter(m.projectFilter, p.Path) {
			out = append(out, p)
		}
	}
	return out
}

func openRepo(p config.Project) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		repo, err := git.Open(ctx, p.Path)
		return repoOpenedMsg{project: p, repo: repo, err: err}
	}
}

func (m Model) forgetProject(p config.Project) tea.Cmd {
	return func() tea.Msg {
		projects, err := config.ForgetProject(p.Path)
		if err != nil {
			return errMsg{err}
		}
		return projectsMsg{projects: projects}
	}
}

// adoptRepo points the whole interface at another repository.
//
// Everything derived from the old one is dropped rather than reloaded over:
// a cursor, a filter or a review mark carried across would be describing a
// repository that is no longer on screen. What the user chose — the theme, the
// key map, the editor — stays.
func (m *Model) adoptRepo(repo *git.Repo) tea.Cmd {
	m.closeTerminal()

	m.repo = repo
	m.modal = modalNone
	m.err = nil

	m.status, m.statusFP, m.opState = nil, "", git.OpNone
	m.reviewed = nil
	m.clearFilter()

	m.rows, m.cursor, m.listOff = nil, 0, 0
	m.diffKey, m.diffRaw, m.diffHunks, m.hunkCursor = "", "", nil, 0
	m.focus = paneList

	m.allBranches, m.branchRows, m.branchCur, m.branchOff = nil, nil, 0, 0

	m.historyRef, m.commits, m.commitCursor, m.commitOff = "", nil, 0, 0
	m.picked, m.historyDone, m.historyBusy = nil, false, false
	m.detailKey = ""

	m.allStashes, m.stashes, m.stashCursor, m.stashOff, m.stashKey = nil, nil, 0, 0, ""

	m.allFiles, m.fileRows, m.fileCursor, m.fileOff = nil, nil, 0, 0
	m.expanded, m.filesLoaded, m.previewKey = nil, false, ""

	m.diff.SetContent("")
	m.detail.SetContent("")
	m.stashView.SetContent("")
	m.preview.SetContent("")

	return tea.Batch(m.loadStatus(), m.loadBranches(), m.loadFiles(), m.rememberProject())
}

// ---------------------------------------------------------------- view

func (m Model) projectsBox() string {
	matches := m.matchingProjects()

	query := "/" + m.projectFilter + "▏"
	lines := []string{
		styleTitle.Render("Projects"),
		styleDim.Render("every repository tuigy has been opened in, most recent first"),
		"",
		styleKey.Render(query),
		"",
	}

	if len(matches) == 0 {
		if len(m.projects) <= 1 {
			return strings.Join(append(lines,
				styleDim.Render("nothing else yet — open tuigy in another repository"),
				styleDim.Render("and it will be here next time"),
			), "\n")
		}
		return strings.Join(append(lines, styleDim.Render("nothing matches")), "\n")
	}

	// Windowed so a long list cannot push the dialog off a short terminal.
	bodyH := max(m.height-headerHeight-footerHeight, 3)
	visible := clamp(bodyH-10, 3, len(matches))
	start := clamp(m.projectCursor-visible/2, 0, max(len(matches)-visible, 0))

	if start > 0 {
		lines = append(lines, styleDim.Render("   ↑ more"))
	}

	width := max(m.width/2, 30)
	for i := start; i < min(start+visible, len(matches)); i++ {
		p := matches[i]
		name := padRight(p.Name, min(nameColumn(matches), width/2))
		where := truncateLeft(config.ShortPath(p.Path), max(width-len(name)-4, 10))

		if i == m.projectCursor {
			lines = append(lines, styleSelected.Render(" ▸ "+name+" "+where+" "))
			continue
		}
		lines = append(lines, "   "+styleBase.Render(name)+" "+styleDim.Render(where))
	}

	if start+visible < len(matches) {
		lines = append(lines, styleDim.Render("   ↓ more"))
	}

	return strings.Join(lines, "\n")
}

// nameColumn is how wide the name column has to be for the paths to line up.
func nameColumn(projects []config.Project) int {
	width := 0
	for _, p := range projects {
		width = max(width, len([]rune(p.Name)))
	}
	return width
}

package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// Repo is an open git repository.
type Repo struct {
	// Root is the top of the working tree.
	Root string
	// GitDir is the absolute path of the .git directory. In a linked worktree
	// it is not Root/.git.
	GitDir string

	run *Runner
}

// Open finds the repository containing path by searching upwards.
func Open(ctx context.Context, path string) (*Repo, error) {
	root, err := NewRunner(path).Read(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}

	repo := &Repo{Root: strings.TrimSpace(root)}
	repo.run = NewRunner(repo.Root)

	gitDir, err := repo.run.Read(ctx, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return nil, err
	}
	repo.GitDir = strings.TrimSpace(gitDir)

	return repo, nil
}

// EditorCommand is the editor git itself would open, and whether that was
// actually chosen by the user rather than git's own fallback.
//
// Asking git rather than reading the environment directly is what makes
// core.editor work: git consults GIT_EDITOR, then core.editor, then VISUAL and
// EDITOR, and plenty of people set only the config option. The distinction
// matters because git's last resort is vi, which reads the same in the answer
// whether it was asked for or not.
//
// There is no error to report. On a machine with no editor installed at all,
// git refuses to name one — which is an answer, not a failure, and exactly the
// case the caller has to handle anyway.
func (r *Repo) EditorCommand(ctx context.Context) (command string, chosen bool) {
	if out, err := r.run.Read(ctx, "var", "GIT_EDITOR"); err == nil {
		command = strings.TrimSpace(out)
	}

	if _, err := r.run.Read(ctx, "config", "--get", "core.editor"); err == nil {
		chosen = true
	}
	for _, name := range []string{"GIT_EDITOR", "VISUAL", "EDITOR"} {
		if os.Getenv(name) != "" {
			chosen = true
		}
	}

	return command, chosen
}

// Name is the name of the repository directory.
func (r *Repo) Name() string { return filepath.Base(r.Root) }

// OpState identifies a git operation that is partway through.
type OpState string

const (
	OpNone       OpState = ""
	OpMerge      OpState = "merge"
	OpCherryPick OpState = "cherry-pick"
	OpRevert     OpState = "revert"
	OpRebase     OpState = "rebase"
)

// State reports an operation left unfinished, if any.
//
// git exposes no porcelain command for this, so we look for the marker files
// inside .git — the same thing git's own shell prompt does.
func (r *Repo) State() OpState {
	switch {
	case r.gitPathExists("MERGE_HEAD"):
		return OpMerge
	case r.gitPathExists("CHERRY_PICK_HEAD"):
		return OpCherryPick
	case r.gitPathExists("REVERT_HEAD"):
		return OpRevert
	case r.gitPathExists("rebase-merge"), r.gitPathExists("rebase-apply"):
		return OpRebase
	default:
		return OpNone
	}
}

func (r *Repo) gitPathExists(name string) bool {
	_, err := os.Stat(filepath.Join(r.GitDir, name))
	return err == nil
}

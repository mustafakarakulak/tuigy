package git

import (
	"context"
	"os"
	"path/filepath"
)

// Stage adds the given paths to the index.
func (r *Repo) Stage(ctx context.Context, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	_, err := r.run.Write(ctx, append([]string{"add", "--"}, paths...)...)
	return err
}

// StageAll adds every change, untracked files included, to the index.
func (r *Repo) StageAll(ctx context.Context) error {
	_, err := r.run.Write(ctx, "add", "--all")
	return err
}

// Unstage removes the given paths from the index, leaving the working tree alone.
func (r *Repo) Unstage(ctx context.Context, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	_, err := r.run.Write(ctx, append([]string{"restore", "--staged", "--"}, paths...)...)
	return err
}

// UnstageAll resets the index to HEAD, leaving the working tree alone.
func (r *Repo) UnstageAll(ctx context.Context) error {
	_, err := r.run.Write(ctx, "reset")
	return err
}

// Discard throws away a file's working tree changes.
//
// This cannot be undone: a tracked file is restored to its HEAD state and an
// untracked one is deleted from disk. Callers are expected to have confirmed
// with the user first.
func (r *Repo) Discard(ctx context.Context, f FileStatus) error {
	if f.Worktree == StatusUntracked {
		return os.RemoveAll(filepath.Join(r.Root, f.Path))
	}
	_, err := r.run.Write(ctx, "restore", "--", f.Path)
	return err
}

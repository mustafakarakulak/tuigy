package git

import "context"

// FileDiff returns the raw diff of a single file.
//
// When staged is true the index is compared against HEAD, otherwise the working
// tree is compared against the index.
func (r *Repo) FileDiff(ctx context.Context, f FileStatus, staged bool) (string, error) {
	if f.Worktree == StatusUntracked {
		return r.untrackedDiff(ctx, f.Path)
	}

	args := []string{"diff"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", f.Path)

	return r.readDiff(ctx, args...)
}

// StagedDiff is the diff of every staged change.
func (r *Repo) StagedDiff(ctx context.Context) (string, error) {
	return r.readDiff(ctx, "diff", "--cached")
}

// untrackedDiff presents an untracked file as if every line were added.
//
// The literal "/dev/null" is git's own sentinel for "the empty side of a diff",
// not a filesystem path, so it is spelled out rather than taken from os.DevNull
// — which would be "NUL" on Windows and mean nothing to git.
//
// --no-index exits 1 when it finds a difference, which is not an error here.
func (r *Repo) untrackedDiff(ctx context.Context, path string) (string, error) {
	out, err := r.readDiff(ctx, "diff", "--no-index", "--", "/dev/null", path)
	if err != nil && ExitCode(err) == 1 {
		return out, nil
	}
	return out, err
}

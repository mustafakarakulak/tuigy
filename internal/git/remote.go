package git

import (
	"context"
	"errors"
	"slices"
	"strings"
)

// Remotes lists the configured remote names.
func (r *Repo) Remotes(ctx context.Context) ([]string, error) {
	out, err := r.run.Read(ctx, "remote")
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

// defaultRemote picks the remote to publish a new branch to: "origin" when it
// exists, otherwise the only remote there is.
func (r *Repo) defaultRemote(ctx context.Context) (string, error) {
	remotes, err := r.Remotes(ctx)
	if err != nil {
		return "", err
	}

	switch {
	case slices.Contains(remotes, "origin"):
		return "origin", nil
	case len(remotes) == 1:
		return remotes[0], nil
	case len(remotes) == 0:
		return "", errors.New("no remote configured")
	default:
		return "", errors.New("several remotes configured and none is named origin")
	}
}

// Fetch updates remote-tracking refs. With all, every remote is fetched.
//
// --prune keeps the branch list honest: without it, branches deleted on the
// remote linger locally and get offered for checkout.
func (r *Repo) Fetch(ctx context.Context, all bool) error {
	args := []string{"fetch", "--prune"}
	if all {
		args = append(args, "--all")
	}
	_, err := r.run.Write(ctx, args...)
	return err
}

// Pull updates the current branch, fast-forward only.
//
// Refusing to merge is deliberate. A pull that silently creates a merge commit
// is the single most common way of accidentally polluting a branch's history,
// so a divergent branch is reported instead of quietly reconciled.
func (r *Repo) Pull(ctx context.Context) error {
	_, err := r.run.Write(ctx, "pull", "--ff-only")
	return err
}

// Push publishes the current branch.
//
// A branch without an upstream is published to the default remote and the
// upstream is set, which is what the user means by "push" in every case where
// they have not said otherwise.
func (r *Repo) Push(ctx context.Context, branch, upstream string) error {
	if upstream != "" {
		_, err := r.run.Write(ctx, "push")
		return err
	}

	remote, err := r.defaultRemote(ctx)
	if err != nil {
		return err
	}
	_, err = r.run.Write(ctx, "push", "--set-upstream", remote, branch)
	return err
}

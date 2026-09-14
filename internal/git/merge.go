package git

import (
	"context"
	"errors"
	"fmt"
)

// CanFastForward reports whether target can be advanced to source without a
// merge commit, which is the case when target is already an ancestor of source.
func (r *Repo) CanFastForward(ctx context.Context, source, target string) (bool, error) {
	_, err := r.run.Read(ctx, "merge-base", "--is-ancestor", target, source)
	switch {
	case err == nil:
		return true, nil
	case ExitCode(err) == 1:
		return false, nil
	default:
		return false, err
	}
}

// FastForward advances target to source without checking target out.
//
// Fetching from "." updates a ref in this very repository and refuses anything
// that is not a fast-forward, which is exactly the guarantee wanted here: the
// working tree and the branch you are on are left completely alone.
func (r *Repo) FastForward(ctx context.Context, source, target string) error {
	_, err := r.run.Write(ctx, "fetch", ".", source+":"+target)
	return err
}

// Merge merges source into the branch that is currently checked out.
func (r *Repo) Merge(ctx context.Context, source string) error {
	_, err := r.run.Write(ctx, "merge", "--no-edit", source)
	return err
}

// MergeInto merges source into target.
//
// git offers no way to merge into a branch you are not on. Either the update is
// a fast-forward, which can be written straight to the ref, or target has to be
// checked out first — which leaves the user on a different branch afterwards.
// Callers are expected to have told the user which of the two will happen.
func (r *Repo) MergeInto(ctx context.Context, source, target, current string) error {
	if source == target {
		return errors.New("a branch cannot be merged into itself")
	}
	if target == current {
		return r.Merge(ctx, source)
	}

	ff, err := r.CanFastForward(ctx, source, target)
	if err != nil {
		return err
	}
	if ff {
		return r.FastForward(ctx, source, target)
	}

	if err := r.Checkout(ctx, target); err != nil {
		return err
	}
	return r.Merge(ctx, source)
}

// ContinueOperation resumes the merge, cherry-pick or revert that is in progress.
//
// git would open an editor for the resulting commit message; the environment
// set in baseEnv keeps it from doing so and accepts the message git prepared.
func (r *Repo) ContinueOperation(ctx context.Context) error {
	cmd, err := operationCommand(r.State())
	if err != nil {
		return err
	}
	_, err = r.run.Write(ctx, cmd, "--continue")
	return err
}

// AbortOperation throws away the merge, cherry-pick or revert in progress and
// restores the working tree to where it was before.
func (r *Repo) AbortOperation(ctx context.Context) error {
	cmd, err := operationCommand(r.State())
	if err != nil {
		return err
	}
	_, err = r.run.Write(ctx, cmd, "--abort")
	return err
}

func operationCommand(state OpState) (string, error) {
	switch state {
	case OpMerge:
		return "merge", nil
	case OpCherryPick:
		return "cherry-pick", nil
	case OpRevert:
		return "revert", nil
	case OpRebase:
		return "rebase", nil
	default:
		return "", fmt.Errorf("no operation in progress")
	}
}

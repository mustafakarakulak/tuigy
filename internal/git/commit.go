package git

import (
	"context"
	"errors"
	"strings"
)

// ErrNothingStaged reports a commit attempted with an empty index.
var ErrNothingStaged = errors.New("nothing staged to commit")

// Commit records the staged changes with the given message.
func (r *Repo) Commit(ctx context.Context, message string) error {
	if strings.TrimSpace(message) == "" {
		return errors.New("commit message cannot be empty")
	}
	_, err := r.run.Write(ctx, "commit", "-m", message)
	return err
}

// Amend replaces the last commit with the staged changes and the given message.
func (r *Repo) Amend(ctx context.Context, message string) error {
	if strings.TrimSpace(message) == "" {
		return errors.New("commit message cannot be empty")
	}
	_, err := r.run.Write(ctx, "commit", "--amend", "-m", message)
	return err
}

// LastCommitMessage is the full message of HEAD, used to prefill the amend view.
func (r *Repo) LastCommitMessage(ctx context.Context) (string, error) {
	out, err := r.run.Read(ctx, "log", "-1", "--pretty=%B")
	return strings.TrimSpace(out), err
}

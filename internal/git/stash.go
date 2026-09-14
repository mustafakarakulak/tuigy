package git

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Stash is one entry in the stash list.
type Stash struct {
	// Ref is git's own selector, such as "stash@{0}". Every operation takes
	// this rather than an index, because dropping one renumbers the rest.
	Ref string
	// Branch is the branch the stash was made on.
	Branch string
	// Message is what the stash was saved as: either the text given at the
	// time, or the commit subject git fell back to.
	Message string
	Date    time.Time
}

// Stash saves the working tree and index, leaving a clean tree behind.
//
// Untracked files are left alone, matching plain `git stash`: they never block
// a branch switch, and sweeping build output into a stash surprises people.
func (r *Repo) Stash(ctx context.Context, message string) error {
	args := []string{"stash", "push"}
	if m := strings.TrimSpace(message); m != "" {
		args = append(args, "--message", m)
	}
	_, err := r.run.Write(ctx, args...)
	return err
}

// Stashes lists saved stashes, most recent first.
func (r *Repo) Stashes(ctx context.Context) ([]Stash, error) {
	out, err := r.run.Read(ctx, "stash", "list", "--format=%gd%x00%gs%x00%ct")
	if err != nil {
		return nil, err
	}

	var stashes []Stash
	for line := range strings.SplitSeq(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		if s, ok := parseStash(line); ok {
			stashes = append(stashes, s)
		}
	}
	return stashes, nil
}

func parseStash(line string) (Stash, bool) {
	f := strings.SplitN(line, "\x00", 3)
	if len(f) < 3 {
		return Stash{}, false
	}

	s := Stash{Ref: f[0]}
	s.Branch, s.Message = parseStashSubject(f[1])
	if secs, err := strconv.ParseInt(f[2], 10, 64); err == nil {
		s.Date = time.Unix(secs, 0)
	}
	return s, true
}

// parseStashSubject splits git's reflog subject for a stash, which is
// "On <branch>: <message>" when a message was given and
// "WIP on <branch>: <hash> <subject>" when git made one up.
func parseStashSubject(subject string) (branch, message string) {
	rest, ok := strings.CutPrefix(subject, "WIP on ")
	automatic := ok
	if !ok {
		rest, ok = strings.CutPrefix(subject, "On ")
		if !ok {
			return "", subject
		}
	}

	branch, message, ok = strings.Cut(rest, ": ")
	if !ok {
		return "", subject
	}

	// An automatic message is the commit the stash was made on top of, which
	// says nothing about the stash itself; the hash in front is pure noise.
	if automatic {
		if _, subject, found := strings.Cut(message, " "); found {
			message = subject
		}
	}
	return branch, message
}

// StashApply restores a stash and leaves it in the list.
func (r *Repo) StashApply(ctx context.Context, ref string) error {
	return r.stashOp(ctx, "apply", ref)
}

// StashPop restores a stash and removes it. If applying conflicts, git keeps
// the stash, so nothing is lost.
func (r *Repo) StashPop(ctx context.Context, ref string) error {
	return r.stashOp(ctx, "pop", ref)
}

// StashDrop deletes a stash without restoring it. This cannot be undone from
// within tuigy, so callers are expected to have confirmed with the user.
func (r *Repo) StashDrop(ctx context.Context, ref string) error {
	return r.stashOp(ctx, "drop", ref)
}

func (r *Repo) stashOp(ctx context.Context, op, ref string) error {
	if ref == "" {
		return errors.New("no stash selected")
	}
	_, err := r.run.Write(ctx, "stash", op, ref)
	return err
}

// StashDiff is the diff a stash would restore.
func (r *Repo) StashDiff(ctx context.Context, ref string) (string, error) {
	if ref == "" {
		return "", errors.New("no stash selected")
	}
	return r.readDiff(ctx, "stash", "show", "--patch", ref)
}

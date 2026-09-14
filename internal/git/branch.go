package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Branch is one local or remote-tracking branch.
type Branch struct {
	Name    string // short name, e.g. "main" or "origin/main"
	Ref     string // full ref, e.g. "refs/heads/main"
	Remote  bool   // lives under refs/remotes
	Current bool

	Upstream string
	Ahead    int
	Behind   int
	// Gone reports an upstream that is configured but no longer exists.
	Gone bool

	Hash      string
	Subject   string
	Author    string
	Committed time.Time
}

// branchFormat lists the fields Branches reads, NUL-separated so that a subject
// containing anything at all cannot break the parse.
const branchFormat = "%(HEAD)%00%(refname)%00%(refname:short)%00%(upstream:short)" +
	"%00%(upstream:track)%00%(objectname:short)%00%(committerdate:unix)" +
	"%00%(authorname)%00%(contents:subject)"

// Branches lists local and remote-tracking branches, most recently committed first.
//
// Recency beats alphabetical order here: the branches you are switching between
// are almost always the ones you touched last.
func (r *Repo) Branches(ctx context.Context) ([]Branch, error) {
	out, err := r.run.ReadC(ctx,
		"for-each-ref",
		"--sort=-committerdate",
		"--format="+branchFormat,
		"refs/heads", "refs/remotes",
	)
	if err != nil {
		return nil, err
	}

	var branches []Branch
	for line := range strings.SplitSeq(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		b, ok := parseBranch(line)
		if !ok {
			continue
		}
		branches = append(branches, b)
	}
	return branches, nil
}

func parseBranch(line string) (Branch, bool) {
	f := strings.SplitN(line, "\x00", 9)
	if len(f) < 9 {
		return Branch{}, false
	}

	b := Branch{
		Current:  f[0] == "*",
		Ref:      f[1],
		Name:     f[2],
		Upstream: f[3],
		Hash:     f[5],
		Author:   f[7],
		Subject:  f[8],
	}
	b.Remote = strings.HasPrefix(b.Ref, "refs/remotes/")

	// refs/remotes/<remote>/HEAD is a symref to the default branch, not a
	// branch of its own; listing it would just duplicate an entry.
	if b.Remote && strings.HasSuffix(b.Ref, "/HEAD") {
		return Branch{}, false
	}

	b.Ahead, b.Behind, b.Gone = parseTrack(f[4])
	if secs, err := strconv.ParseInt(f[6], 10, 64); err == nil {
		b.Committed = time.Unix(secs, 0)
	}
	return b, true
}

// parseTrack reads for-each-ref's tracking summary, which is one of "",
// "[gone]", "[ahead N]", "[behind N]" or "[ahead N, behind N]". The command is
// run under a C locale so these words are stable.
func parseTrack(s string) (ahead, behind int, gone bool) {
	s = strings.Trim(s, "[]")
	if s == "" {
		return 0, 0, false
	}
	if s == "gone" {
		return 0, 0, true
	}

	for part := range strings.SplitSeq(s, ", ") {
		kind, num, ok := strings.Cut(part, " ")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(num)
		if err != nil {
			continue
		}
		switch kind {
		case "ahead":
			ahead = n
		case "behind":
			behind = n
		}
	}
	return ahead, behind, false
}

// Checkout switches to an existing branch.
//
// A remote-tracking name such as "origin/feature" is handled by git's own DWIM:
// it creates a local branch of the same short name that tracks the remote.
func (r *Repo) Checkout(ctx context.Context, name string) error {
	if remote, ok := strings.CutPrefix(name, "refs/remotes/"); ok {
		name = remote
	}
	_, err := r.run.Write(ctx, "switch", name)
	return err
}

// CheckoutRemote creates a local branch tracking the given remote-tracking branch.
//
// If a local branch of that short name already exists, it is switched to
// instead: checking out origin/main should land you on main either way. The
// original error is kept when the fallback fails too, since it explains more.
func (r *Repo) CheckoutRemote(ctx context.Context, remoteBranch string) error {
	_, err := r.run.Write(ctx, "switch", "--track", remoteBranch)
	if err == nil {
		return nil
	}

	_, short, ok := strings.Cut(remoteBranch, "/")
	if !ok || short == "" {
		return err
	}
	if _, fallbackErr := r.run.Write(ctx, "switch", short); fallbackErr != nil {
		return err
	}
	return nil
}

// CreateBranch creates a branch from the given starting point and switches to it.
func (r *Repo) CreateBranch(ctx context.Context, name, from string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	args := []string{"switch", "--create", name}
	if from != "" {
		args = append(args, from)
	}
	_, err := r.run.Write(ctx, args...)
	return err
}

// DeleteBranch removes a local branch. Without force, git refuses to delete a
// branch whose commits are not merged anywhere else.
func (r *Repo) DeleteBranch(ctx context.Context, name string, force bool) error {
	flag := "--delete"
	if force {
		flag = "-D"
	}
	_, err := r.run.Write(ctx, "branch", flag, name)
	return err
}

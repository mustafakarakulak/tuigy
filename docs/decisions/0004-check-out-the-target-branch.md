# 4. Check out the target branch for merge and cherry-pick, and say so first

Status: accepted

## Context

The interface lets the user pick a source and a target: "merge feature/payment into
develop", from wherever they happen to be standing. git has no such operation. It
merges into the branch that is checked out.

There are three ways to bridge that, and all of them cost something:

- Check the target out, do the work, and leave the user there.
- Write straight to the ref when the update is a fast-forward, which touches nothing.
- Do the work in a temporary worktree, leaving the current one alone.

## Decision

Fast-forward straight to the ref when that is possible, using `git fetch . source:target`,
which refuses anything that is not a fast-forward.

Otherwise check the target out and do the work there.

Either way, say which one will happen before it does. The confirmation names the
outcome: "merged into the branch you are on", "fast-forward: your working tree and
current branch are untouched", or "develop will be checked out first, and you will be
left on it".

## Consequences

The common case — merging into the branch you are on — costs nothing. The next most
common case, advancing a branch you are not on, touches neither the working tree nor
the current branch.

The remaining case moves the user, which is surprising unless it is announced. It is
announced.

The temporary worktree approach was considered and rejected for now: it avoids moving
the user, but a conflict inside a temporary worktree is considerably harder to explain
and to resolve than one in the working tree they can see.

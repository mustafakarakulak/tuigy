# 5. Pull is fast-forward only, and nothing is stashed unasked

Status: accepted

## Context

Two git operations routinely do something the user did not intend.

`git pull` on a divergent branch creates a merge commit. Most people who hit this did
not want it, and many do not notice until the history is already polluted.

Switching branches with local changes fails. A tool can be helpful by stashing them
first — and the work then disappears from the working tree with no obvious way back.

## Decision

`pull` runs with `--ff-only`. A divergent branch is reported, not reconciled.

When local changes block a branch switch, the failure opens a dialog offering to stash
and switch. It is never done without that answer, and the stash is given a message
saying what it was for.

## Consequences

A divergent branch needs a deliberate rebase or merge, which tuigy does not currently
offer from the pull key. That is the intended friction: the user decides how their
history is shaped.

The stash dialog only appears when the working tree is actually dirty, so an unrelated
checkout failure reports its own error rather than offering something irrelevant.

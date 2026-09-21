# Roadmap

What is built, what is deliberately not, and what is next. Kept here so that the
reasoning does not have to be reconstructed each time.

## Built

Everything on the original plan, plus a few things that were not on it:

- changes, branches, history and stashes, each with its own tab
- stage, unstage, discard, commit, amend; hunk-level staging from inside the diff
- branch switching, creation, deletion; fetch, pull, push
- merge with an explicit source and target; cherry-pick of one or more commits
- conflict handling for merge, cherry-pick and revert
- live refresh, review marks, filtering, copying, themes, key bindings
- commit messages from a coding agent

## Deliberately not built

From the original specification's own list, and still out:

advanced interactive rebase · a full conflict editor · blame · bisect · submodules ·
a commit graph · tag management · force push · commit signing settings

A file tree, an embedded terminal and a pull request dashboard have also been
proposed and declined. See [0014](decisions/0014-what-tuigy-is-not.md) for the test
each was measured against.

## Next, roughly in order of value

**Worktree management.** The best fit for the audience that is left unbuilt: running
several agents in parallel, each in its own worktree, and moving between them. A new
tab and a switching flow.

**Undo via the reflog.** Recovering from a bad reset, merge or cherry-pick. Medium
sized and independent of everything else.

**Opening a pull request for the current branch.** The tail of the push flow, through
`gh`. Small. Not a dashboard.

**Handing the terminal over.** A key that suspends tuigy, runs a shell or an agent in
the real terminal, and refreshes on return. About thirty lines, reusing what `e` does.

**Syntax highlighting in diffs.** Costs 2.1 MB of binary for chroma, and needs the
added/removed colouring reworked onto backgrounds so syntax colours have somewhere to
live. Worth doing after deciding how that should look.

**A command palette.** Fuzzy search over all 52 actions, which would remove the
question of which key does what entirely.

## Distribution

The release pipeline works and has been rehearsed, but nothing has been tagged. When
it is:

- a tap of one's own is the realistic route; `brew install` from homebrew-core needs
  notability thresholds that a new repository does not meet, and self-submissions face
  three times the bar
- `go install` already works
- Windows is not supported: opening an editor and generating a commit message both go
  through a POSIX shell

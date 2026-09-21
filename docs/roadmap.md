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

And then, against [0014](decisions/0014-what-tuigy-is-not.md)'s own reasoning and with
[0015](decisions/0015-a-workspace-around-the-git-tool.md) written to say why:

- a repository tree of what git tracks, with the file itself in the pane beside it,
  and folding by directory, by subtree or all at once
- an embedded shell in a band across the bottom, under both panes
- a project switcher over every repository tuigy has been opened in
- a settings screen that changes key bindings as well as the theme, writing each one
  to the configuration file as it is made
- a command palette on `:`, listing what the keyboard would do from where you are,
  with the key beside each command

## Deliberately not built

From the original specification's own list, and still out:

advanced interactive rebase · a full conflict editor · blame · bisect · submodules ·
a commit graph · tag management · force push · commit signing settings

A pull request dashboard has been proposed and declined, and editing a file is the
line tuigy does not cross. See
[0015](decisions/0015-a-workspace-around-the-git-tool.md) for the test each is
measured against, and [0014](decisions/0014-what-tuigy-is-not.md) for the one it
replaced.

## Next, roughly in order of value

**Worktree management.** The best fit for the audience that is left unbuilt: running
several agents in parallel, each in its own worktree, and moving between them. A new
tab and a switching flow.

**Undo via the reflog.** Recovering from a bad reset, merge or cherry-pick. Medium
sized and independent of everything else.

**Opening a pull request for the current branch.** The tail of the push flow, through
`gh`. Small. Not a dashboard.

**Syntax highlighting in diffs.** Costs 2.1 MB of binary for chroma, and needs the
added/removed colouring reworked onto backgrounds so syntax colours have somewhere to
live. Worth doing after deciding how that should look.

**Scrollback in the terminal band.** What scrolled past is currently the running
program's to reach. See [0016](decisions/0016-the-terminal-is-a-band.md).

## Distribution

The release pipeline works and has been rehearsed, but nothing has been tagged. When
it is:

- a tap of one's own is the realistic route; `brew install` from homebrew-core needs
  notability thresholds that a new repository does not meet, and self-submissions face
  three times the bar
- `go install` already works
- Windows is not supported: opening an editor and generating a commit message both go
  through a POSIX shell, and the terminal band is a POSIX pseudo-terminal

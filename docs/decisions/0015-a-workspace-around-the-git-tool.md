# 15. A workspace around the git tool

Status: accepted

Supersedes the file tree and embedded terminal parts of
[14](0014-what-tuigy-is-not.md).

## Context

[14](0014-what-tuigy-is-not.md) declined a file tree and an embedded terminal pane,
and closed by saying that deciding to become something larger was open, but had to be
made deliberately and recorded here rather than arrived at one feature at a time.

This is that record. A repository tree, a terminal pane and a project switcher were
all asked for together, and all three were built.

## Context: what the test missed

The test 14 set was: *does it make tuigy the fastest way to review and land what a
coding agent just wrote?* That test is still the right one. What it got wrong was
where a review starts and stops.

**Reading a diff raises questions the diff cannot answer.** What else is in this
package. What the function being called actually does. Whether the file the agent
created duplicates one that already exists. 14 answered this with "the editor does it
better", which is true of editing and false of looking: leaving for an editor to
answer a question about a diff means coming back and finding your place again. The
tree here is not an "open this file" menu — it is a pane that shows the file, beside
the pane showing the change to it. `e` still hands the file to the real editor.

**Running something is part of landing it.** The tests, a linter, the agent again.
14 saw this and proposed handing the terminal over — suspend, run, come back. That is
the right shape for one command and the wrong shape for watching a test run against
the files you are reading, because tuigy is not on screen while it happens.

**The cost argument no longer holds.** 14 priced an embedded terminal as a PTY, a VT
parser, and permanent maintenance of emulation bugs. The parser is a dependency
(`charmbracelet/x/vt`), not something written here; the maintenance is someone else's.
What is genuinely ours is the focus model, which is one reserved keystroke and is
recorded in [16](0016-the-terminal-is-a-band.md).

**A repository is not the unit of work any more.** Several agents run in several
checkouts. Restarting tuigy per repository throws away the session: the review marks,
the filter, the shell that is mid-command. Switching in place is
[17](0017-projects-are-remembered.md).

## Decision

tuigy is a git tool with a workspace around it. The test becomes: **does it keep you
in one place while you review and land what an agent wrote?**

That is a wider door than 14's, so what it lets through is stated:

- **A repository tree — accepted.** Reading in place, not opening. It shows what git
  tracks, so ignored build output never appears in it.
- **An embedded terminal — accepted.** A band across the bottom, under both panes.
  See [16](0016-the-terminal-is-a-band.md).
- **A project switcher — accepted.** See [17](0017-projects-are-remembered.md).
- **A pull request dashboard — still declined.** Nothing here changes 14's reasoning:
  it needs GitHub authentication and an API dependency in a tool that is currently
  plain git, and gh-dash exists. Opening a pull request for the current branch is
  still accepted and still unbuilt.
- **Editing — still declined, and this is the line.** Nothing in tuigy writes to a
  file's contents. The preview pane is read-only and `e` hands the file over. A tool
  that edits has to own undo, syntax, autosave and everything after; that is the
  decision that would change what this is, and it is not being made.

## Consequences

The tabs renumbered. Files took the first slot and everything moved along:
`1 Files · 2 Changes · 3 Branches · 4 History · 5 Stashes`. This breaks muscle memory
and breaks a configuration file that rebound the tab keys by name. tuigy still opens
on Changes, because watching what an agent just wrote is still what it is for.

The competitive set moves. 14 named lazygit; a git tool with a tree, a shell and a
project switcher is closer to a terminal workspace than to lazygit, and it will be
compared with one.

The binary carries a terminal emulator: about 1 MB on top of 6.8.

Keys are running out. 14 counted 52 actions across 54 keystrokes; there are now 63
across 63, and the single-letter space is nearly gone — folding the tree had to take
`+` and `-` because the letters were spent. The command palette on the roadmap was a
convenience when it was written and is closer to a requirement now.

The surface to keep working grew by three features that have nothing to do with git,
which is the cost 14 was protecting against and which is now being paid on purpose.

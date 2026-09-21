# 14. tuigy is a git tool, not an IDE

Status: superseded in part by [15](0015-a-workspace-around-the-git-tool.md)

The test this record sets, and its reasoning about the pull request dashboard, still
hold. The file tree and the embedded terminal it declines were both later built, and
the closing paragraph here asked for exactly the record that
[15](0015-a-workspace-around-the-git-tool.md) is. Left as written.

## Context

The original brief called tuigy a "terminal UI IDE". The specification written
immediately after opened with the opposite: *managing branches, commits and diffs
without needing an IDE*. That tension has come up more than once, in the form of
reasonable-sounding additions: a file tree, an embedded terminal pane, a pull request
dashboard.

Each of them is a good feature of some product. The question is whether they are good
features of this one.

## Decision

The test a proposal has to pass: **does it make tuigy the fastest way to review and
land what a coding agent just wrote?**

That is the capability actually built — live refresh, review marks, hunk staging,
commit messages from the user's own agent — and it is the one thing tuigy does that
mature alternatives do not.

Measured against it:

**A file tree of the repository — declined.** In a tool that cannot edit, a tree
serves "open this file", which the editor does better. The changed-files list already
is the relevant tree, filtered to what matters and grouped by directory where that
helps.

**An embedded terminal pane — declined.** Not on principle but on cost and fit. tuigy
binds 52 actions across 54 keystrokes; a shell needs all of them plus ctrl+c,
ctrl+d, ctrl+u and tab. Hosting one means a focus model where every key goes to the
child except one escape — which is tmux's prefix, reimplemented worse, inside a git
tool. Underneath it sits a terminal emulator: a PTY, a VT parser, and permanent
maintenance of emulation bugs. The user already has tmux, Zellij, or their terminal's
own splits.

The intent behind it is real, though: not wanting to leave. That is served by handing
the terminal over — suspend, run the command, come back and refresh — which is the
mechanism `e` already uses for the editor and is about thirty lines.

**A pull request dashboard — declined; opening a pull request — accepted, not yet
built.** Pushing is the last step tuigy owns and the pull request is the next one, so
creating one from the current branch fits. A dashboard is gh-dash's job, and building
one would mean adding GitHub authentication and an API dependency to a tool that is
currently plain git and works with GitLab, Gitea or a bare SSH remote. When it is
built it should shell out to `gh`, for the same reasons as
[0006](0006-commit-messages-from-the-users-agent.md).

## Consequences

The product stays small, which is what makes it fast and what makes it possible to
explain in one sentence.

This is a positioning decision, not a technical one. Deciding to become an IDE shell is
open — it just changes the competitive set from lazygit to VS Code, and multiplies the
surface area. It should be made deliberately and recorded here, not arrived at one
feature at a time.

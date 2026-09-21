# tuigy documentation

Written for whoever picks this up next, including the author six months from now.

- [architecture.md](architecture.md) — how the pieces fit together, and the shape of
  the code you will be editing.
- [decisions/](decisions) — why things are the way they are. Each record covers one
  decision that would be expensive to reverse or that looks arbitrary from the outside.
- [roadmap.md](roadmap.md) — what is deliberately not built, and what is next.

## The decisions

| | |
|---|---|
| [1](decisions/0001-drive-git-by-shelling-out.md) | Drive git by shelling out to the binary |
| [2](decisions/0002-bubble-tea-for-the-interface.md) | Bubble Tea for the interface |
| [3](decisions/0003-poll-rather-than-watch.md) | Poll for changes rather than watch the filesystem |
| [4](decisions/0004-check-out-the-target-branch.md) | Check out the target branch for merge and cherry-pick, and say so first |
| [5](decisions/0005-never-surprise-the-user.md) | Pull is fast-forward only, and nothing is stashed unasked |
| [6](decisions/0006-commit-messages-from-the-users-agent.md) | Generate commit messages by running the user's own agent |
| [7](decisions/0007-ask-git-which-editor.md) | Ask git which editor to open, but not for its fallback |
| [8](decisions/0008-configuration-is-optional-but-honoured.md) | Configuration is optional, and a broken file stops startup |
| [9](decisions/0009-themes-are-a-palette-of-roles.md) | Themes are a palette of roles |
| [10](decisions/0010-clip-every-pane.md) | Clip every pane and dialog to its own bounds |
| [11](decisions/0011-cursor-behaviour.md) | The cursor holds still for the agent and steps aside for the user |
| [12](decisions/0012-review-marks-expire.md) | Review marks expire when the file changes |
| [13](decisions/0013-pin-the-coverage-toolchain.md) | Pin the Go version the coverage floor is measured with |
| [14](decisions/0014-what-tuigy-is-not.md) | tuigy is a git tool, not an IDE *(superseded in part by 15)* |
| [15](decisions/0015-a-workspace-around-the-git-tool.md) | A workspace around the git tool |
| [16](decisions/0016-the-terminal-is-a-band.md) | The terminal is a band, and reserves one key |
| [17](decisions/0017-projects-are-remembered.md) | Every repository is remembered, and switching happens in place |
| [18](decisions/0018-bindings-are-changed-where-they-are-read.md) | Key bindings are changed where they are read |
| [19](decisions/0019-the-palette-offers-what-the-keys-would-do.md) | The palette offers what the keys would do, and nothing else |

Start with [15](decisions/0015-a-workspace-around-the-git-tool.md) if you are deciding
whether something belongs in tuigy at all, and then with
[14](decisions/0014-what-tuigy-is-not.md), which it supersedes in part and which still
holds the reasoning it was measured against.
[1](decisions/0001-drive-git-by-shelling-out.md) is where to start if you are changing
how tuigy talks to git.

These are in English for the same reason the code is: the project is open to
contributors who do not read Turkish. [README.tr.md](../README.tr.md) covers using
tuigy in Turkish.

## Writing a decision record

One file per decision, numbered in order, named after the decision rather than the
component. Keep them short: context, the decision, and what it costs. A record is not
a design document — it exists so that nobody has to re-derive the reasoning, or
re-discover the problem it avoids.

Records are not edited when the world moves on. Write a new one and mark the old one
superseded, so the history of the thinking stays readable.

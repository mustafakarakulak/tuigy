# 11. The cursor holds still for the agent and steps aside for the user

Status: accepted

## Context

The file list is rebuilt from scratch every time the status changes. Where the cursor
lands afterwards has to serve two situations that pull in opposite directions.

An agent writing files in another pane adds rows underneath the cursor. If the cursor
followed its index, it would drift onto a different file while the user is reading a
diff.

Staging a file moves it from one section to another. If the cursor followed the file,
staging several files in a row would mean navigating back after each one.

## Decision

After a rebuild, in order:

1. If the file under the cursor has not moved, stay on it.
2. If it changed section — which is what staging does — take the position it vacated
   in the old section.
3. If that section is now empty, follow the file to where it went.

## Consequences

Files appearing in the background never move the selection. Holding space stages a run
of files without touching anything else.

The same reasoning applies to the history and stash lists, which key their selection on
the commit hash and the stash contents rather than on a position.

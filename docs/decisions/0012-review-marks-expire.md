# 12. Review marks expire when the file changes

Status: accepted

## Context

`v` ticks a file off as read. The tick exists because the product's premise is
reviewing what a coding agent wrote, and a long list of changed files needs a way to
keep your place.

A tick that survives the agent rewriting the file is worse than no tick at all: it says
the file has been read when it has not.

The status poll cannot see this. A file modified before and modified after is reported
identically — the letters do not change.

## Decision

Record the file's modification time and size when it is ticked. Re-check them on every
poll, not only when the status changes, and drop the tick when either differs.

## Consequences

The tick means what it says. An agent that rewrites a reviewed file silently undoes the
review, and the count in the header goes down.

Hashing the contents would be exact where this is merely reliable, but it would run
against every reviewed file every second. A changed file always changes one of the two.

The marks live in memory for the session. They describe a review pass, not a property
of the repository.

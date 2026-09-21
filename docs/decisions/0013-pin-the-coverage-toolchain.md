# 13. Pin the Go version the coverage floor is measured with

Status: accepted

## Context

CI enforces a coverage floor of 90%. The first run after it was added failed at 88.8%
on a commit that measured 91.4% locally — same source, same tests.

Go 1.27 changed how coverage blocks are recorded. Measured on the same commit:

| Go   | reported |
|------|----------|
| 1.24 | 88.8%    |
| 1.25 | 88.8%    |
| 1.26 | 88.8%    |
| 1.27 | 91.4%    |

The coverage job took its Go version from `go.mod`, which names the minimum the module
supports. The gate therefore depended on whichever toolchain happened to run it.

## Decision

The coverage job pins its Go version explicitly. The other jobs continue to use the
version from `go.mod`, because testing the minimum supported toolchain is their point.

The floor and the version it was calibrated against live together in the Makefile, and
CI calls `make cover` rather than repeating the logic.

## Consequences

The number means the same thing every time it is measured, which is the only property
that makes a floor useful.

A contributor on an older Go sees a lower figure. `make cover` says so when it fails,
rather than reporting a number they cannot reconcile.

Raising the pinned version is a deliberate change, and will move the reported figure.

# 6. Generate commit messages by running the user's own agent

Status: accepted

## Context

Writing a commit message from a staged diff is a model call. That can be made by
talking to a provider's API, or by running a coding agent the user already has.

Talking to an API means an API key to store, a provider to choose, billing that is
ours to explain, an SDK to keep current, and a model name that ages.

## Decision

Run a command. The staged diff goes to its standard input and the message is read
from its output.

`claude` is used automatically when it is on `PATH`. Anything else is a line in the
config file or the `TUIGY_AI_COMMIT` environment variable, which takes precedence.

## Consequences

tuigy never holds a credential and never makes a network request. Whoever the user has
already chosen and configured is who answers, including a local model behind a script
of their own.

The message lands in the editable field and nothing is committed until the user
confirms, so a bad answer costs a keystroke.

If no agent is found the feature is not advertised at all, rather than offering a key
that then explains it cannot do the thing.

The cost is that output quality is not ours to control, and that agents wrap answers
in code fences often enough that unwrapping them is part of the work.

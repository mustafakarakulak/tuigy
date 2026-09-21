# 19. The palette offers what the keys would do, and nothing else

Status: accepted

## Context

[15](0015-a-workspace-around-the-git-tool.md) closed by counting the cost of what it
had just let in: 63 actions across 63 keystrokes, the single-letter space gone, and
folding the tree reduced to taking `+` and `-` because the letters had run out. It
recorded the command palette as having moved from a convenience to close to a
requirement.

A palette raises three questions, and the first two have obvious wrong answers.

**What is on the list.** "Everything" is the obvious answer and it is wrong twice
over. Half the key map is movement — up, down, page-down, next-pane, scroll-left —
and a list you scroll past "move down" to reach "push" has made the tool slower. The
other half is worse: most commands act on what the cursor is on, and `stash-pop` on
the branches tab does nothing at all.

**How a command runs.** The key map is a switch on `key.Matches`; there is no table
mapping a name to a behaviour. The palette could synthesise the keystroke the action
is bound to and feed it back through `handleKey`, which would make drift impossible.
But a key means different things in different places — `enter` checks out a branch,
pops a stash and confirms a dialog — so a synthesised keystroke does whatever the
current context does, not what the user picked off the list. Choosing "pop this
stash" and having a branch checked out is precisely the surprise
[5](0005-never-surprise-the-user.md) rules out.

**Which key opens it.** The one key in tuigy that has to be guessable, by definition:
it is what you reach for when you do not know the key.

## Decision

**The palette lists what the keyboard would do from where you are.** A command
belongs either to one tab or to everywhere. On the changes tab it offers committing,
staging and discarding; on the stashes tab it offers popping and dropping; both offer
push, fetch, the terminal, the project switcher and the five tabs. Nothing is listed
that would silently do nothing, so the palette is exactly as truthful as the keyboard
it stands in for — including that a command acting on an empty list does nothing,
which is also what its key does.

The tab's own commands come first. They are the ones that needed the context.

**Movement stays out.** The palette is for commands. Scrolling is done by scrolling.

**Each command carries its own label rather than the binding's description**, because
`delete` removes a branch on one tab and drops a stash on another. The footer already
relabels these per tab, for the same reason and in the same words.

**Each command calls the method its key calls.** Where the key switch held the body
inline — the five tab switches, the unstage of the selection, generating a message,
a branch's history — that body moved into a method both now call. The palette cannot
drift into doing something different from the key, because there is only one
implementation left to do. What it could still drift on is naming an action that has
been renamed, and a test walks every command on every tab asserting the action exists
and has a key to show.

**The key shown beside each command is the point, not decoration.** A palette that
only ran things would leave you using it forever; one that says `c` next to "commit
the staged changes" is teaching its way out of a job.

**`:` opens it.** Not ctrl+k, which is what an editor would use and which this project
has spent three files' worth of examples and tests treating as the spare key nothing
is bound to — taking it would have shadowed the documented `commit: [c, ctrl+k]`, and
the shadowed binding would have silently never fired. tuigy is already vim-flavoured
enough — `j`, `k`, `g`, `G`, `/` — that `:` is the first thing a user of it tries, and
it costs no chord for something reached often.

**Typing beats navigating.** Inside the palette every key that is not an arrow, a
page key, enter or escape types into the filter. `j` and `k` are letters here; a list
you cannot type "checkout" into is not a palette.

**The filter is a subsequence, not a substring.** `sa` finds "stage everything". This
is deliberately not what `/` does to a list: a list filter is typed while looking at
the list, so a substring is what the eye is already reading, while a palette is typed
at from memory of the words.

**A subsequence match is loose, so the matches are ranked.** `st` otherwise reaches
two thirds of the list with the obvious answer somewhere in the middle of it. The
tightest match leads — the one whose letters sit closest together — and between two
equally tight ones, the one that starts earliest: "stage" is contiguous in both "stage
everything" and "commit the staged changes", and only one of them is what was typed.
With nothing typed there is nothing to rank, and the list keeps its order.

## Consequences

There are now two matching rules in tuigy. A third would mean the difference was not
real.

The palette is a second door onto the same rooms, so every new command is two edits
rather than one: the key switch and the table. The alternative was a palette that
could be wrong about what it does, which is worse than one that can be incomplete —
and a command missing from it is visible the moment anyone looks.

Reaching a command that belongs to another tab takes two steps: go to the tab, open
the palette again. That is the cost of never lying about what a line will do. The tab
switches are in the palette for exactly this reason, and they load the tab they arrive
at rather than leaving it empty.

`:` is spent. It was the last punctuation key worth having.

The action count is 64, and the palette is one of them: it can be rebound from the
settings page like anything else. It does not list itself.

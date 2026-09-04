# TUI Overhaul, Piece 2: What Must Be Learned

**Status:** design approved, not yet implemented
**Date:** 2026-09-04

## Goal

Someone opening `dv keys` or the help screen sees, first and separately, only
the keys this application actually teaches — and is told, in that same list,
that one of them reaches all the others by name.

## Why

A user ran Piece 1 and reported three things. Two were fixed there. The third
was:

> There are too many shortcuts to learn.

That is true as the interface presents it, and false as the interface works.
`dv keys` prints 52 lines covering 48 actions, in one undifferentiated run.
But roughly a third of those are keys every editor and every macOS application
already uses — `⌘Z`, `⌘X`, `⌘C`, `⌘V`, `⌘A`, `⌥←`, `Home`, `⌘S`. Nobody
learns those here; they arrive knowing them. Printing them beside `F5` and
`⌘⇧U` is what makes the list look like 48 things to memorise.

And of the rest, one — the command palette — reaches every other by name.
Piece 1 built that (`command.covers`, and a test proving no action is
reachable only through a chord), but the key reference has never said so.

**The problem is the presentation, not the count.**

### Why this is not the prefix mode the roadmap named

Piece 1's roadmap called this piece "escaping key theft — a prefix mode,
`dv keys` extended, conflict diagnosis". Investigation says that names a
means, not the end, and the means answers a different question.

A prefix mode (tmux's `Ctrl+B`) solves **keys the terminal or multiplexer
eats**. It moves 48 actions behind a leader key; there are still 48. It would
also change `Map.Lookup`'s contract from "one event is one action" to a
sequence, which reaches its four consumers in `internal/ui` and `internal/cli`
and sits next to the vim state machine.

Nothing here needs that. Key theft already has answers this repository shipped:
`keymap.TerminalAdviceShort` names a fallback that works, every binding has an
F-key fallback, `dv keys --tmux` emits the tmux settings that let modified keys
through, and `dv keys --debug` reports what the terminal actually sends.

A prefix mode remains a legitimate future piece **if evidence appears that the
F-key fallbacks are not enough**. This piece does not build one, and does not
change any key handling at all.

## Scope

**In.**

- `Action` carries whether its binding is one the user already knows.
- Every bindable action must be classified explicitly; a test enforces it.
- `dv keys` and the help screen present the two groups separately, with the
  familiar one compressed rather than enumerated.
- The key reference says that the palette reaches everything else.
- `README.md`'s Keys section follows.

**Out, deliberately.**

- A prefix mode. See above.
- Removing or rebinding anything. Muscle memory is not the cost being paid
  here, and breaking it to shorten a list would trade a real cost for a
  presentational one.
- Any change to `Map.Lookup`, `Map.Bindings`, `Map.Apply`, or the presets.
  **No key handling changes.**
- `dv keys --debug`. It answers a different question — what the terminal sends
  — and answers it well.
- The first-run card. It is already five entries, one of which is the palette.

## Where the classification lives

`Action` already carries its own metadata, and this follows that pattern
exactly rather than inventing a second one:

```go
// Familiar reports that this action's binding is the one every editor and
// every macOS application already uses, so it is not something dv teaches.
//
// It is a property of the action, not of the chord: a preset may rebind ⌘/,
// and "comment the line" differing between editors is what makes it dv's to
// teach — the shape of the key says nothing about whether it is known.
func (a Action) Familiar() bool { return familiar[a] }
```

`familiar` is a `map[Action]bool` beside `descriptions` (`action.go:147`) and
`reserved` (`:203`).

### The test that keeps it honest

A Go map answers `false` for a key it does not hold. Without a check, an
action added later is **silently classified as dv's own** — safe in direction,
but a classification nobody made.

So, beside the existing rule that every bindable action appears exactly once
in `helpGroups` (`internal/ui/dialog_test.go:15`):

> **Every bindable action must appear in `familiar`, with either value.**

That is the same shape as the three rules this repository already enforces
automatically — the exactly-once help rule, every `vim.Reference()` sequence
typed into a real state machine, and every action reachable without a chord.
This is the fourth, not a new idea.

## The classification

The boundary is **"is this key the same in every application?"** — not "does
it look conventional".

**Familiar — 19 actions.**

| Group | Count | Actions |
|---|---|---|
| Cursor | 8 | word left/right, select word left/right, line start/end, select to line start/end |
| Editing | 6 | undo, redo, cut, copy, paste, select all |
| Deleting | 2 | delete word left, delete to line start |
| Finding | 1 | find in the editor or results |
| Files | 1 | save file |
| Session | 1 | quit |

**dv's own — 29 actions.** Everything else, including four that look
conventional and are not:

| Action | Why it is dv's to teach |
|---|---|
| `⌘/` toggle comment | The chord is conventional; what it does differs between editors. The test is sameness, not shape. |
| `⌘D` duplicate line | VS Code uses the same chord for "select next occurrence". Same key, different result — the most dangerous kind. |
| `⌘Y` delete line | A DataGrip convention, not a universal one. |
| `⇥` / `⇧⇥` next/previous pane | Universal in web forms and GUI dialogs, not in terminal applications. |

`⌘S` (save) and `⌘F` (find) are Familiar: saving is saving, and finding is
finding. That `⌘F` searches the results as well as the editor is a difference
the description carries, not the classification. `⌘⇧F` (search the query
history) is dv's own — searching *history* is not what that chord means
elsewhere.

**When a call is close, classify it as dv's own.** The costs are asymmetric: a
familiar action wrongly marked dv's own adds one line to a list, while an
unfamiliar one wrongly marked familiar is a key the user is never taught. `⇥`
was moved to dv's own during design review for exactly this reason.

## What the surfaces do with it

One classification, three readers, each answering its own question. Because
all three read the same source, they cannot drift.

### `dv keys`

Its job is completeness, so it shows both — ordered and titled so the shorter,
harder list comes first.

```
The 29 that are dv's own
  F5              run the statement under the cursor
  F3              every command by name — this one reaches the rest
  ⌘B              show or hide the schema tree
  …

Already what you expect — 19 keys, nothing new
  ⌘Z ⌘⇧Z undo/redo · ⌘X ⌘C ⌘V cut/copy/paste · ⌘A select all
  ⌥←→ by word · Home/End · ⌥⌫ ⌘⌫ delete · ⌘S save · ⌘Q quit
```

**The familiar block is packed, not enumerated.** Nineteen actions that cost
nothing to learn must not cost nineteen lines — that is the same defect in a
new arrangement. Three lines is the budget.

The `F3` line is the lever this piece turns. The palette has reached every
command by name since Piece 1; the reference has never said so, so a reader
counting keys had no reason to believe the count was optional.

### The help screen (`F1`)

`startHere`'s five stay exactly as they are. Below them, `helpGroups` renders
with the familiar actions packed the same way, so the screen moves closer to
one page.

### The first-run card

Unchanged.

## Package boundaries

The dependency rule holds: `keymap` does not know how anything is displayed,
and the UI does not know why an action was classified.

| File | Change |
|---|---|
| `internal/keymap/action.go` | `familiar` map, `Familiar()` |
| `internal/keymap/action_test.go` | every-action-classified rule; the five judgement calls pinned |
| `internal/keymap/display.go` | a pure function splitting actions by classification |
| `internal/cli/keys.go` | two sections; the familiar block packed |
| `internal/ui/dialog.go` | the same split on the help screen |
| `README.md` | the Keys section follows |

## Testing

Everything here is a pure function over strings. No terminal, no database.

- Every bindable action appears in `familiar`.
- The five judgement calls — `⌘/`, `⌘D`, `⌘Y`, `⇥`, `⌘S` — are pinned
  individually, so a later change to any of them is a decision rather than a
  drift.
- No action appears in both sections of `dv keys`, and none is missing from
  both.
- The familiar block fits in three lines: the packing is the feature, and a
  test that only checked content would pass while it silently unpacked.
- The `F3` entry states that it reaches the rest.
- The help screen's `startHere` five are unchanged.

## What must be true when this is done

- `dv keys` opens with the list of things `dv` actually teaches, and that list
  is 29 lines rather than 52.
- The keys a user already knows are present, findable, and take three lines.
- A reader is told, inside the list, that one key reaches the rest.
- No key handling changed: `Lookup`, `Bindings`, `Apply` and every preset are
  untouched, and the input path in `internal/ui` has no diff.

## Size

One map and one function in `keymap`, a split function, two renderers, a
README section, and somewhat more test than production code.

## Risks and deferred decisions

**The classification is a judgement, and judgements age.** `⌘D` is dv's own
today because VS Code means something else by it; if that changed, the entry
would be wrong and nothing would fail. The pinned tests make each call visible
and deliberate, but they cannot make it true. This is why the tie-break rule
favours teaching.

**Packing is a presentation choice that fights terminal width.** Three lines
of packed keys assume roughly eighty columns. Narrower than that, the block
wraps or truncates. `dv keys` writes to stdout rather than a laid-out screen,
so this is not the status bar's shedding problem — but it has not been tested
below eighty columns and should be, before anyone claims it degrades well.

**A prefix mode is still unbuilt.** If evidence appears that the F-key
fallbacks are insufficient — a terminal that eats those too, or a user who
cannot reach them — that is a separate piece, and this one does not make it
harder to add.

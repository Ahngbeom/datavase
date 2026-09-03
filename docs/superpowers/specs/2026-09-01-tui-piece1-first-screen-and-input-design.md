# TUI Overhaul, Piece 1: The First Screen and the Ways In

**Status:** design approved, not yet implemented
**Date:** 2026-09-01

## Goal

Someone who has never run `dv` should be able to type, find what they can do
from where they are standing, and learn the key for it in the same glance —
without reading anything first. Someone who has used it for a year should not
pay a single row of screen, a single keystroke, or a single frame for that.

## Why

`dv` is a terminal SQL client competing with DataGrip and DBeaver for
engineers who already have a working tool. Three costs keep it from being
picked up:

- **The editor is modal out of the box.** Someone arriving from a GUI client
  opens `dv` and cannot type. `CLAUDE.md` lists five safeguards that exist
  only to make that default survivable, which is itself the evidence that the
  default is being paid for.
- **The mouse means almost nothing.** `app.go:435` calls `EnableMouse(true)`
  and nothing else in this repository interprets a click. Every discovery path
  runs through a key the user has to already know: `F1` for the reference,
  `⌘⇧A` for the palette, and the first-run card, which is shown once.
- **"What can I do here" has no answer in place.** The palette answers it, but
  only for someone who knows the palette exists and can reach its chord. The
  comment on `command.covers` in `palette.go` records the day this went wrong
  in earnest: a terminal that kept `⌘` for its own menus left the schema tree
  with no way in at all.

### What was benchmarked

`herdr` — a Rust terminal runtime for coding agents — was read for this. Its
*runtime* shape was already borrowed once; that is the subject of
`2026-08-27-dv-runtime-design.md` and is done. What is borrowed here is its
*interface* strategy, which is a different thing:

- **The mouse is the discovery surface, and the keyboard is the graduation.**
  herdr's docs introduce clicking first and present prefix keys as optional.
  Its context menus name the shortcut beside the command, so using the mouse
  teaches the key.
- **Progressive disclosure, addressed at two audiences at once.** herdr says
  out loud that it serves both people who have never used a multiplexer and
  people who already know the model, and it does not make the second group pay
  for the first.

What was deliberately **not** borrowed:

- **A permanent sidebar of state.** herdr's sidebar exists to show many
  agents' states at once. `dv` has no such plurality, and a permanent bar is a
  row taxed on every session forever. See "Hints" below for what replaces it.
- **A prefix key mode.** It is the right answer to keys being eaten by the
  terminal, and it is Piece 2 of the roadmap, not this piece.
- **A plugin marketplace.** No.

## The roadmap this belongs to

Five pieces, ordered so that each one's tests become the next one's regression
net, and so that the pieces touching the lower packages come last.

| # | Piece | Solves | Touches |
|---|---|---|---|
| **1** | **The first screen and the ways in** (this spec) | first five minutes; cost of leaving a GUI client | `ui` only, plus a default in `keymap` and `config` |
| 2 | Escaping key theft — a prefix mode, `dv keys` extended, conflict diagnosis | keys eaten by the terminal or multiplexer | `keymap` interface |
| 3 | The workbench — N open buffers, each carrying its own result, scroll and sort; the editor header becomes a tab strip | only one result at a time; grouping related queries | `result` ownership |
| 4 | Knowing while away — completion notifications, events queued while detached | not knowing a long statement finished | `snapshot`, `ui` |
| 5 | The cost of moving — datasource and schema switching, and whether a buffer can carry its own datasource | switching cost; a piece of work that spans two datasources | `session`, `ui` |

Piece 3 was originally scoped as "multiple result tabs". It is stated above as
the workbench because the need behind it — keeping several related queries
under one roof — is already answered by the worktree: a query is code, code is
a file, a file has a name, and a directory groups it. An in-application
"session" list would be a worse copy of the filesystem, would collide with the
two existing meanings of *session* in this codebase (`session.Session`, the
open datasource; and the runtime session the daemon holds), and would reverse
a scope decision the runtime spec made deliberately. What is actually missing
is not naming but simultaneity, and that is what the workbench supplies.

Cross-datasource grouping is the one thing the worktree cannot express. It is
named in Piece 5 rather than solved here.

## Scope

**In.**

- The default keyboard preset becomes `datagrip`, without silently changing
  anyone's existing session.
- Screen regions publish where they drew what, so a click can be resolved.
- Left click means something on the top bar, region headers, result column
  headers, the tree, and the status bar.
- Right click opens a context menu generated from the palette's own command
  list, with the current key beside each entry.
- The status bar's existing one-shot greeting generalises into context hints
  that appear only when the bar has nothing else to say.
- The mouse can be turned off, losing discovery paths and no function.

**Out, deliberately.**

- Drag: resizing panes, dragging column widths, drag-selecting cells. Drag
  carries state across events, and that state is where a mouse layer stops
  being small. Revisit after this ships.
- A permanent hint bar or sidebar. See "Hints".
- Any change to `proto`, `daemon`, `screen`, `db`, or `snapshot`. Mouse events
  already cross the runtime boundary; see below.
- Themes, notifications, config reload. Later pieces or not at all.

## The mouse already crosses the runtime

Nothing in the protocol needs to change. The path exists end to end today:

```
client terminal → attach.go:126  (*tcell.EventMouse)
                → proto.KindMouse (proto.go:29, Mouse at proto.go:96)
                → daemon/serve.go:286  PostEventWait(tcell.NewEventMouse(...))
                → the server's ui.App
```

`screen.Screen.EnableMouse` is a no-op (`screen.go:284`) because the client
turns mouse reporting on for its own real terminal (`attach.go:56`); it is not
a gap. This piece is therefore confined to `internal/ui` plus two default
values.

## Zones: knowing what was drawn where

`topBarState.renderWidth` (`topbar.go:52`) degrades through `topBarForms`
until the line fits, and `tabbed` abbreviates its header to the width it was
last drawn at (`panel.go`). Only the renderer knows which columns hold what.

So the renderer says so:

```go
// zone is a run of columns on one row that means something when clicked.
type zone struct {
	from, to int        // columns, half-open
	target   zoneTarget
}

func (t topBarState) renderWidth(width int) (string, []zone)
func (t *tabbed) headerZones() []zone
```

Computing the zones in the same pass as the string is the whole point: a
degradation that drops the datasource name has to drop its zone in the same
breath, or a click lands on whatever moved into those columns. Both functions
stay pure and are unit-tested without a terminal, which is what
`topbar_test.go`, `status_test.go` and `panel_test.go` already do.

`ui` holds a `hitmap` — the zones of the last frame, by row — that the mouse
capture consults.

**`headerZones` must not assume two tabs.** `tabbed` already carries
`names []string`. Writing the zone walk against that list rather than against
`results`/`ddl` costs nothing now and is what makes Piece 3's tab strip nearly
free in the UI.

## Clicks

A single `a.app.SetMouseCapture`, mirroring the key router at `app.go:748`.
It is global for the same reason the key router is: the primitive that
receives a click is often not the one that should act. `tabbed`'s header is a
`TextView` that cannot take focus, and clicking a tab name there has to change
the `Pages` below it.

`fireMouseActions` (tview `application.go:538`) calls the capture **before**
the primitives and forwards nothing if the capture returns a nil event. So the
rule is: **a click that lands in a zone is claimed and swallowed; every other
click is passed through untouched.** Row selection and scrolling are already
correct in tview and are left alone. This is what keeps the layer small.

Clicks resolve to existing `keymap.Action`s wherever one exists, so the mouse
path and the key path arrive at the same function. The integration tests
already written as `h.do(action)` then serve as the mouse's regression net,
and the mouse cannot come to do more or less than the keyboard.

| Target | Result |
|---|---|
| Top bar, datasource name | `ActionSwitchDataSource` |
| Top bar, schema name | `ActionUseSchema` |
| Top bar, `F1 keys` | `ActionHelp` |
| Region header, a tab name | switch to *that* tab — named, not cycled |
| Region header, the region's name | focus that region; `▌` moves |
| Result column header | `ActionSortColumn` on that column, same toggle as the key |
| Result row, double click | `ActionInspect` |
| Tree node, tables list row | what Enter does there |
| Status bar, the mode field | switch keyboard (`keymap …`) |
| Status bar, the writes-unlocked notice | `disable writes` |

### What is deliberately not clickable

**The environment spine and the top bar's environment chip.** `theme.go` says
the environment colour is the one visual thing standing between the user and a
production mistake, and `spine.go` says it is the only colour that ignores the
terminal's palette. A warning that is also a control means a misclick on the
production marker looks like it changed the environment. Nothing is lost:
switching datasource is on the name immediately beside it.

**The branch name.** The worktree is decided outside the session; `dv` reports
it and does not set it.

## The context menu

`command` (`palette.go:76`) gains one field:

```go
// contexts names the places this command is offered on right-click. Empty
// means the palette only: a command that makes sense everywhere makes a
// menu longer without making it more useful.
contexts []menuContext
```

Six contexts: `ctxEditor`, `ctxResult`, `ctxTree`, `ctxTables`, `ctxTopBar`,
`ctxStatusBar`. The hitmap already knows which one a click landed in.

**There are no menu-only commands.** Every entry is an element of
`paletteCommands()`, so anything seen in a menu can be found again by name in
the palette and on the `:` line. This is the reason for choosing a
context-filtered palette over per-widget menu tables: four menu tables would
be four more things to go stale, which is what `startHere`, `vim.Reference()`
and `helpGroups` were each built to avoid.

**Each row carries the current key**, produced by `a.keyLabel(cmd.covers)` at
draw time — never written out, for the same reason the first-run card and the
key reference are generated. A command with no `covers` leaves the column
blank and the menu's last line says how to reach it from the palette. This is
the graduation path: discover with the mouse, read the key in place, use the
key next time.

**The widget is new; the data is not.** `palette.go` is a searchable list
dialog with an input field. A menu is a different object: no search, positioned
at the click, half a dozen rows, arrows and Enter and Esc. `menu.go` is a new
widget that shares the command list and the dispatch, not the UI. It flips
about the click point rather than running off the screen, and on a narrow
terminal **it drops the key column before it abbreviates a name** — the key is
recoverable from the palette; the name is the whole of the row.

### Commands that gain a name

`gridcopy.go` has `copyCell` (line 91) and `copyRow` (line 105); only `copy
row` is a palette command. `copyCell` is reachable only through `⌘C` splitting
on context (`ActionCopyOrCancel`). Putting it in a menu requires giving it a
name, and a name makes it reachable from the palette and the `:` line too.

Three commands are added and no more:

- `copy cell` — the unnamed half of `ActionCopyOrCancel`.
- clearing a column's sort, which `ActionSortColumn` can reach by toggling
  but cannot be asked for directly.
- `use this schema`, on a tree node. Selecting a schema in the tree already
  points completion and the tables tab at it; it does not issue `USE`, which
  is what the `●` marks and what an unqualified statement follows. The menu
  entry is that second thing, on the node already under the pointer.

Inventing commands to fill a menu is the failure mode here, and three is the
budget.

## Hints

**No new bar.** herdr uses a permanent sidebar because it has many agents to
report on; `dv` has one session, and a permanent row is taxed forever on
people who stopped needing it in week one.

The mechanism already exists. `status.opening []string` (`status.go:64`) is a
list of independent clauses, most important first, dropped whole rather than
cut — its comment explains that a list loses far less to a dropped clause than
a sentence loses to being chopped. And `app.go:1233` clears it the moment
anything else happens, because keeping it would leave the advice competing
with what the bar is actually for.

That rule is right. Only its content is too narrow. `opening` becomes
`hints`, populated from **the focused region's context**, filtered from the
same `paletteCommands()` the menu uses, with the same generated key labels.

**When hints come back.** Once displaced, hints return on the first focus of a
region this session, and not afterwards. A beginner meets each region once and
is told what is there; someone working never sees them, because after the
first statement the bar always has a row count or an error to carry instead.
The bit is per-region and in memory only — `internal/intro` keeps its
equivalent as a file's existence because it must survive a restart, and this
one must not.

**Terminal advice outranks context hints.** `keymap.TerminalAdviceShort`
(`app.go:329`) warns at startup when the terminal cannot deliver the primary
bindings, and its comment gives the reason: finding out that `Ctrl+Enter` does
nothing by pressing it is the worst way to learn it. That clause stays first
in the list. Piece 2 extends it; this piece only pins the order.

## The first-run card

`startHere` (`dialog.go:188`) keeps its five actions; each has a stated reason
and each still holds. One generated line is added saying that right-clicking
shows what can be done where the pointer is, with the key beside it. That one
line is the entrance to everything else in this piece, which is worth more
than a sixth action.

## Turning the mouse off

Mouse reporting disables the terminal's own text selection. For someone who
copies by dragging, meaningful clicks are a regression, not a feature.

`defaults` gains `mouse: true|false`, and the palette gains `mouse off` /
`mouse on`. With it off, zones, menus and hints go quiet and **no function is
lost** — which is the dividend of resolving every click to an action or a
palette command. Most terminals restore native selection under `Shift`-drag,
and the hint says so once.

## The default keyboard

`keymap.DefaultPreset` (`preset.go:34`) becomes `PresetDataGrip`. `Map.Modal()`
follows it, so the editor types when it is not asked to be modal.

Nothing about the vim keyboard changes. `keymap vim` is already a palette
command (`palette.go:417`), so switching mid-session works today, and all five
safeguards `CLAUDE.md` lists stay in place for the people who choose it. One
is added: the status bar's mode field also names the way to the other
keyboard, generated by `a.keyLabel` rather than written out.

### Not silently

`config.Keymap.Preset` empty means the default (`config/keymap.go:15`), so
changing the constant would change the keyboard under anyone who hand-wrote a
config without `preset:`. `dv init` always writes one explicitly
(`init.go:336`), so the set is small — but a silent change is not acceptable
at any size.

Parsing therefore distinguishes *omitted* from *empty*. A session that loaded
an omitted preset says so once, in the opening clauses, and names the palette
command that goes back. `dv init` is unchanged: it asks, and its cursor starts
on `DefaultPreset`, which now means datagrip. `presetDescriptions`
(`init.go:81`) is already accurate for both.

### The documents change in the same commit

`CLAUDE.md`'s "**The default preset is `vim`,** so the editor is modal out of
the box" paragraph and the safeguard list it introduces, and `README.md`'s
"The editor is modal by default" paragraph, are part of this change. Both use
the modal default as the justification for other decisions; left alone, they
become the stated grounds for something that is no longer true.

## Package boundaries

No new package. `command` carries `run func(a *App)` and cannot leave `ui`;
zones are `ui` widget coordinates. The pure parts are separated into functions
tested without a terminal, which is the local pattern already.

| File | Change |
|---|---|
| `ui/zone.go` | new — zone, hitmap, pure hit testing |
| `ui/mouse.go` | new — the global capture, click → action |
| `ui/menu.go` | new — the context menu widget and its pure layout |
| `ui/topbar.go` | `renderWidth` returns zones |
| `ui/panel.go` | `headerZones`, written against `names` |
| `ui/status.go` | `opening` → `hints`; per-region first visit |
| `ui/palette.go` | `contexts`; three new commands; `mouse on`/`mouse off` |
| `ui/intro.go`, `ui/dialog.go` | the card's mouse line |
| `ui/app.go` | install the capture; hint population on focus change |
| `keymap/preset.go` | `DefaultPreset` |
| `config/keymap.go`, `config/config.go` | omitted-vs-empty preset; `mouse` |

## Build order

Each step is green on its own and revertable on its own.

1. **The default preset, and the documents.** Independent of everything else
   and shippable by itself.
2. **Zones only.** `renderWidth` and `headerZones` return coordinates; unit
   tests check the boundaries against the rendered string in every degradation
   form. **No user-visible change.** Deliberate: this is the step where being
   right about columns is the entire content, and mixing behaviour in would
   let a wrong coordinate present itself as a strange behaviour.
3. **Left click.** Capture, hitmap, action translation, `h.clickZone`.
4. **Right click.** `contexts`, `menu.go`, the three commands, the
   discoverability tests.
5. **Hints and the card line.**

TDD throughout: the failing test first, and watched failing for the right
reason. Step 3 is the one to be suspicious of — if a test goes from build
failure straight to green, shift a zone boundary by one column and confirm it
bites.

## Testing

**Pure units, no terminal.**

- Zone boundaries agree with the visible columns of the rendered string, in
  every `topBarForm` and at every abbreviation `tabbed` performs.
- Menu filtering by context; the key column dropping before a name is
  abbreviated; the flip at a screen edge.
- Hints appear only at `phaseIdle` with no message, warning or error; a clause
  is dropped whole; terminal advice stays ahead of context hints.
- `FromConfig("", nil)` yields datagrip and `Modal()` is false.
- An omitted preset is distinguishable from an empty one.

**Checked automatically, alongside the two rules that already are** — every
bindable action appears exactly once in `helpGroups`; every sequence in
`vim.Reference()` is typed into a real state machine:

- Every `contexts` element names a context that exists. A typo would show up
  in no menu and be silent otherwise.
- No context has an empty menu. A right click that opens nothing reads as a
  dead region.

**Integration, on the `SimulationScreen`.** `InjectMouse(x, y, buttons, mod)`
exists (`tcell simulation.go:339`), so the existing harness carries this with
no new infrastructure.

- `h.clickZone(target)` presses a zone **by name**, for the reason `h.do`
  names actions rather than keys: a layout change must not invalidate a
  behavioural test. `h.click(x, y)` is used only by the tests that verify the
  zone mapping itself.
- Clicking a column header and `h.do(ActionSortColumn)` produce the same row
  order. If the two paths ever diverge, this is where it shows.
- A menu entry's key label follows a rebound harness, proving it is generated.
- Choosing a command from the menu and from the palette produce the same
  result.
- Focusing a region for the first time shows its hints; running a statement
  clears them; returning to that region does not bring them back.
- A session loaded from an omitted preset says so once, and stops saying it
  after `keymap vim`.
- The shared harness keeps pinning `datagrip` explicitly even though it is now
  the default, so those tests keep meaning what they were written to mean if
  the default moves again.

## What must be true when this is done

- Someone who runs `dv` with no config types without reading anything, finds
  what they can do by right-clicking, and reads the key there.
- Everything the mouse can do, the keyboard can do, and the reverse — enforced
  by tests, not by care.
- Turning the mouse off costs discovery paths and no function.
- A vim user's session is exactly what it is today.
- Four rules are now checked automatically instead of two.

## Size

Three new files in `ui`, roughly 500–700 lines, with somewhat more test than
that; eight existing files changed. `proto`, `daemon`, `screen`, `db` and
`snapshot` are untouched.

## Risks and deferred decisions

**Zones are recomputed per frame and read on click.** The hitmap is written
during draw and read from the event goroutine. tview serialises both through
its own loop today, but this is the one place in this design where a
concurrency assumption is being made rather than avoided, and it should be
confirmed against `QueueUpdateDraw` before step 3 is called done.

**Double click to inspect.** tview reports double clicks, but the interval is
tview's and not configurable. If it proves unreliable across terminals, the
row's context menu already carries `inspect` and the double click can be
dropped without losing the capability.

**`mouse: false` and the runtime split.** The setting lives on the server,
which is where `ui.App` runs, while `EnableMouse` is called by the client
(`attach.go:56`). Turning the mouse off in configuration therefore stops `dv`
interpreting clicks but does not stop the client requesting mouse reporting,
so the terminal's native selection stays disabled. Making the client honour it
needs a handshake field and belongs to whichever piece next opens `proto` —
until then, `mouse: false` is documented as "dv ignores the mouse", not "your
terminal gets its selection back".

**Three new commands is a budget, not a measurement.** If implementing the
menus shows a context genuinely bare, the answer is to widen an existing
command's `contexts`, not to invent a fourth command without saying so here.

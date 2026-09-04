# Piece 2: What Must Be Learned — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `dv keys` and the help screen show the 30 keys `dv` actually teaches first, pack the 16 a user already knows into three lines, and say inside that list that the palette reaches everything else.

**Architecture:** `Action` gains a `Familiar()` classification beside its existing `Describe()` and `Reserved()`, backed by a `familiar` map that a test requires every bindable action to appear in. A pure splitting function in `keymap` divides the action list by that classification; `internal/cli/keys.go` and `internal/ui/dialog.go` each render the two groups their own way. **No key handling changes** — `Lookup`, `Bindings`, `Apply` and the presets are untouched.

**Tech Stack:** Go (CGO_ENABLED=0), `github.com/gdamore/tcell/v2`, `github.com/rivo/tview`, `github.com/mattn/go-runewidth`. Tests: standard `testing`; everything in this plan is a pure function over strings and needs no terminal and no database.

**Spec:** `docs/superpowers/specs/2026-09-04-piece2-what-must-be-learned-design.md`

## Global Constraints

Every task's requirements implicitly include this section.

- **No key handling changes.** Do not touch `Map.Lookup`, `Map.Bindings`, `Map.Apply`, `internal/keymap/preset.go`, or any binding table. If a change seems to need one, stop and report — it is out of scope.
- **The classification is 16 familiar / 30 dv's own**, totalling the 46 actions `AllActions()` returns. (`dv keys` prints 52 lines; the tail is the vim reference, not actions.) The judgement calls are fixed by the spec: `⌘/` (toggle comment), `⌘D` (duplicate line), `⌘Y` (delete line), `⇥`/`⇧⇥` (next/previous pane) and `⌘C` (`CopyOrCancel` — with nothing selected it cancels the running statement) are **dv's own**; `⌘S` (save) and `⌘F` (find) are **familiar**. `⌘⇧F` (search history) is **dv's own**.
- **There is no undo or redo action.** `tview`'s text area handles both, so they never reach `keymap` and must not appear in the map.
- **When a call is close, classify as dv's own.** The costs are asymmetric: wrongly marking something dv's own adds a line to a list; wrongly marking it familiar means a key is never taught.
- **The familiar block is packed, not enumerated.** Three lines is the budget. Sixteen actions that cost nothing to learn must not cost sixteen lines.
- **`Familiar()` is a property of the action, not the chord.** A preset may rebind anything; the classification does not consult bindings.
- Do not change `dv keys --debug`, `dv keys --tmux`, `startHere` (`internal/ui/dialog.go:188`), or the first-run card.
- **Comments say why, not what** (`CLAUDE.md`), and the rule is strict: a comment earns its place only when it says what *quietly* breaks the next edit at that line. History, counts and diff voice belong in the commit message.
- **Test names state the user-visible consequence**, not the function called.
- **TDD.** Write the failing test first and watch it fail *for the right reason*. If a test goes from build failure straight to green, mutate the implementation and confirm it bites.
- Commit messages are this repository's style: plain imperative sentences, **no** `feat:`/`fix:` prefixes. Each ends with:
  ```
  Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_01ULMfbLEPrZqNVxWjRbHBnK
  ```

**Commands.**

```sh
make test     # unit tests, no database
make lint     # go vet ./... && gofmt -l .
make build    # CGO_ENABLED=0 go build -o dv ./cmd/dv
go test ./internal/keymap/ -run TestName -v
```

Integration tests are not needed for any task here. If you find yourself writing one, stop and report why.

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `internal/keymap/action.go` | the `familiar` map and `Familiar()`, beside `descriptions` and `reserved` | 1 |
| `internal/keymap/action_test.go` | every-action-classified rule; the judgement calls pinned | 1 |
| `internal/keymap/display.go` | `SplitByFamiliarity` — a pure split of the action list | 2 |
| `internal/keymap/display_test.go` | the split's contract | 2 |
| `internal/cli/keys.go` | two sections; the familiar block packed | 3 |
| `internal/cli/keys_test.go` | section order, packing budget, the palette line | 3 |
| `internal/ui/dialog.go` | the same split on the help screen | 4 |
| `internal/ui/dialog_test.go` | help screen keeps its rules and gains the packing | 4 |
| `README.md` | the Keys section follows | 4 |

Four tasks. Task 1 is the data and its enforcement; Task 2 is the pure split; Tasks 3 and 4 are the two renderers. Each is independently reviewable, and Tasks 1–2 ship no user-visible change on their own — deliberately, because getting the classification right is the whole content there.

---

### Task 1: The classification, and the rule that keeps it complete

**Files:**
- Modify: `internal/keymap/action.go` (after `reserved`, around line 203)
- Test: `internal/keymap/action_test.go` (create if absent — check first with `ls internal/keymap/`)

**Interfaces:**
- Consumes: nothing.
- Produces: `func (a Action) Familiar() bool`; the unexported `familiar map[Action]bool`.

- [ ] **Step 1: Write the failing tests**

Create or append to `internal/keymap/action_test.go`:

```go
// A Go map answers false for a key it does not hold, so an action added
// without a decision would silently claim dv teaches it nothing about it.
// The classification has to be made, not defaulted.
func TestEveryActionIsClassified(t *testing.T) {
	for _, a := range AllActions() {
		if _, ok := familiar[a]; !ok {
			t.Errorf("action %q is in no familiarity class; add it to familiar", a)
		}
	}
}

// The classification counts what a user must be taught. If it drifts, the
// key reference silently starts teaching the wrong list.
func TestTheClassificationSplitsFortySixIntoSixteenAndThirty(t *testing.T) {
	var known, ours int
	for _, a := range AllActions() {
		if a.Familiar() {
			known++
			continue
		}
		ours++
	}

	if known != 16 || ours != 30 {
		t.Errorf("classified %d familiar and %d dv's own, want 16 and 30", known, ours)
	}
}

// These five look conventional and are not, or look unusual and are. Each is
// a judgement the design made explicitly; a later change to any of them
// should be a decision rather than a drift.
func TestTheJudgementCallsHold(t *testing.T) {
	for _, tt := range []struct {
		action   Action
		familiar bool
		why      string
	}{
		{ActionToggleComment, false, "the chord is conventional but what it does differs between editors"},
		{ActionDuplicateLine, false, "VS Code uses the same chord to select the next occurrence"},
		{ActionDeleteLine, false, "a DataGrip convention, not a universal one"},
		{ActionNextPane, false, "universal in GUI dialogs, not in terminal applications"},
		{ActionPrevPane, false, "universal in GUI dialogs, not in terminal applications"},
		{ActionCopyOrCancel, false, "with nothing selected ⌘C cancels the running statement"},
		{ActionSaveFile, true, "saving is saving"},
		{ActionFind, true, "finding is finding; that it also searches results is a description, not a class"},
		{ActionSearchHistory, false, "searching history is not what that chord means elsewhere"},
	} {
		if got := tt.action.Familiar(); got != tt.familiar {
			t.Errorf("%q.Familiar() = %v, want %v — %s", tt.action, got, tt.familiar, tt.why)
		}
	}
}
```

- [ ] **Step 2: Run them and watch them fail for the right reason**

Run: `go test ./internal/keymap/ -run "TestEveryActionIsClassified|TestTheClassification|TestTheJudgementCalls" -v`
Expected: FAIL to compile — `undefined: familiar`, `a.Familiar undefined`.

- [ ] **Step 3: Add the map and the method**

In `internal/keymap/action.go`, after the `reserved` map (which ends around line 203) and before `order`:

```go
// familiar says whether an action's binding is the one every editor and every
// macOS application already uses, so it is not something dv teaches.
//
// The test is sameness, not shape: ⌘D looks conventional and means "select
// the next occurrence" in VS Code, which is the most expensive kind of
// difference — a user believes they already know it. Where a call is close,
// it goes here as false, because a key wrongly listed as known is a key
// nobody is taught.
//
// It is a property of the action rather than of the chord. A preset may
// rebind anything; what a reader already knows does not move with it.
var familiar = map[Action]bool{
	// Cursor movement and selection, identical in every text field.
	ActionWordLeft:          true,
	ActionWordRight:         true,
	ActionSelectWordLeft:    true,
	ActionSelectWordRight:   true,
	ActionLineStart:         true,
	ActionLineEnd:           true,
	ActionSelectLineStart:   true,
	ActionSelectLineEnd:     true,
	ActionDeleteWordLeft:    true,
	ActionDeleteToLineStart: true,

	// The clipboard, and the keys that mean the same thing everywhere.
	// ⌘C is deliberately absent: its action is CopyOrCancel.
	ActionCut:       true,
	ActionPaste:     true,
	ActionSelectAll: true,
	ActionSaveFile:  true,
	ActionFind:      true,
	ActionQuit:      true,

	ActionRun:              false,
	ActionRunAll:           false,
	ActionCancel:           false,
	ActionCopyOrCancel:     false,
	ActionToggleComment:    false,
	ActionDuplicateLine:    false,
	ActionDeleteLine:       false,
	ActionNextPane:         false,
	ActionPrevPane:         false,
	ActionToggleSidebar:    false,
	ActionRefreshSchema:    false,
	ActionUseSchema:        false,
	ActionComplete:         false,
	ActionFindNext:         false,
	ActionFindPrev:         false,
	ActionSearchHistory:    false,
	ActionCommandPalette:   false,
	ActionGoToTable:        false,
	ActionFindFile:         false,
	ActionCycleTab:         false,
	ActionInspect:          false,
	ActionSortColumn:       false,
	ActionSwitchDataSource: false,
	ActionExplain:          false,
	ActionAnalyze:          false,
	ActionSessions:         false,
	ActionKillSession:      false,
	ActionLocks:            false,
	ActionHelp:             false,
	ActionDetach:           false,
}

// Familiar reports that dv does not have to teach this action's key.
func (a Action) Familiar() bool { return familiar[a] }
```

**Check the list against the real enumeration before trusting it.** It must cover exactly the actions `AllActions()` returns — no more, no fewer. `TestEveryActionIsClassified` tells you if an action is missing; `TestTheClassificationSplitsFortySixIntoSixteenAndThirty` tells you if the counts are off. The 46/16/30 figures were measured against this worktree's binary. If what you find differs, **stop and report** rather than adjusting the numbers to fit — a mismatch means an action moved, and that is a decision, not an arithmetic correction.

- [ ] **Step 4: Run them and watch them pass**

Run: `go test ./internal/keymap/ -run "TestEveryActionIsClassified|TestTheClassification|TestTheJudgementCalls" -v`
Expected: PASS

- [ ] **Step 5: Confirm the completeness test bites**

Temporarily delete one entry from `familiar` — pick `ActionHelp`. Run `TestEveryActionIsClassified`; it must fail naming that action. Restore it. Put the failing output in your report: without this, a map with a missing key silently classifies by zero value, which is the defect the test exists to prevent.

- [ ] **Step 6: Whole package and lint**

Run: `make test && make lint`
Expected: PASS, no gofmt output.

- [ ] **Step 7: Commit**

```bash
git add internal/keymap/action.go internal/keymap/action_test.go
git commit -m "$(cat <<'EOF'
Have each action say whether dv has to teach its key

Nineteen of the forty-eight are keys every editor and every macOS
application already uses. Printing them beside F5 and ⌘⇧U is what makes the
reference look like forty-eight things to memorise.

The classification is a property of the action rather than the chord, and a
test requires every bindable action to appear in the map: a Go map answers
false for a key it does not hold, so an action added later would otherwise
be classified by nobody.

Nothing reads it yet.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01ULMfbLEPrZqNVxWjRbHBnK
EOF
)"
```

---

### Task 2: The split, as a pure function

**Files:**
- Modify: `internal/keymap/display.go`
- Test: `internal/keymap/display_test.go`

**Interfaces:**
- Consumes: `Action.Familiar()` from Task 1; `AllActions()` (`action.go:239`).
- Produces: `func SplitByFamiliarity(actions []Action) (ours, known []Action)` — order within each group preserved from the input.

**No user-visible change in this task.** Nothing calls it yet; Tasks 3 and 4 do.

- [ ] **Step 1: Write the failing test**

Append to `internal/keymap/display_test.go`:

```go
// The reference leads with what has to be learned, so the split must keep
// dv's own actions first and must not quietly drop or duplicate any: an
// action in neither group is one nobody can discover.
func TestTheSplitKeepsEveryActionExactlyOnce(t *testing.T) {
	all := AllActions()
	ours, known := SplitByFamiliarity(all)

	if len(ours)+len(known) != len(all) {
		t.Fatalf("split %d actions into %d + %d", len(all), len(ours), len(known))
	}

	seen := make(map[Action]int, len(all))
	for _, a := range ours {
		seen[a]++
		if a.Familiar() {
			t.Errorf("%q is familiar but was put in dv's own", a)
		}
	}
	for _, a := range known {
		seen[a]++
		if !a.Familiar() {
			t.Errorf("%q is dv's own but was put in the familiar group", a)
		}
	}
	for _, a := range all {
		if seen[a] != 1 {
			t.Errorf("%q appears %d times across the two groups, want 1", a, seen[a])
		}
	}
}

// The help screen's grouping is deliberate, so the split must not reorder
// what it is handed.
func TestTheSplitPreservesTheOrderItWasGiven(t *testing.T) {
	in := []Action{ActionRun, ActionCopy, ActionExplain, ActionPaste}
	ours, known := SplitByFamiliarity(in)

	if len(ours) != 2 || ours[0] != ActionRun || ours[1] != ActionExplain {
		t.Errorf("dv's own = %v, want [run explain] in that order", ours)
	}
	if len(known) != 2 || known[0] != ActionCopy || known[1] != ActionPaste {
		t.Errorf("familiar = %v, want [copy paste] in that order", known)
	}
}
```

- [ ] **Step 2: Run and watch it fail**

Run: `go test ./internal/keymap/ -run TestTheSplit -v`
Expected: FAIL to compile — `undefined: SplitByFamiliarity`.

- [ ] **Step 3: Write the function**

Append to `internal/keymap/display.go`:

```go
// SplitByFamiliarity divides actions into the ones dv has to teach and the
// ones a reader already knows, keeping each group in the order given.
//
// The order is the caller's: the help screen groups by purpose and `dv keys`
// follows the reference order, and neither wants this to have an opinion.
func SplitByFamiliarity(actions []Action) (ours, known []Action) {
	for _, a := range actions {
		if a.Familiar() {
			known = append(known, a)
			continue
		}
		ours = append(ours, a)
	}
	return ours, known
}
```

- [ ] **Step 4: Run and watch it pass**

Run: `go test ./internal/keymap/ -run TestTheSplit -v`
Expected: PASS

- [ ] **Step 5: Confirm the test bites**

Temporarily invert the condition (`if !a.Familiar()`). `TestTheSplitKeepsEveryActionExactlyOnce` must fail on both membership assertions. Revert.

- [ ] **Step 6: Whole package and lint**

Run: `make test && make lint`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/keymap/display.go internal/keymap/display_test.go
git commit -m "$(cat <<'EOF'
Split the action list by what has to be learned

A pure function over the classification, with the caller's order preserved:
the help screen groups by purpose and dv keys follows the reference order,
and neither wants this deciding for them.

Still nothing reads it; the two renderers follow.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01ULMfbLEPrZqNVxWjRbHBnK
EOF
)"
```

---

### Task 3: `dv keys` leads with what dv teaches

**Files:**
- Modify: `internal/cli/keys.go` (the render loop is at `:87-102`)
- Test: `internal/cli/keys_test.go`

**Interfaces:**
- Consumes: `SplitByFamiliarity` (Task 2), `Action.Familiar()` (Task 1).
- Produces: no exported shape. The output gains two headed sections.

The current loop walks `keymap.AllActions()` and prints one padded line per action (`keys.go:87-102`). Read it before changing it: the padding uses `keymap.PadLabel` with display width rather than `%-Ns`, because `⌘` and `⇥` take more than one cell.

- [ ] **Step 1: Write the failing tests**

Append to `internal/cli/keys_test.go` (read the file first and match how it captures output — there are existing tests over this command):

```go
// The list a reader has to learn comes first. Behind it, the keys they
// already know are packed rather than enumerated: nineteen actions that cost
// nothing to learn must not cost nineteen lines, which is the same defect
// this change exists to remove.
func TestKeysLeadsWithWhatDVTeachesAndPacksTheRest(t *testing.T) {
	out := runKeys(t)

	ours := strings.Index(out, "that are dv's own")
	known := strings.Index(out, "Already what you expect")
	if ours < 0 || known < 0 {
		t.Fatalf("the two sections are not both present:\n%s", out)
	}
	if ours > known {
		t.Errorf("the familiar keys come before the ones dv teaches:\n%s", out)
	}

	packed := out[known:]
	if end := strings.Index(packed, "\n\n"); end > 0 {
		packed = packed[:end]
	}
	if lines := strings.Count(strings.TrimSpace(packed), "\n"); lines > 3 {
		t.Errorf("the familiar block runs to %d lines, want at most 3:\n%s", lines+1, packed)
	}
}

// One key reaches every command by name, and a reader counting keys has no
// reason to believe the count is optional unless the list says so.
func TestKeysSaysThePaletteReachesTheRest(t *testing.T) {
	out := runKeys(t)

	i := strings.Index(out, "that are dv's own")
	if i < 0 {
		t.Fatalf("no section for what dv teaches:\n%s", out)
	}
	if !strings.Contains(out[i:], "reaches the rest") {
		t.Errorf("the list never says one key reaches the others:\n%s", out)
	}
}

// Every action stays reachable in the reference: one dropped from both
// sections is one nobody can discover.
func TestKeysStillListsEveryAction(t *testing.T) {
	out := runKeys(t)

	for _, a := range keymap.AllActions() {
		if !strings.Contains(out, a.Describe()) {
			t.Errorf("%q is missing from dv keys:\n%s", a, out)
		}
	}
}
```

`runKeys` is a helper that runs the command and returns stdout. If `keys_test.go` already has one under another name, use that instead of adding a second; if it has none, write it in the shape the file's existing tests use.

- [ ] **Step 2: Run and watch them fail**

Run: `go test ./internal/cli/ -run TestKeys -v`
Expected: FAIL — the section headings are absent.

- [ ] **Step 3: Rewrite the render loop**

Replace the loop at `keys.go:87-102`. Keep `PadLabel`, the `Reserved()` note, and the existing trailing blocks (`printVimKeys`, the terminal advice, the `⌘` forwarding hint) exactly as they are.

```go
	ours, known := keymap.SplitByFamiliarity(keymap.AllActions())

	fmt.Fprintf(a.Out, "The %d that are dv's own\n", len(ours))
	for _, action := range ours {
		labels := make([]string, 0, 3)
		for _, b := range km.DisplayBindings(action) {
			labels = append(labels, b.Label(mac))
		}

		note := ""
		if action.Reserved() {
			note = "  (not built yet)"
		}
		if action == keymap.ActionCommandPalette {
			note = "  ← this one reaches the rest"
		}
		// Padded by display width rather than rune count: ⌘ and ⇥ take more
		// than one cell, and %-Ns would leave the column ragged.
		fmt.Fprintf(a.Out, "  %s  %s%s\n",
			keymap.PadLabel(strings.Join(labels, "  "), keyColumn),
			action.Describe(), note)
	}

	fmt.Fprintf(a.Out, "\nAlready what you expect — %d keys, nothing new\n", len(known))
	for _, line := range packFamiliar(km, known, mac) {
		fmt.Fprintf(a.Out, "  %s\n", line)
	}
```

Then write `packFamiliar` in the same file:

```go
// packFamiliar renders the keys a reader already knows as a few dense lines.
//
// One line each would spend sixteen rows on the half of the reference that
// teaches nothing, which is the shape this section exists to replace. The
// width is the conventional terminal's, because `dv keys` writes to a pipe
// as often as to a screen and has no rect to ask.
func packFamiliar(km *keymap.Map, actions []keymap.Action, mac bool) []string {
	const width = 76

	var (
		lines   []string
		current string
	)
	for _, a := range actions {
		labels := make([]string, 0, 3)
		for _, b := range km.DisplayBindings(a) {
			labels = append(labels, b.Label(mac))
		}
		entry := strings.Join(labels, " ") + " " + a.Describe()

		switch {
		case current == "":
			current = entry
		case keymap.LabelWidth(current)+3+keymap.LabelWidth(entry) <= width:
			current += " · " + entry
		default:
			lines = append(lines, current)
			current = entry
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
```

If nineteen entries do not fit in three lines at width 76, **do not widen the budget to make the test pass**. Shorten what each entry says — the description is `Describe()`'s full sentence and the packed block does not need it; a shorter form ("undo", "cut") is what the spec's example shows. Report which you did.

- [ ] **Step 4: Run and watch them pass**

Run: `go test ./internal/cli/ -run TestKeys -v`
Expected: PASS

- [ ] **Step 5: Read the real output**

Run: `make build && ./dv keys`
Read it as someone meeting `dv` today. Put the full output in your report. If the packed block wraps in your own terminal, say so — the width is a guess this task is allowed to correct with evidence.

- [ ] **Step 6: Whole suite and lint**

Run: `make test && make lint`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/cli/keys.go internal/cli/keys_test.go
git commit -m "$(cat <<'EOF'
Lead dv keys with the keys dv actually teaches

The reference printed forty-eight actions in one undifferentiated run, so
the nineteen a reader already knows padded the list they had to learn. Those
nineteen now sit behind the twenty-nine, packed into a few lines instead of
one row each.

The palette's entry says it reaches the rest. It has since piece 1; the
reference had never mentioned it, so a reader counting keys had no reason to
believe the count was optional.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01ULMfbLEPrZqNVxWjRbHBnK
EOF
)"
```

---

### Task 4: The help screen and the README follow

**Files:**
- Modify: `internal/ui/dialog.go` (the `helpGroups` render loop)
- Modify: `README.md` (the `### Keys` section)
- Test: `internal/ui/dialog_test.go`

**Interfaces:**
- Consumes: `SplitByFamiliarity` (Task 2), `Action.Familiar()` (Task 1).
- Produces: no exported shape.

`startHere`'s five entries (`dialog.go:188`) do **not** change. The change is below them, in the `helpGroups` loop.

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/dialog_test.go`:

```go
// The help screen exists to be read in one sitting. Keys a reader already
// knows still belong on it — they are findable there — but they must not
// spend a row each pushing what dv teaches off the bottom.
func TestTheHelpScreenPacksTheKeysAReaderAlreadyKnows(t *testing.T) {
	h := newHelpHarness(t)

	body := h.helpText()
	i := strings.Index(body, "Already what you expect")
	if i < 0 {
		t.Fatalf("the help screen has no section for familiar keys:\n%s", body)
	}

	packed := body[i:]
	if end := strings.Index(packed, "\n\n"); end > 0 {
		packed = packed[:end]
	}
	if lines := strings.Count(strings.TrimSpace(packed), "\n"); lines > 3 {
		t.Errorf("the familiar block runs to %d lines on the help screen:\n%s", lines+1, packed)
	}
}
```

`newHelpHarness` and `helpText` stand for however `dialog_test.go` already builds the help body — read the file and use its existing mechanism rather than adding one. The existing `TestEveryActionAppearsOnTheHelpScreen` (`dialog_test.go:15`) tells you how the groups are reached without a terminal.

- [ ] **Step 2: Run and watch it fail**

Run: `go test ./internal/ui/ -run TestTheHelpScreenPacks -v`
Expected: FAIL — the section is absent.

- [ ] **Step 3: Apply the split to the help groups**

In `dialog.go`, the loop is:

```go
	for _, group := range helpGroups {
		fmt.Fprintf(&b, "\n%s\n", headingTag(group.title))

		for _, action := range group.actions {
			line(action)
		}
	}
```

Change it so each group renders only its dv's-own actions, and the familiar ones from every group are collected and packed once at the end, under their own heading. Reuse the packing rather than writing a second one — if `packFamiliar` has to move to `internal/keymap` so both callers reach it, move it, and say so; `internal/cli` importing `internal/ui` is not an option.

Two rules to preserve:
- `TestEveryActionAppearsOnTheHelpScreen` must still pass: every action appears **exactly once** on the screen, wherever it now sits.
- A group whose actions are all familiar must not leave an empty heading behind.

- [ ] **Step 4: Run and watch it pass**

Run: `go test ./internal/ui/ -run "TestTheHelpScreenPacks|TestEveryActionAppears" -v`
Expected: PASS

- [ ] **Step 5: Update the README**

`README.md`'s `### Keys` section carries a table of every binding. Split it the same way: the keys `dv` teaches first, then a short paragraph or compact table for the ones a reader already knows, and a sentence saying the palette reaches every command by name.

Do not restructure the surrounding prose — the paragraphs about `⌘`/`Ctrl` equivalence, the tmux spelling and the impossible combinations all stay.

- [ ] **Step 6: Read the real screen**

Run: `make build`, start `dv` against any datasource, press `F1`. Put a description of what you see in your report — specifically whether the screen now fits without scrolling on an 80×40 terminal, and if not, how far over it runs.

- [ ] **Step 7: Whole suite and lint**

Run: `make test && make lint`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/ui/dialog.go internal/ui/dialog_test.go README.md
git commit -m "$(cat <<'EOF'
Pack the keys a reader already knows on the help screen too

The screen groups by purpose, and every group carried its share of keys that
mean the same thing in every application. They stay findable, collected and
packed once at the foot rather than spending a row each above what dv has to
teach.

The README's key table follows the same split.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01ULMfbLEPrZqNVxWjRbHBnK
EOF
)"
```

---

## Verification of the whole piece

```sh
make lint
make test
make build && ./dv keys
```

The spec's closing claims, each with the test that holds it:

| Claim | Held by |
|---|---|
| `dv keys` opens with what dv teaches, 30 lines not 52 | `TestKeysLeadsWithWhatDVTeachesAndPacksTheRest` (Task 3) |
| The familiar keys are present and take three lines | the packing assertion in the same test, and Task 4's for the help screen |
| A reader is told one key reaches the rest | `TestKeysSaysThePaletteReachesTheRest` (Task 3) |
| Every action stays discoverable | `TestKeysStillListsEveryAction` (Task 3), `TestEveryActionAppearsOnTheHelpScreen` (unchanged) |
| The classification is complete and deliberate | `TestEveryActionIsClassified`, `TestTheJudgementCallsHold` (Task 1) |
| No key handling changed | `git diff --stat` shows no hunk in `map.go`, `binding.go` or `preset.go` |

## Where to stop and report rather than improvise

- **The counts do not come out 16/30 over 46.** The spec's numbers came from the shipped binary. A mismatch means an action was added or removed since; report it rather than adjusting the expected numbers to match what you find.
- **Sixteen entries will not pack into three lines** even with short descriptions. Report the shortest you achieved and let the budget be a decision, not a silent widening.
- **`packFamiliar` needs to move to `internal/keymap`** for Task 4 to reuse it. That is expected and fine — but it makes `keymap` render a line of output, which is a boundary this repository guards. Say so in your report so the reviewer judges it deliberately.

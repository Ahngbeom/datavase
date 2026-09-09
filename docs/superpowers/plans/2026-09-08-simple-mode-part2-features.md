# 초단순 버전 Part 2: 신규 기능과 문서 — 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Part 1로 줄어든 `dv` 위에 결과 전체 복사(Markdown/JSON), 테이블 미리보기(`LIMIT 100`), 앱 안 데이터소스 관리, 설정 없는 첫 실행을 얹고, 문서를 새 제품에 맞춘다.

**Architecture:** 순수 패키지에 먼저 기능을 넣고(`export.Markdown`, `config.Save`), UI는 그것을 배선한다. 데이터소스 목록·폼은 `internal/ui/dspicker.go` 한 컴포넌트이고, 두 호스트가 쓴다: 설정이 없거나 이름 없이 실행했을 때 세션 없이 뜨는 **런처**(`ui.Launch`, 자기 `tview.Application`을 가짐)와, 실행 중 `⌘⇧D`로 여는 **앱 안 다이얼로그**. 스펙이 말한 "세션 없는 `App`" 대신 런처를 두는 이유는, `App`의 거의 모든 메서드가 `a.conn != nil`을 전제하고 그 전제를 하나씩 풀 때 생기는 nil 분기가 새 기능보다 크기 때문이다. 런처는 연결된 세션을 돌려주고, 그 다음은 지금의 `ui.New(sess, ...)` 경로 그대로다.

**Tech Stack:** Go 1.26, tview/tcell (`tview.Form`, `tview.List`), `gopkg.in/yaml.v3`.

**Spec:** `docs/superpowers/specs/2026-09-08-simple-mode-design.md` — Part 1 계획(`2026-09-08-simple-mode-part1-deletion.md`)이 끝난 트리에서 시작한다.

## Global Constraints

- Part 1의 Global Constraints가 그대로 적용된다(CGO-free, `Ctrl`=`⌘`, F키 대체, 13306, 주석은 "왜"만, 커밋 트레일러).
- TDD: 실패하는 테스트를 먼저 쓰고 실패 이유를 확인한 뒤 구현한다. 빌드 실패에서 바로 초록이 된 테스트는 구현을 변형해 무는지 확인한다.
- 비밀번호는 `config.yaml`에 절대 쓰지 않는다. `secret.Store`로만 간다.
- `config.Save`가 파일의 유일한 쓰기 경로다.
- 미리보기 행 수는 상수 100이다. 설정하지 않는다.
- 새 액션은 `internal/ui/dialog.go`의 `helpGroups`에 정확히 한 번 들어가야 한다(`dialog_test.go`가 검사한다). 액션 설명은 다른 설명의 부분 문자열이 되면 안 된다(렌더된 도움말에서 `Describe()`를 세는 테스트가 있다).

## 매 태스크 공통 검증 명령

```sh
go build ./... && go vet ./... && go vet -tags integration ./...
go test ./...
gofmt -l .
```

UI 통합 테스트가 있는 태스크는 MariaDB를 띄워 돌린다.

```sh
make db-up     # 한 번
go test -tags integration ./internal/ui/ -run <이 태스크의 테스트> -v
```

## 파일 구조

| 경로 | 책임 |
|---|---|
| `internal/export/export.go` | `Markdown(w, columns, rows)` 추가 |
| `internal/keymap/action.go`, `map.go` | `ActionCopyResult` |
| `internal/ui/copyresult.go` | 형식 선택창과 결과 전체 복사 |
| `internal/ui/panel.go`, `zone.go`, `mouse.go` | 결과 헤더의 `copy` 존 |
| `internal/ui/preview.go` | `LIMIT 100` 미리보기 SQL과 실행 |
| `internal/config/save.go` | `Write`, `Save`, `Empty` |
| `internal/ui/dsform.go` | 폼 값 → `DataSource` 변환과 검증 (순수, 터미널 없이 테스트) |
| `internal/ui/dspicker.go` | 목록 + 폼 컴포넌트 (`tview.List`, `tview.Form`) |
| `internal/ui/launch.go` | 세션 없이 뜨는 런처 |
| `internal/ui/datasource.go` | 앱 안에서 컴포넌트를 띄우는 쪽 (검색 상자 목록은 제거) |
| `cmd/dv/main.go`, `internal/cli/cli.go` | 런처 배선, 설정 없는 시작 |
| `README.md`, `CLAUDE.md`, `CHANGELOG.md`, `.goreleaser.yaml` | 문서 |

---

### Task 1: `export.Markdown`

**Files:**
- Modify: `internal/export/export.go`, `internal/export/export_test.go`

**Interfaces:**
- Produces: `func Markdown(w io.Writer, columns []string, rows [][]any) error`. 헤더 행, 구분 행, 값 행. 값의 `|`는 `\|`, 개행은 `<br>`, NULL은 빈 칸. 컬럼이 없으면 아무것도 쓰지 않는다.

- [ ] **Step 1: 실패하는 테스트를 쓴다**

`internal/export/export_test.go` 끝에 추가한다.

```go
func TestMarkdownWritesAHeaderASeparatorAndRows(t *testing.T) {
	var buf bytes.Buffer
	if err := Markdown(&buf, sampleColumns(), sampleRows()); err != nil {
		t.Fatalf("Markdown() error = %v, want nil", err)
	}
	want := "| id | email | note |\n" +
		"| --- | --- | --- |\n" +
		"| 1 | a@example.com |  |\n" +
		"| 2 | b@example.com | hello |\n"
	if got := buf.String(); got != want {
		t.Errorf("Markdown() =\n%s\nwant\n%s", got, want)
	}
}

func TestMarkdownEscapesPipesAndNewlines(t *testing.T) {
	var buf bytes.Buffer
	rows := [][]any{{"a|b", "line one\nline two"}}
	if err := Markdown(&buf, []string{"x", "y"}, rows); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); !strings.Contains(got, `| a\|b | line one<br>line two |`) {
		t.Errorf("Markdown() = %q; a pipe or a newline inside a cell breaks the table", got)
	}
}

func TestMarkdownWithNoRowsStillWritesTheHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := Markdown(&buf, []string{"id"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "| id |\n| --- |\n" {
		t.Errorf("Markdown() = %q, want a header and a separator only", got)
	}
}

func TestMarkdownWithNoColumnsWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	if err := Markdown(&buf, nil, nil); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("Markdown() = %q, want nothing: a table with no columns is not a table", buf.String())
	}
}

func TestMarkdownIsFaithfulToTypedValues(t *testing.T) {
	var buf bytes.Buffer
	at := time.Date(2026, 9, 8, 10, 30, 0, 0, time.UTC)
	rows := [][]any{{int64(42), 3.5, true, at, []byte{0xff, 0x00}}}
	if err := Markdown(&buf, []string{"n", "f", "b", "t", "bin"}, rows); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); !strings.Contains(got, "| 42 | 3.5 | 1 | 2026-09-08 10:30:00 | /wA= |") {
		t.Errorf("Markdown() = %q; values must render the way CSV renders them", got)
	}
}
```

- [ ] **Step 2: 실패를 확인한다**

```sh
go test ./internal/export/ -run TestMarkdown
```

기대: `undefined: Markdown`.

- [ ] **Step 3: 구현한다**

`internal/export/export.go`의 패키지 주석을 `// Package export writes result sets to CSV, JSON and Markdown.`으로 바꾸고, `JSON` 아래에 추가한다.

```go
// Markdown writes columns and rows as a pipe table.
//
// A table with no columns is written as nothing at all: "|  |" is not a
// table, and a paste that contains it looks like a bug rather than an empty
// result.
func Markdown(w io.Writer, columns []string, rows [][]any) error {
	if len(columns) == 0 {
		return nil
	}

	var b strings.Builder
	writeRow := func(cells []string) {
		b.WriteString("| ")
		b.WriteString(strings.Join(cells, " | "))
		b.WriteString(" |\n")
	}

	header := make([]string, len(columns))
	separator := make([]string, len(columns))
	for i, name := range columns {
		header[i] = markdownCell(name)
		separator[i] = "---"
	}
	writeRow(header)
	writeRow(separator)

	cells := make([]string, len(columns))
	for _, row := range rows {
		for i := range cells {
			cells[i] = ""
			if i < len(row) {
				cells[i] = markdownCell(plainText(row[i]))
			}
		}
		writeRow(cells)
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// markdownCell keeps a value inside its cell: a pipe would start a new
// column and a newline a new row, and either silently misaligns everything
// after it.
func markdownCell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\r\n", "<br>")
	return strings.ReplaceAll(s, "\n", "<br>")
}
```

`plainText`의 주석 `// plainText renders a value for CSV: faithful, never abbreviated.`를 `// plainText renders a value for CSV and Markdown: faithful, never abbreviated.`로 고친다.

- [ ] **Step 4: 통과를 확인하고 변형해 본다**

```sh
go test ./internal/export/ -run TestMarkdown -v
```

기대: 5개 PASS. `markdownCell`의 `|` 치환을 잠시 빼고 `TestMarkdownEscapesPipesAndNewlines`가 FAIL하는지 본 뒤 되돌린다.

- [ ] **Step 5: 커밋**

```sh
git add internal/export
git commit -m "Export a result as a Markdown table

Pipes and newlines inside a cell are escaped, because either one would
quietly shift every cell after it; NULL is an empty cell, as in CSV."
```

---

### Task 2: `⌘⇧C` — 결과 전체를 Markdown 또는 JSON으로 복사

**Files:**
- Create: `internal/ui/copyresult.go`, `internal/ui/copyresult_test.go`, `internal/ui/copyresult_integration_test.go`
- Modify: `internal/keymap/action.go`, `internal/keymap/map.go`, `internal/keymap/keymap_test.go`, `internal/ui/dialog.go`, `internal/ui/app.go`, `internal/ui/panel.go`, `internal/ui/panel_test.go`, `internal/ui/zone.go`, `internal/ui/mouse.go`

**Interfaces:**
- Consumes: `export.Markdown`, `export.JSON`, `result.Buffer.Columns/Row/RowCount/AtCapacity`, `gridContent.bufferRow`, `(a *App) setClipboard`, `plural`.
- Produces: `keymap.ActionCopyResult`(이름 `"copy-result"`, 바인딩 `⌘⇧C`/`Ctrl+Shift+C`/`F3`). `(a *App) showCopyFormats()`, `(a *App) copyResult(f copyFormat)`, `zoneCopyResult`. `regionHeader`의 시그니처가 `regionHeader(names []string, active int, focused bool, detail string, detailTarget zoneTarget, width int)`가 되고 `tabbed.detailTarget` 필드가 생긴다.

- [ ] **Step 1: 키맵 테스트를 쓴다**

`internal/keymap/keymap_test.go`에 추가한다.

```go
func TestCopyResultIsBoundToCommandShiftC(t *testing.T) {
	m := Default()
	for _, ev := range []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModMeta|tcell.ModShift),
		tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModCtrl|tcell.ModShift),
		tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone),
	} {
		if got := m.Lookup(ev); got != ActionCopyResult {
			t.Errorf("Lookup(%v) = %v, want ActionCopyResult", ev.Name(), got)
		}
	}
	// Plain ⌘C must still be the copy-or-cancel key.
	if got := m.Lookup(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModMeta)); got != ActionCopyOrCancel {
		t.Errorf("⌘C = %v, want ActionCopyOrCancel", got)
	}
}
```

- [ ] **Step 2: 실패를 확인한다**

```sh
go test ./internal/keymap/ -run TestCopyResult
```

기대: `undefined: ActionCopyResult`.

- [ ] **Step 3: 액션을 추가한다**

`internal/keymap/action.go`:
- 상수 목록의 `ActionSortColumn` 다음에 추가한다.

```go
	// ActionCopyResult puts the whole result on the clipboard, in a format
	// chosen when the key is pressed.
	ActionCopyResult
```

- `actionNames`에 `ActionCopyResult: "copy-result",`
- `descriptions`에 `ActionCopyResult: "copy the whole result as Markdown or JSON",`
- `order`에서 `ActionSortColumn` 바로 뒤에 `ActionCopyResult`.

`internal/keymap/map.go`의 `baseMap()`, `SortColumn` 바인딩 아래에 추가한다.

```go
	// Shift+C beside the plain copy key, so "copy more" is the same hand
	// shape as "copy". F3 is the fallback for a terminal that cannot send a
	// shifted control letter.
	m.bind(ActionCopyResult,
		append(ctrlAndCmdRune('c', tcell.ModShift), Binding{Key: tcell.KeyF3})...)
```

`internal/ui/dialog.go`의 `helpGroups` "Results" 그룹을 `[]keymap.Action{keymap.ActionSortColumn, keymap.ActionCopyResult}`로 바꾼다.

- [ ] **Step 4: 키맵과 도움말 테스트 통과를 확인한다**

```sh
go test ./internal/keymap/ ./internal/ui/ -run 'TestCopyResult|TestEveryAction|TestNoTwoActions|TestDefaultBindingsDoNotCollide'
```

기대: PASS.

- [ ] **Step 5: 복사 본체의 단위 테스트를 쓴다**

`internal/ui/copyresult_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/result"
)

func TestResultTextFollowsTheDisplayedOrder(t *testing.T) {
	buf := result.NewBuffer(10)
	buf.SetColumns([]string{"n"}, nil)
	buf.Append([][]any{{int64(3)}, {int64(1)}, {int64(2)}})
	content := newGridContent(buf)
	content.sortBy(0) // ascending

	text, err := resultText(buf, content, formatMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	want := "| n |\n| --- |\n| 1 |\n| 2 |\n| 3 |\n"
	if text != want {
		t.Errorf("resultText() =\n%s\nwant the sorted order\n%s", text, want)
	}
}

func TestResultTextAsJSONIsAnArrayOfObjects(t *testing.T) {
	buf := result.NewBuffer(10)
	buf.SetColumns([]string{"id"}, nil)
	buf.Append([][]any{{int64(7)}})

	text, err := resultText(buf, newGridContent(buf), formatJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, `"id": 7`) {
		t.Errorf("resultText() = %q, want JSON with the row", text)
	}
}

func TestCopySummarySaysWhenTheBufferWasCut(t *testing.T) {
	got := copySummary(3, formatMarkdown, 120, true)
	for _, want := range []string{"3 rows", "Markdown", "120 B", "truncated"} {
		if !strings.Contains(got, want) {
			t.Errorf("copySummary() = %q, want it to mention %q", got, want)
		}
	}
	if got := copySummary(1, formatJSON, 2048, false); strings.Contains(got, "truncated") || !strings.Contains(got, "2.0 KB") {
		t.Errorf("copySummary() = %q", got)
	}
}
```

- [ ] **Step 6: 실패를 확인한다**

```sh
go test ./internal/ui/ -run 'TestResultText|TestCopySummary'
```

기대: `undefined: resultText`, `formatMarkdown`, `copySummary`.

- [ ] **Step 7: `copyresult.go`를 쓴다**

```go
package ui

import (
	"bytes"
	"fmt"

	"github.com/Ahngbeom/datavase/internal/export"
	"github.com/Ahngbeom/datavase/internal/result"
	"github.com/rivo/tview"
)

// copyFormat is how the whole result is written to the clipboard.
type copyFormat int

const (
	formatMarkdown copyFormat = iota
	formatJSON
)

func (f copyFormat) String() string {
	if f == formatJSON {
		return "JSON"
	}
	return "Markdown"
}

const pageCopyFormat = "copy-format"

// showCopyFormats asks which format, then copies.
func (a *App) showCopyFormats() {
	if a.buf.ColumnCount() == 0 {
		a.notice("no result to copy")
		return
	}

	list := tview.NewList().ShowSecondaryText(true)
	list.AddItem("Markdown", "a table, for a document or a chat", 'm', func() {
		a.pages.RemovePage(pageCopyFormat)
		a.app.SetFocus(a.grid)
		a.copyResult(formatMarkdown)
	})
	list.AddItem("JSON", "an array of objects, for a script", 'j', func() {
		a.pages.RemovePage(pageCopyFormat)
		a.app.SetFocus(a.grid)
		a.copyResult(formatJSON)
	})
	list.SetDoneFunc(func() {
		a.pages.RemovePage(pageCopyFormat)
		a.app.SetFocus(a.grid)
	})
	list.SetBorder(true).SetTitle(" copy the result as ")

	a.pages.AddPage(pageCopyFormat, centred(list, 44, 8), true, true)
	a.app.SetFocus(list)
}

// copyResult writes the whole buffer to the clipboard.
func (a *App) copyResult(f copyFormat) {
	text, err := resultText(a.buf, a.content, f)
	if err != nil {
		a.notice("copy failed: " + err.Error())
		return
	}
	a.setClipboard(text)
	a.notice(copySummary(a.buf.RowCount(), f, len(text), a.buf.AtCapacity()))
}

// resultText renders the buffer in the order the grid shows it.
//
// The buffer rather than the screen: the grid truncates long values and
// doubles brackets for the markup parser, and neither belongs in a paste.
// Rows go through the grid's order so that a sorted column copies sorted.
func resultText(buf *result.Buffer, content *gridContent, f copyFormat) (string, error) {
	rows := make([][]any, 0, buf.RowCount())
	for grid := 1; grid <= buf.RowCount(); grid++ {
		rows = append(rows, buf.Row(content.bufferRow(grid)))
	}

	var out bytes.Buffer
	var err error
	switch f {
	case formatJSON:
		err = export.JSON(&out, buf.Columns(), rows)
	default:
		err = export.Markdown(&out, buf.Columns(), rows)
	}
	return out.String(), err
}

// copySummary is the notice after a copy.
//
// The size is stated because OSC 52 has a per-terminal ceiling that is
// exceeded without a word; a reader who pastes half a table can at least see
// that the whole was larger than their terminal passes on.
func copySummary(rows int, f copyFormat, size int, truncated bool) string {
	s := fmt.Sprintf("%s copied as %s (%s)", plural(rows, "row"), f, humanBytes(size))
	if truncated {
		s += " · truncated at buffer_max"
	}
	return s
}

func humanBytes(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
```

`internal/ui/app.go`의 `dispatch()`에 `case keymap.ActionSortColumn:` 아래로 추가한다.

```go
	case keymap.ActionCopyResult:
		a.showCopyFormats()
```

`bufferRow`가 `gridRow-1`을 인덱스로 쓴다는 점(헤더가 0행)에 맞춰 위 루프는 1부터 센다.

- [ ] **Step 8: 단위 테스트 통과를 확인한다**

```sh
go test ./internal/ui/ -run 'TestResultText|TestCopySummary' -v
```

기대: PASS.

- [ ] **Step 9: 결과 헤더에 클릭할 수 있는 `copy`를 단다**

`internal/ui/zone.go`의 `zoneTarget` 상수에 `zoneCopyResult`를 추가한다.

`internal/ui/panel.go`:
- `tabbed`에 필드를 추가한다.

```go
	// detailTarget makes the trailing detail a control. zoneNone leaves it
	// as text.
	detailTarget zoneTarget
```

- `renderHeader()`의 호출을 `regionHeader(t.names, t.active, t.HasFocus(), t.detailText(), t.detailTarget, t.width)`로 바꾼다.
- `regionHeader`의 시그니처에 `detailTarget zoneTarget`을 넣고, detail을 붙이는 블록을 다음으로 바꾼다.

```go
	if room := remaining - used - len(gap); detail != "" && room >= 4 {
		before := visibleCost(line)
		line += gap + tag(colourMuted, result.Truncate(detail, room))
		if detailTarget != zoneNone {
			zones = append(zones, zone{from: before + len(gap), to: visibleCost(line), target: detailTarget, index: -1})
		}
	}
```

`panel_test.go`의 `regionHeader(...)` 호출(5곳)에 `zoneNone` 인자를 끼운다. 그리고 테스트를 하나 추가한다.

```go
func TestADetailWithATargetIsAZone(t *testing.T) {
	_, zones := regionHeader([]string{"results"}, 0, false, "⌘⇧C copy", zoneCopyResult, 60)
	var found bool
	for _, z := range zones {
		if z.target == zoneCopyResult && z.to > z.from {
			found = true
		}
	}
	if !found {
		t.Errorf("zones = %+v, want one over the detail", zones)
	}
}
```

`internal/ui/app.go`:
- `buildWidgets()`에서 `a.resultTabs = newTabbed().watch(a.resultDetail)` 뒤에 `a.resultTabs.detailTarget = zoneCopyResult`를 넣는다.
- `resultDetail()`을 다음으로 바꾼다. 결과가 있을 때만 `copy`를 보이고, 없을 때는 지금의 힌트를 보인다.

```go
func (a *App) resultDetail() string {
	if a.buf.ColumnCount() > 0 && a.running == nil {
		return a.keyLabel(keymap.ActionCopyResult) + " copy"
	}
	return resultHint(resultState{
		columns: a.buf.ColumnCount(),
		running: a.running != nil,
		wrote:   a.status.written != nil,
	})
}
```

`internal/ui/mouse.go`의 `mouseAction()`에 추가한다.

```go
	case zoneCopyResult:
		return a.dispatch(keymap.ActionCopyResult)
```

- [ ] **Step 10: 통합 테스트를 쓴다**

`internal/ui/copyresult_integration_test.go`:

```go
//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
)

func TestTheCopyKeyOffersAFormatAndCopiesTheWholeResult(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.typeSQL("SELECT 1 AS a, 'x|y' AS b UNION ALL SELECT 2, 'z'")
	h.do(keymap.ActionRun)
	h.waitFor("two rows", func(a *App) bool { return a.status.phase == phaseDone && a.buf.RowCount() == 2 })

	h.do(keymap.ActionCopyResult)
	h.waitFor("the format chooser", func(a *App) bool { return a.pages.HasPage(pageCopyFormat) })
	h.press(tcell.KeyEnter) // Markdown is first

	h.waitFor("the clipboard", func(a *App) bool { return strings.HasPrefix(a.clipboard, "| a | b |") })
	if !h.inspect(func(a *App) bool { return strings.Contains(a.clipboard, `x\|y`) }) {
		t.Error("the pipe inside a value was not escaped")
	}
	h.waitFor("the notice", func(a *App) bool { return strings.Contains(a.status.message, "2 rows copied as Markdown") })
}

func TestClickingCopyOnTheResultHeaderOpensTheChooser(t *testing.T) {
	h := newHarness(t, config.EnvDev)

	h.typeSQL("SELECT 1")
	h.do(keymap.ActionRun)
	h.waitFor("a result", func(a *App) bool { return a.status.phase == phaseDone })

	h.clickZone(zoneCopyResult, -1)
	h.waitFor("the format chooser", func(a *App) bool { return a.pages.HasPage(pageCopyFormat) })
}

func TestCopyingWithNoResultSaysSo(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.do(keymap.ActionCopyResult)
	h.waitFor("the notice", func(a *App) bool { return a.status.message == "no result to copy" })
}
```

`tcell` import를 추가한다(`github.com/gdamore/tcell/v2`).

- [ ] **Step 11: 통합 테스트를 돌린다**

```sh
go test -tags integration ./internal/ui/ -run 'CopyKey|ClickingCopy|CopyingWithNoResult' -v
```

기대: 3개 PASS. `resultHint`의 기존 테스트(`resulthint_test.go`)가 `resultState`의 필드 변경으로 깨지면 필드명에 맞춰 고친다.

- [ ] **Step 12: 전체 검증과 커밋**

```sh
go build ./... && go vet -tags integration ./... && go test ./... && gofmt -l .
git add -A
git commit -m "Copy the whole result as Markdown or JSON

⌘⇧C, F3, or the copy label on the result header. The text comes from
the buffer in the grid's order, and the notice states the size because
OSC 52 clips silently past a terminal-specific ceiling."
```

---

### Task 3: 테이블 미리보기 — `LIMIT 100`

**Files:**
- Create: `internal/ui/preview.go`, `internal/ui/preview_test.go`, `internal/ui/preview_integration_test.go`
- Modify: `internal/ui/tables.go`, `internal/ui/mouse.go`, `internal/keymap/action.go`

**Interfaces:**
- Consumes: `sqlparse.QuoteIdentifier`, `sqlparse.Parse`, `(a *App) runStatement`, `nodeRef`, `a.tree.GetCurrentNode()`, `a.listedTables`, `a.currentSchema()`.
- Produces: `previewSQL(schema, table string) string`, `(a *App) previewTable(schema, table string)`. 트리 더블클릭과 테이블 탭의 `↩`/클릭이 미리보기다. 트리의 `↩`와 단일 클릭은 지금처럼 펼친다.

스펙과 다른 점 하나: 스펙은 트리의 `↩`도 미리보기라고 했지만, tview의 `TreeView`는 키보드로 노드를 펼치는 다른 키가 없다. `↩`를 미리보기로 바꾸면 키보드만 쓰는 사람이 컬럼을 볼 방법이 없어진다. 그래서 트리에서는 더블클릭만 미리보기이고, 키보드 미리보기는 테이블 탭의 `↩`다.

- [ ] **Step 1: 단위 테스트를 쓴다**

`internal/ui/preview_test.go`:

```go
package ui

import "testing"

func TestPreviewSQLQuotesBothNamesAndLimitsTo100(t *testing.T) {
	got := previewSQL("app db", "order-items")
	want := "SELECT * FROM `app db`.`order-items` LIMIT 100"
	if got != want {
		t.Errorf("previewSQL() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: 실패를 확인한다**

```sh
go test ./internal/ui/ -run TestPreviewSQL
```

기대: `undefined: previewSQL`.

- [ ] **Step 3: `preview.go`를 쓴다**

```go
package ui

import (
	"fmt"

	"github.com/Ahngbeom/datavase/internal/sqlparse"
)

// previewLimit is how many rows a click on a table shows. A hundred fits a
// screen or two, which is what "let me see what is in here" asks for.
const previewLimit = 100

func previewSQL(schema, table string) string {
	return fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d",
		sqlparse.QuoteIdentifier(schema), sqlparse.QuoteIdentifier(table), previewLimit)
}

// previewTable runs the preview without touching the editor.
//
// The editor holds the user's own text, and a click that replaced it would
// lose work to a gesture nobody reads as destructive. The SQL that ran is
// shown in the status bar instead.
func (a *App) previewTable(schema, table string) {
	if a.running != nil {
		a.notice("a statement is already running; ^C cancels it")
		return
	}
	sql := previewSQL(schema, table)
	a.runStatement(sqlparse.Parse(sql))
	a.notice(sql)
}
```

- [ ] **Step 4: 통과를 확인한다**

```sh
go test ./internal/ui/ -run TestPreviewSQL -v
```

- [ ] **Step 5: 테이블 탭과 트리에 잇는다**

`internal/ui/tables.go`의 `renderTables()`에서 Part 1이 남긴 `a.notice("table preview is not built yet")`를 `a.previewTable(schema, table.Name)`로 바꾼다.

`internal/ui/mouse.go`의 `mouseLeftDoubleClick()`에서 `gridVisible` 검사 앞에 추가한다.

```go
	// The first click of the pair already made the node current and expanded
	// it (tview's TreeView selects on a click); the second is the preview.
	if a.sidebarVisible && a.schemaTabs.current() == tabTree && a.tree.InRect(x, y) {
		if ref, ok := a.tree.GetCurrentNode().GetReference().(*nodeRef); ok && ref.kind == nodeTable {
			a.previewTable(ref.schema, ref.table)
			return nil, action
		}
		return ev, action
	}
```

`internal/keymap/action.go`의 `descriptions`에서 `ActionInspect`는 그대로 두고, 도움말에 미리보기를 알리기 위해 `dialog.go`의 `helpText()` 끝, `Enter in the schema tree expands it, or pastes a column name.` 줄을 다음으로 바꾼다.

```go
	b.WriteString("\n  Enter in the schema tree expands it, or pastes a column name.\n" +
		"  Double-click a table there, or Enter on one in the tables tab, to see its first 100 rows.\n")
```

- [ ] **Step 6: 통합 테스트를 쓴다**

`internal/ui/preview_integration_test.go`:

```go
//go:build integration

package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/catalog"
	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/keymap"
	"github.com/gdamore/tcell/v2"
)

// previewFixture makes a table with three rows and lists it in the tables
// tab, and leaves the editor holding text a preview must not disturb.
func previewFixture(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, config.EnvDev)

	h.typeSQL("CREATE TABLE IF NOT EXISTS preview_t (id INT); TRUNCATE preview_t; INSERT INTO preview_t VALUES (1),(2),(3)")
	h.do(keymap.ActionRunAll)
	h.waitFor("the fixture", func(a *App) bool { return a.batch == nil && a.running == nil })

	schema := h.app.conn.DataSource().Database
	h.seedCache(catalog.Snapshot{
		Schemas: []string{schema},
		Tables:  map[string][]catalog.Table{schema: {{Name: "preview_t"}}},
	})
	h.typeSQL("-- my draft")
	return h
}

func TestEnterOnATableInTheTablesTabShowsItsRows(t *testing.T) {
	h := previewFixture(t)

	h.showSidebar()
	h.app.app.QueueUpdateDraw(func() {
		h.app.schemaTabs.show(tabTables)
		h.app.renderTables()
		h.app.app.SetFocus(h.app.tableList)
	})
	h.waitFor("the table listed", func(a *App) bool { return len(a.listedTables) == 1 })
	h.press(tcell.KeyEnter)

	h.waitFor("three rows", func(a *App) bool { return a.status.phase == phaseDone && a.buf.RowCount() == 3 })
	if got := h.editorText(); got != "-- my draft" {
		t.Errorf("editor = %q; a preview must not replace what was being written", got)
	}
	h.waitFor("the SQL in the status bar", func(a *App) bool {
		return strings.Contains(a.status.message, "LIMIT 100")
	})
}

func TestDoubleClickingATableInTheTreeShowsItsRows(t *testing.T) {
	h := previewFixture(t)

	h.showSidebar()
	h.waitForBackgroundRefresh(h.app.conn.DataSource().Name)
	// Expand the schema so its tables are nodes.
	x, y := h.treeNodePosition(1)
	h.click(x, y)
	h.waitFor("the table node", func(a *App) bool {
		for _, n := range a.tree.GetRoot().GetChildren() {
			for _, c := range n.GetChildren() {
				if ref, ok := c.GetReference().(*nodeRef); ok && ref.table == "preview_t" {
					return true
				}
			}
		}
		return false
	})

	// The table node is the first child of the first schema node.
	tx, ty := h.treeNodePosition(2)
	h.doubleClick(tx, ty)
	h.waitFor("three rows", func(a *App) bool { return a.status.phase == phaseDone && a.buf.RowCount() == 3 })
}

func TestAPreviewWhileAStatementRunsIsRefused(t *testing.T) {
	h := previewFixture(t)
	h.typeSQL("SELECT SLEEP(5)")
	h.do(keymap.ActionRun)
	h.waitFor("running", func(a *App) bool { return a.running != nil })

	h.app.app.QueueUpdateDraw(func() { h.app.previewTable("x", "y") })
	h.waitFor("the refusal", func(a *App) bool {
		return strings.HasPrefix(a.status.message, "a statement is already running")
	})
	h.do(keymap.ActionCancel)
}
```

`treeNodePosition(offset)`의 의미(루트 아래 몇 번째 줄인가)는 하네스 453행의 정의를 읽고, 스키마 노드가 여러 개면(`information_schema` 등) `datavase_test`가 몇 번째인지에 맞춰 offset을 고른다. 더 견고하게 하려면 노드를 찾아 `a.tree.SetCurrentNode(node)`를 한 뒤 해당 줄을 더블클릭한다.

- [ ] **Step 7: 통합 테스트를 돌린다**

```sh
go test -tags integration ./internal/ui/ -run 'ShowsItsRows|PreviewWhile' -v
```

기대: 3개 PASS.

- [ ] **Step 8: 전체 검증과 커밋**

```sh
go build ./... && go vet -tags integration ./... && go test ./... && gofmt -l .
git add -A
git commit -m "Preview a table's first hundred rows from the tree or the tables tab

Double-click in the tree, Enter or a click in the tables tab. The editor
is left alone: what was being typed is the user's, and the SQL that ran
is shown in the status bar instead."
```

---

### Task 4: `config.Write`, `config.Save`, `config.Empty` — 설정 파일 쓰기

**Files:**
- Create: `internal/config/save.go`, `internal/config/save_test.go`
- Modify: `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces:**
- Produces: `func Write(w io.Writer, c *Config) error`, `func Save(path string, c *Config) error`(임시 파일 + rename, 0600, 저장 전 `validate`), `func Empty() *Config`(데이터소스 없는 기본값). 데이터소스가 0개인 설정이 유효해진다. `DataSource.Database`, `TLSCA`, `Tunnel`, `Tunnel.Identity`, `Defaults.Mouse`에 `omitempty`.

- [ ] **Step 1: 실패하는 테스트를 쓴다**

`internal/config/save_test.go`:

```go
package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sample() *Config {
	c := &Config{DataSources: []DataSource{
		{Name: "local", Host: "127.0.0.1", User: "root", Database: "app"},
		{Name: "bastion", Host: "db.internal", User: "ro", TLS: TLSRequired,
			Tunnel: &Tunnel{Host: "jump.example.com", User: "me"}},
	}}
	c.applyDefaults()
	return c
}

func TestWriteThenParseRoundTrips(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, sample()); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	back, err := Parse(&buf)
	if err != nil {
		t.Fatalf("Parse(Write()) error = %v\n%s", err, buf.String())
	}
	if !reflect.DeepEqual(back.DataSources, sample().DataSources) {
		t.Errorf("round trip changed the datasources:\n got %+v\nwant %+v", back.DataSources, sample().DataSources)
	}
}

func TestWriteLeavesOutWhatWasNeverSet(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, sample()); err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"env:", "tls_ca:", "identity:", "keymap:", "mouse:"} {
		if strings.Contains(buf.String(), absent) {
			t.Errorf("Write() contains %q for a field that was never set:\n%s", absent, buf.String())
		}
	}
	if strings.Count(buf.String(), "tunnel:") != 1 {
		t.Errorf("Write() must write the tunnel once, for the datasource that has one:\n%s", buf.String())
	}
}

func TestSaveWritesAnOwnerOnlyFileTheLoaderReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	if err := Save(path, sample()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 600: the file names hosts and accounts", info.Mode().Perm())
	}
	if _, err := Load(path); err != nil {
		t.Errorf("Load(Save()) error = %v", err)
	}
}

func TestSaveRefusesAnInvalidConfigAndKeepsTheOldFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(path, sample()); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)

	bad := sample()
	bad.DataSources[1].Name = "local" // duplicate
	if err := Save(path, bad); err == nil {
		t.Fatal("Save() = nil, want an error for a duplicate name")
	}

	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Error("a refused save changed the file")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("directory has %d entries, want only config.yaml; a temp file was left behind", len(entries))
	}
}

func TestAConfigWithNoDatasourcesIsValid(t *testing.T) {
	cfg, err := Parse(strings.NewReader("datasources: []\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v; an empty list is where a first run starts", err)
	}
	if len(cfg.DataSources) != 0 || cfg.Defaults.AutoLimit != DefaultAutoLimit {
		t.Errorf("cfg = %+v", cfg)
	}
	if got := Empty(); got.Defaults.FetchChunk != DefaultFetchChunk {
		t.Errorf("Empty() lacks defaults: %+v", got.Defaults)
	}
}
```

- [ ] **Step 2: 실패를 확인한다**

```sh
go test ./internal/config/ -run 'TestWrite|TestSave|TestAConfigWithNoDatasources'
```

기대: `undefined: Write`, `Save`, `Empty`.

- [ ] **Step 3: `config.go`를 고친다**

- `DataSource`의 태그: `Database string \`yaml:"database,omitempty"\``, `Tunnel *Tunnel \`yaml:"tunnel,omitempty"\``, `TLSCA string \`yaml:"tls_ca,omitempty"\``. (`Env`는 Part 1에서 이미 `omitempty`.)
- `Tunnel.Identity`: `yaml:"identity,omitempty"`.
- `Defaults.Mouse`: `yaml:"mouse,omitempty"`.
- `validate()`에서 `if len(c.DataSources) == 0 { return errors.New("no datasources defined") }`를 지운다. `config_test.go`의 `TestParseRejectsInvalidConfig` 테이블에서 `"no datasources"` 행을 지운다. `errors` import가 안 쓰이면 지운다.

- [ ] **Step 4: `save.go`를 쓴다**

```go
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Empty is the configuration of a machine that has none: no datasources,
// the defaults in force.
func Empty() *Config {
	c := &Config{}
	c.applyDefaults()
	return c
}

// Write serialises c as YAML.
func Write(w io.Writer, c *Config) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("encoding the configuration: %w", err)
	}
	return enc.Close()
}

// Save replaces the file at path with c.
//
// The write goes to a temporary file beside it and is renamed into place:
// a crash or a full disk midway leaves the previous file intact rather than
// a truncated one that the next run cannot parse. Owner-only, because the
// file names hosts and accounts inside someone's network.
func Save(path string, c *Config) error {
	if err := c.validate(); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	defer os.Remove(tmp.Name())

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := Write(tmp, c); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	return nil
}
```

`ignored` 비공개 필드는 yaml이 무시하므로 쓰이지 않는다. `Keymap`은 Parse 뒤 nil이라 `omitempty`로 빠진다.

- [ ] **Step 5: 통과를 확인하고 변형해 본다**

```sh
go test ./internal/config/ -v
```

기대: 전부 PASS. `Save`의 `c.validate()` 호출을 잠시 빼고 `TestSaveRefusesAnInvalidConfigAndKeepsTheOldFile`이 FAIL하는지 본 뒤 되돌린다.

- [ ] **Step 6: 커밋**

```sh
git add internal/config
git commit -m "Write the configuration back: Save is atomic and owner-only

The datasource dialog is the file's writer now, so config learns to
serialise itself. A config with no datasources is valid because that is
what a first run holds."
```

---

### Task 5: 데이터소스 폼과 목록 컴포넌트

**Files:**
- Create: `internal/ui/dsform.go`, `internal/ui/dsform_test.go`, `internal/ui/dspicker.go`
- Modify: (없음 — 아직 어디에도 배선하지 않는다; 다음 두 태스크가 쓴다)

**Interfaces:**
- Consumes: `config.Config`, `config.DataSource`, `config.Tunnel`, `config.TLSMode` 상수, `secret.Store`.
- Produces:
  - `dsform.go`: `type dsFields struct{...}`, `func (f dsFields) toDataSource() (config.DataSource, error)`, `func validateDataSource(cfg *config.Config, editing string, ds config.DataSource) error`.
  - `dspicker.go`: `type pickerDeps struct`, `func newDSPicker(app *tview.Application, deps pickerDeps) *dsPicker`, `func (p *dsPicker) Primitive() tview.Primitive`, `func (p *dsPicker) setStatus(text string)`, `func (p *dsPicker) setBusy(busy bool)`.

- [ ] **Step 1: 폼 검증의 실패하는 테스트를 쓴다**

`internal/ui/dsform_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"github.com/Ahngbeom/datavase/internal/config"
)

func fieldsFor(name string) dsFields {
	return dsFields{name: name, host: "db.example.com", port: "3306", user: "app", tls: string(config.TLSPreferred)}
}

func TestFieldsBecomeADataSource(t *testing.T) {
	f := fieldsFor("prod")
	f.database = "shop"
	f.tunnelHost, f.tunnelPort, f.tunnelUser = "jump", "2222", "me"

	ds, err := f.toDataSource()
	if err != nil {
		t.Fatal(err)
	}
	if ds.Port != 3306 || ds.Database != "shop" || ds.Tunnel == nil || ds.Tunnel.Port != 2222 || ds.Tunnel.User != "me" {
		t.Errorf("toDataSource() = %+v", ds)
	}
}

func TestAnEmptyPortMeansTheDefault(t *testing.T) {
	f := fieldsFor("x")
	f.port, f.tunnelHost, f.tunnelPort = "", "jump", ""
	f.tunnelUser = "me"
	ds, err := f.toDataSource()
	if err != nil {
		t.Fatal(err)
	}
	if ds.Port != config.DefaultPort || ds.Tunnel.Port != config.DefaultTunnelPort {
		t.Errorf("ports = %d / %d, want the defaults", ds.Port, ds.Tunnel.Port)
	}
}

func TestANonNumericPortIsRefused(t *testing.T) {
	f := fieldsFor("x")
	f.port = "abc"
	if _, err := f.toDataSource(); err == nil || !strings.Contains(err.Error(), "port") {
		t.Errorf("toDataSource() error = %v, want one naming the port", err)
	}
}

func TestATunnelWithoutAHostIsNoTunnel(t *testing.T) {
	f := fieldsFor("x")
	f.tunnelUser = "me" // a user typed into the tunnel section, no host
	if _, err := f.toDataSource(); err == nil || !strings.Contains(err.Error(), "tunnel host") {
		t.Errorf("toDataSource() error = %v, want a refusal naming the tunnel host", err)
	}
	f.tunnelUser = ""
	ds, err := f.toDataSource()
	if err != nil || ds.Tunnel != nil {
		t.Errorf("an empty tunnel section must mean no tunnel; got %+v, %v", ds.Tunnel, err)
	}
}

func TestValidateRefusesADuplicateNameExceptTheOneBeingEdited(t *testing.T) {
	cfg := &config.Config{DataSources: []config.DataSource{{Name: "a", Host: "h", User: "u"}, {Name: "b", Host: "h", User: "u"}}}

	dup := config.DataSource{Name: "a", Host: "h", User: "u"}
	if err := validateDataSource(cfg, "", dup); err == nil {
		t.Error("adding a second \"a\" was accepted")
	}
	if err := validateDataSource(cfg, "a", dup); err != nil {
		t.Errorf("editing \"a\" under its own name was refused: %v", err)
	}
	renamed := config.DataSource{Name: "b", Host: "h", User: "u"}
	if err := validateDataSource(cfg, "a", renamed); err == nil {
		t.Error("renaming \"a\" to the existing \"b\" was accepted")
	}
}

func TestValidateRequiresNameHostAndUser(t *testing.T) {
	cfg := &config.Config{}
	for _, ds := range []config.DataSource{
		{Host: "h", User: "u"},
		{Name: "n", User: "u"},
		{Name: "n", Host: "h"},
	} {
		if err := validateDataSource(cfg, "", ds); err == nil {
			t.Errorf("validateDataSource(%+v) = nil, want an error", ds)
		}
	}
}
```

- [ ] **Step 2: 실패를 확인한다**

```sh
go test ./internal/ui/ -run 'TestFields|TestAnEmptyPort|TestANonNumeric|TestATunnel|TestValidate'
```

기대: `undefined: dsFields`.

- [ ] **Step 3: `dsform.go`를 쓴다**

```go
package ui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Ahngbeom/datavase/internal/config"
)

// dsFields is what the form holds, as typed. Turning it into a DataSource
// is separate from the widgets so the rules can be tested without a screen.
type dsFields struct {
	name, host, port, user, database string
	tls, tlsCA                        string
	tunnelHost, tunnelPort, tunnelUser, tunnelIdentity string
}

func fieldsFromDataSource(ds *config.DataSource) dsFields {
	f := dsFields{
		name: ds.Name, host: ds.Host, port: strconv.Itoa(ds.Port), user: ds.User,
		database: ds.Database, tls: string(ds.TLS), tlsCA: ds.TLSCA,
	}
	if ds.Tunnel != nil {
		f.tunnelHost, f.tunnelUser, f.tunnelIdentity = ds.Tunnel.Host, ds.Tunnel.User, ds.Tunnel.Identity
		f.tunnelPort = strconv.Itoa(ds.Tunnel.Port)
	}
	return f
}

// toDataSource parses the typed values. An empty port is the default; an
// empty tunnel host with nothing else typed in the tunnel section means no
// tunnel, and with something typed means a mistake worth naming.
func (f dsFields) toDataSource() (config.DataSource, error) {
	port, err := parsePort(f.port, config.DefaultPort, "port")
	if err != nil {
		return config.DataSource{}, err
	}

	ds := config.DataSource{
		Name: strings.TrimSpace(f.name), Host: strings.TrimSpace(f.host), Port: port,
		User: strings.TrimSpace(f.user), Database: strings.TrimSpace(f.database),
		TLS: config.TLSMode(f.tls), TLSCA: strings.TrimSpace(f.tlsCA),
	}

	tunnelTyped := strings.TrimSpace(f.tunnelPort) != "" || strings.TrimSpace(f.tunnelUser) != "" ||
		strings.TrimSpace(f.tunnelIdentity) != ""
	if host := strings.TrimSpace(f.tunnelHost); host != "" {
		tport, err := parsePort(f.tunnelPort, config.DefaultTunnelPort, "tunnel port")
		if err != nil {
			return config.DataSource{}, err
		}
		ds.Tunnel = &config.Tunnel{Host: host, Port: tport,
			User: strings.TrimSpace(f.tunnelUser), Identity: strings.TrimSpace(f.tunnelIdentity)}
	} else if tunnelTyped {
		return config.DataSource{}, errors.New("tunnel host is required when the tunnel section is filled in")
	}
	return ds, nil
}

func parsePort(s string, def int, what string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 65535 {
		return 0, fmt.Errorf("%s must be a number between 1 and 65535", what)
	}
	return n, nil
}

// validateDataSource is what Save checks before the file is touched: the
// fields the connection cannot do without, and a name that is not already
// someone else's. editing is the name the entry had, so renaming to itself
// is not a duplicate.
func validateDataSource(cfg *config.Config, editing string, ds config.DataSource) error {
	switch {
	case ds.Name == "":
		return errors.New("name is required")
	case ds.Host == "":
		return errors.New("host is required")
	case ds.User == "":
		return errors.New("user is required")
	}
	for i := range cfg.DataSources {
		other := cfg.DataSources[i].Name
		if other == ds.Name && other != editing {
			return fmt.Errorf("a datasource named %q already exists", ds.Name)
		}
	}
	if ds.Tunnel != nil && ds.Tunnel.User == "" {
		return errors.New("tunnel user is required")
	}
	return nil
}
```

- [ ] **Step 4: 통과를 확인한다**

```sh
go test ./internal/ui/ -run 'TestFields|TestAnEmptyPort|TestANonNumeric|TestATunnel|TestValidate' -v
```

기대: 전부 PASS.

- [ ] **Step 5: `dspicker.go`를 쓴다**

```go
package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/secret"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// probeTimeout bounds the Test button, so a bastion that is not answering
// gives the form back.
const probeTimeout = 15 * time.Second

// pickerDeps is what the datasource list and form need from whoever hosts
// them — the launcher before a session exists, or the App during one.
type pickerDeps struct {
	cfg *config.Config
	// current is the name of the connected datasource, or empty.
	current string
	// save persists cfg. It is the host's because only the host knows the
	// path, and the App wants to say something after it succeeds.
	save func() error
	// secrets holds passwords. Nil means none can be stored, which the form
	// says rather than pretending.
	secrets secret.Store
	// probe answers "can this be reached", for the Test button.
	probe func(ctx context.Context, ds *config.DataSource, password string) (string, error)
	// connect is what Enter on an entry does. The host owns the session.
	connect func(ds *config.DataSource)
	// close is Escape on the list.
	close func()
}

// dsPicker is the list of datasources and the form behind it.
type dsPicker struct {
	app  *tview.Application
	deps pickerDeps

	pages  *tview.Pages
	list   *tview.List
	status *tview.TextView
	busy   bool
}

const (
	pickerList = "list"
	pickerForm = "form"
)

func newDSPicker(app *tview.Application, deps pickerDeps) *dsPicker {
	p := &dsPicker{app: app, deps: deps, pages: tview.NewPages()}

	p.list = tview.NewList().ShowSecondaryText(true).SetHighlightFullLine(true)
	p.list.SetBorder(true).SetTitle(" datasources — Enter connect · a add · e edit · d delete · Esc close ")
	p.list.SetInputCapture(p.listKey)
	p.list.SetDoneFunc(func() {
		if p.deps.close != nil {
			p.deps.close()
		}
	})

	p.status = tview.NewTextView().SetDynamicColors(true)

	body := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(p.list, 0, 1, true).
		AddItem(p.status, 1, 0, false)
	p.pages.AddPage(pickerList, body, true, true)

	p.renderList()
	return p
}

func (p *dsPicker) Primitive() tview.Primitive { return p.pages }

func (p *dsPicker) setStatus(text string) { p.status.SetText(text) }

// setBusy blocks a second connect while one is in flight.
func (p *dsPicker) setBusy(busy bool) { p.busy = busy }

func (p *dsPicker) renderList() {
	p.list.Clear()
	if len(p.deps.cfg.DataSources) == 0 {
		p.list.AddItem("no datasources yet", "press a to add one", 0, nil)
		return
	}
	for i := range p.deps.cfg.DataSources {
		ds := &p.deps.cfg.DataSources[i]
		detail := fmt.Sprintf("%s@%s:%d", ds.User, ds.Host, ds.Port)
		if ds.Tunnel != nil {
			detail += " via " + ds.Tunnel.Host
		}
		if ds.Name == p.deps.current {
			detail += " · connected"
		}
		p.list.AddItem(ds.Name, detail, 0, func() { p.connectTo(ds) })
	}
}

func (p *dsPicker) selected() *config.DataSource {
	i := p.list.GetCurrentItem()
	if i < 0 || i >= len(p.deps.cfg.DataSources) {
		return nil
	}
	return &p.deps.cfg.DataSources[i]
}

func (p *dsPicker) listKey(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Key() != tcell.KeyRune {
		return ev
	}
	switch ev.Rune() {
	case 'a':
		p.showForm(nil)
	case 'e':
		if ds := p.selected(); ds != nil {
			p.showForm(ds)
		}
	case 'd':
		if ds := p.selected(); ds != nil {
			p.confirmDelete(ds)
		}
	default:
		return ev
	}
	return nil
}

func (p *dsPicker) connectTo(ds *config.DataSource) {
	if p.busy {
		p.setStatus("still connecting…")
		return
	}
	if p.deps.connect != nil {
		p.deps.connect(ds)
	}
}

// confirmDelete asks, then removes the entry and its stored password.
func (p *dsPicker) confirmDelete(ds *config.DataSource) {
	name := ds.Name
	modal := newModal().
		SetText(fmt.Sprintf("Delete %s?\n\nIts stored password goes with it.", name)).
		AddButtons([]string{"Cancel", "Delete"}).
		SetDoneFunc(func(_ int, label string) {
			p.pages.RemovePage("confirm")
			p.app.SetFocus(p.list)
			if label != "Delete" {
				return
			}
			p.remove(name)
		})
	p.pages.AddPage("confirm", modal, true, true)
	p.app.SetFocus(modal)
}

func (p *dsPicker) remove(name string) {
	kept := p.deps.cfg.DataSources[:0]
	for _, ds := range p.deps.cfg.DataSources {
		if ds.Name != name {
			kept = append(kept, ds)
		}
	}
	p.deps.cfg.DataSources = kept
	if err := p.deps.save(); err != nil {
		p.setStatus(tag(colourDanger, "saving: "+err.Error()))
		return
	}
	if p.deps.secrets != nil {
		// A missing entry is not a failure: there may never have been one.
		_ = p.deps.secrets.Delete(name)
	}
	p.renderList()
	p.setStatus("deleted " + name)
}

// showForm opens the add form (ds nil) or the edit form.
func (p *dsPicker) showForm(ds *config.DataSource) {
	editing := ""
	fields := dsFields{port: "", tls: string(config.TLSPreferred)}
	if ds != nil {
		editing = ds.Name
		fields = fieldsFromDataSource(ds)
	}
	password := ""

	tlsModes := []string{string(config.TLSPreferred), string(config.TLSRequired),
		string(config.TLSVerifyCA), string(config.TLSVerifyIdentity), string(config.TLSDisabled)}
	tlsIndex := 0
	for i, m := range tlsModes {
		if m == fields.tls {
			tlsIndex = i
		}
	}

	form := tview.NewForm()
	form.AddInputField("name", fields.name, 30, nil, func(s string) { fields.name = s })
	form.AddInputField("host", fields.host, 30, nil, func(s string) { fields.host = s })
	form.AddInputField("port", fields.port, 6, nil, func(s string) { fields.port = s })
	form.AddInputField("user", fields.user, 30, nil, func(s string) { fields.user = s })
	passwordLabel := "password"
	if ds != nil {
		passwordLabel = "password (blank keeps the stored one)"
	}
	form.AddPasswordField(passwordLabel, "", 30, '•', func(s string) { password = s })
	form.AddInputField("database", fields.database, 30, nil, func(s string) { fields.database = s })
	form.AddDropDown("tls", tlsModes, tlsIndex, func(option string, _ int) { fields.tls = option })
	form.AddInputField("tls_ca (PEM file, verify modes only)", fields.tlsCA, 30, nil, func(s string) { fields.tlsCA = s })
	form.AddInputField("tunnel host (blank: no tunnel)", fields.tunnelHost, 30, nil, func(s string) { fields.tunnelHost = s })
	form.AddInputField("tunnel port", fields.tunnelPort, 6, nil, func(s string) { fields.tunnelPort = s })
	form.AddInputField("tunnel user", fields.tunnelUser, 30, nil, func(s string) { fields.tunnelUser = s })
	form.AddInputField("tunnel identity (key file)", fields.tunnelIdentity, 30, nil, func(s string) { fields.tunnelIdentity = s })

	message := tview.NewTextView().SetDynamicColors(true)
	say := func(text string) { message.SetText(text) }

	closeForm := func() {
		p.pages.RemovePage(pickerForm)
		p.app.SetFocus(p.list)
	}

	form.AddButton("Test", func() {
		candidate, err := fields.toDataSource()
		if err != nil {
			say(tag(colourDanger, err.Error()))
			return
		}
		if err := validateDataSource(p.deps.cfg, editing, candidate); err != nil {
			say(tag(colourDanger, err.Error()))
			return
		}
		pw := password
		if pw == "" && p.deps.secrets != nil && editing != "" {
			pw, _ = p.deps.secrets.Get(editing)
		}
		say("testing…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
			defer cancel()
			version, err := p.deps.probe(ctx, &candidate, pw)
			p.app.QueueUpdateDraw(func() {
				if err != nil {
					say(tag(colourDanger, err.Error()))
					return
				}
				say("reachable · " + version)
			})
		}()
	})

	form.AddButton("Save", func() {
		candidate, err := fields.toDataSource()
		if err != nil {
			say(tag(colourDanger, err.Error()))
			return
		}
		if err := validateDataSource(p.deps.cfg, editing, candidate); err != nil {
			say(tag(colourDanger, err.Error()))
			return
		}
		if password != "" && p.deps.secrets == nil {
			say(tag(colourDanger, "no keychain here; set DATAVASE_PASSWORD_<NAME> instead"))
			return
		}

		if editing == "" {
			p.deps.cfg.DataSources = append(p.deps.cfg.DataSources, candidate)
		} else {
			for i := range p.deps.cfg.DataSources {
				if p.deps.cfg.DataSources[i].Name == editing {
					p.deps.cfg.DataSources[i] = candidate
				}
			}
		}
		if err := p.deps.save(); err != nil {
			say(tag(colourDanger, "saving: "+err.Error()))
			return
		}
		if password != "" {
			if err := p.deps.secrets.Set(candidate.Name, password); err != nil {
				say(tag(colourDanger, "the file is saved, but the keychain refused the password: "+err.Error()))
				return
			}
		}
		if editing != "" && editing != candidate.Name && p.deps.secrets != nil {
			// A rename moves the password with the entry.
			if old, err := p.deps.secrets.Get(editing); err == nil && password == "" {
				_ = p.deps.secrets.Set(candidate.Name, old)
			}
			_ = p.deps.secrets.Delete(editing)
		}
		p.renderList()
		p.setStatus("saved " + candidate.Name)
		closeForm()
	})
	form.AddButton("Cancel", closeForm)
	form.SetCancelFunc(closeForm)

	title := " add a datasource "
	if ds != nil {
		title = fmt.Sprintf(" edit %s ", ds.Name)
	}
	form.SetBorder(true).SetTitle(title)

	body := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, 0, 1, true).
		AddItem(message, 1, 0, false)
	p.pages.AddPage(pickerForm, body, true, true)
	p.app.SetFocus(form)
}
```


- [ ] **Step 6: 빌드와 lint**

```sh
go build ./... && go vet ./... && gofmt -l .
```

기대: 통과. `dspicker.go`는 아직 아무도 부르지 않지만 `go vet`은 미사용 타입을 문제 삼지 않는다.

- [ ] **Step 7: 커밋**

```sh
git add internal/ui/dsform.go internal/ui/dsform_test.go internal/ui/dspicker.go
git commit -m "A datasource list and form, ready to be hosted

The rules — required fields, a duplicate name, a tunnel section with no
host — live apart from the widgets so they can be tested without a
screen. Passwords go to the keychain and never into the form's output."
```

---

### Task 6: 런처 — 설정이 없거나 이름 없이 실행하면 목록이 먼저 뜬다

**Files:**
- Create: `internal/ui/launch.go`, `internal/ui/launch_integration_test.go`
- Modify: `cmd/dv/main.go`, `internal/cli/cli.go`, `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `newDSPicker`, `config.Save`, `config.Empty`, `session.Session`.
- Produces: `ui.LaunchDeps`, `func Launch(deps LaunchDeps) (*session.Session, error)` — 사용자가 연결하면 세션을, 닫으면 `nil, nil`. `cli.App.Launch func() error` 필드. `cli.open("")`은 데이터소스가 정확히 하나면 지금처럼 바로 열고, 아니면 `Launch`를 부른다. `dv`는 설정 파일이 없으면 `config.Empty()`로 시작한다.

- [ ] **Step 1: `launch.go`를 쓴다**

```go
package ui

import (
	"context"
	"fmt"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/secret"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// LaunchDeps is what the datasource list needs before there is a session.
type LaunchDeps struct {
	Config     *config.Config
	ConfigPath string
	Secrets    secret.Store
	Probe      func(ctx context.Context, ds *config.DataSource, password string) (string, error)
	Connect    func(ctx context.Context, ds *config.DataSource) (*session.Session, error)
	// Screen replaces the terminal, for tests. Nil is the real one.
	Screen tcell.Screen
}

// Launch shows the datasource list on its own and returns the session the
// user connected to, or nil when they closed the list instead.
//
// It is its own small application rather than a state of App because App
// assumes a connection in nearly every method; a launcher that ends by
// handing over a session keeps that assumption true.
func Launch(deps LaunchDeps) (*session.Session, error) {
	app := tview.NewApplication()
	if deps.Screen != nil {
		app.SetScreen(deps.Screen)
	}

	var (
		picker *dsPicker
		got    *session.Session
		fail   error
	)
	picker = newDSPicker(app, pickerDeps{
		cfg:     deps.Config,
		save:    func() error { return config.Save(deps.ConfigPath, deps.Config) },
		secrets: deps.Secrets,
		probe:   deps.Probe,
		close:   app.Stop,
		connect: func(ds *config.DataSource) {
			picker.setBusy(true)
			picker.setStatus(fmt.Sprintf("connecting to %s…", ds.Name))
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
				defer cancel()
				sess, err := deps.Connect(ctx, ds)
				app.QueueUpdateDraw(func() {
					picker.setBusy(false)
					if err != nil {
						picker.setStatus(tag(colourDanger, err.Error()))
						return
					}
					got = sess
					app.Stop()
				})
			}()
		},
	})

	root := tview.NewPages().AddPage("picker", centred(picker.Primitive(), 80, 24), true, true)
	if err := app.SetRoot(root, true).EnableMouse(true).Run(); err != nil {
		fail = err
	}
	return got, fail
}
```

`connectTimeout`은 `datasource.go`에 이미 있다.

- [ ] **Step 2: 런처 통합 테스트를 쓴다**

`internal/ui/launch_integration_test.go`:

```go
//go:build integration

package ui

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ahngbeom/datavase/internal/config"
	"github.com/Ahngbeom/datavase/internal/secret"
	"github.com/Ahngbeom/datavase/internal/session"
	"github.com/Ahngbeom/datavase/internal/testmysql"
	"github.com/gdamore/tcell/v2"
)

func TestTheLauncherConnectsToTheChosenEntry(t *testing.T) {
	ds, password := testmysql.DataSource(t)
	cfg := config.Empty()
	cfg.DataSources = []config.DataSource{*ds}
	store := secret.NewMemory()
	_ = store.Set(ds.Name, password)

	screen := tcell.NewSimulationScreen("UTF-8")
	screen.SetSize(100, 30)

	done := make(chan struct {
		sess *session.Session
		err  error
	}, 1)
	go func() {
		sess, err := Launch(LaunchDeps{
			Config: cfg, ConfigPath: filepath.Join(t.TempDir(), "config.yaml"), Secrets: store,
			Connect: func(ctx context.Context, ds *config.DataSource) (*session.Session, error) {
				pw, _ := store.Get(ds.Name)
				return session.Open(ctx, ds, pw)
			},
			Screen: screen,
		})
		done <- struct {
			sess *session.Session
			err  error
		}{sess, err}
	}()

	time.Sleep(200 * time.Millisecond) // let the list draw
	screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)

	select {
	case r := <-done:
		if r.err != nil || r.sess == nil {
			t.Fatalf("Launch() = %v, %v; want a session", r.sess, r.err)
		}
		r.sess.Close()
	case <-time.After(20 * time.Second):
		t.Fatal("the launcher never returned")
	}
}

func TestClosingTheLauncherReturnsNoSession(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	screen.SetSize(100, 30)

	done := make(chan *session.Session, 1)
	go func() {
		sess, _ := Launch(LaunchDeps{Config: config.Empty(), ConfigPath: filepath.Join(t.TempDir(), "c.yaml"), Screen: screen})
		done <- sess
	}()
	time.Sleep(200 * time.Millisecond)
	screen.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)

	select {
	case sess := <-done:
		if sess != nil {
			t.Error("Escape on an empty list produced a session")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the launcher never returned")
	}
}

func TestAddingAnEntryInTheLauncherWritesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := config.Empty()
	screen := tcell.NewSimulationScreen("UTF-8")
	screen.SetSize(100, 40)

	done := make(chan struct{}, 1)
	go func() {
		Launch(LaunchDeps{Config: cfg, ConfigPath: path, Secrets: secret.NewMemory(), Screen: screen})
		done <- struct{}{}
	}()
	time.Sleep(200 * time.Millisecond)

	type_ := func(s string) {
		for _, r := range s {
			screen.InjectKey(tcell.KeyRune, r, tcell.ModNone)
		}
	}
	screen.InjectKey(tcell.KeyRune, 'a', tcell.ModNone) // add
	time.Sleep(100 * time.Millisecond)
	type_("local")
	screen.InjectKey(tcell.KeyTab, 0, tcell.ModNone)
	type_("127.0.0.1")
	screen.InjectKey(tcell.KeyTab, 0, tcell.ModNone) // port
	screen.InjectKey(tcell.KeyTab, 0, tcell.ModNone) // user
	type_("root")
	// Tab through password, database, tls, tls_ca, tunnel host/port/user/identity, Test → Save.
	for i := 0; i < 10; i++ {
		screen.InjectKey(tcell.KeyTab, 0, tcell.ModNone)
	}
	screen.InjectKey(tcell.KeyEnter, 0, tcell.ModNone) // Save
	time.Sleep(300 * time.Millisecond)
	screen.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the launcher never returned")
	}
	saved, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() after the form saved: %v", err)
	}
	if len(saved.DataSources) != 1 || saved.DataSources[0].Name != "local" || saved.DataSources[0].User != "root" {
		t.Errorf("saved = %+v", saved.DataSources)
	}
}
```

Tab 횟수는 `showForm`의 필드 순서에 맞춘다(name, host, port, user, password, database, tls, tls_ca, tunnel host, tunnel port, tunnel user, tunnel identity, Test, Save, Cancel). `tview.Form`에서 드롭다운은 Tab 한 번으로 지나간다. 첫 실행에서 어긋나면 `screen`의 내용을 `t.Log`로 찍어 위치를 맞춘다.

`sleep` 후 읽기는 이 파일에서만 허용한다. 런처는 `App`의 하네스(`inspect`/`waitFor`)를 쓸 수 없고, 상태를 읽는 대신 종료 채널만 기다린다.

- [ ] **Step 3: 통합 테스트를 돌린다**

```sh
go test -tags integration ./internal/ui/ -run 'Launcher|AddingAnEntry' -v
```

기대: 3개 PASS.

- [ ] **Step 4: `cli`를 배선한다**

`internal/cli/cli.go`:
- `App`에 필드를 추가한다.

```go
	// Launch shows the datasource list without a session and opens the
	// interface on whichever entry the user connects to. It is how a machine
	// with no configuration gets one.
	Launch func() error
```

- `open()`의 시작을 다음으로 바꾼다.

```go
func (a *App) open(name string) int {
	if name == "" {
		if len(a.Config.DataSources) == 1 {
			name = a.Config.DataSources[0].Name
		} else {
			if err := a.Launch(); err != nil {
				fmt.Fprintln(a.Err, err)
				return exitError
			}
			return exitOK
		}
	}
	// 이하 기존: Find, 키체인, OpenUI
```

Part 1 Task 7에서 넣은 "no datasources are configured" 분기와 "more than one datasource is configured" 분기를 지운다.

- `usage()`의 첫 줄을 `dv                    open the interface; the datasource list when there is more than one`와 `dv open <name>        open a named datasource` 두 줄로 바꾼다.

`internal/cli/cli_test.go`에 추가한다.

```go
func TestOpeningWithNoNameAndSeveralDatasourcesLaunchesTheList(t *testing.T) {
	launched := false
	app := &App{
		Config: &config.Config{DataSources: []config.DataSource{{Name: "a"}, {Name: "b"}}},
		Out: io.Discard, Err: io.Discard,
		Launch: func() error { launched = true; return nil },
	}
	if code := app.Run(nil); code != exitOK || !launched {
		t.Errorf("Run() = %d, launched = %v; want the list", code, launched)
	}
}

func TestOpeningWithNoDatasourcesLaunchesTheList(t *testing.T) {
	launched := false
	app := &App{Config: config.Empty(), Out: io.Discard, Err: io.Discard,
		Launch: func() error { launched = true; return nil }}
	if code := app.Run(nil); code != exitOK || !launched {
		t.Errorf("Run() = %d, launched = %v; a first run must open the list", code, launched)
	}
}
```

- [ ] **Step 5: `main.go`를 배선한다**

`run()`:
- 설정 파일이 없을 때 종료하던 분기를 다음으로 바꾼다.

```go
	cfg, err := config.Load(path)
	if errors.Is(err, fs.ErrNotExist) {
		cfg, err = config.Empty(), nil
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
```

- `cli.App` 리터럴에 `Launch: func() error { return launch(cfg, path) }`를 추가한다.
- `openUI` 아래에 추가한다.

```go
// launch shows the datasource list, then opens the interface on the session
// it produced.
func launch(cfg *config.Config, path string) error {
	sess, err := ui.Launch(ui.LaunchDeps{
		Config: cfg, ConfigPath: path, Secrets: secrets(), Probe: probe, Connect: connectTo,
	})
	if err != nil || sess == nil {
		return err
	}
	return openSession(sess, cfg, path)
}
```

- `openUI`를 두 조각으로 나눈다. 연결 부분과, 열린 세션으로 인터페이스를 띄우는 `openSession(sess *session.Session, cfg *config.Config, path string) error`. `openUI`는 `session.Open`한 뒤 `openSession`을 부른다. `openSession`이 catalog/history를 열고 `ui.New(sess, cfg, ui.Deps{Cache, History, Connect: connectTo, ConfigPath: path, Secrets: secrets(), Probe: probe}).Run()`을 한다. (`Deps.ConfigPath`, `Secrets`, `Probe`는 다음 태스크가 추가하므로, 이 태스크에서는 `Cache`, `History`, `Connect`만 넘기고 다음 태스크에서 세 필드를 더한다.)
- `probe()`(428–445행)에서 쓰이지 않는 `history` 블록(430–436)을 지운다.

- [ ] **Step 6: 검증과 커밋**

```sh
go build ./... && go vet -tags integration ./... && go test ./... && gofmt -l .
go build -o /tmp/dv-smoke ./cmd/dv && XDG_CONFIG_HOME=$(mktemp -d) /tmp/dv-smoke </dev/null; echo "exit $?"
```

마지막 줄은 TTY가 없어 tview가 실패하며 0이 아닌 코드로 끝나는 것이 정상이다. 목적은 설정 파일 없이 "no configuration" 대신 런처 경로로 들어가는지 보는 것이다. 실제 터미널에서 `XDG_CONFIG_HOME=$(mktemp -d) ./dv`를 한 번 손으로 열어 목록이 뜨는지 본다.

```sh
git add -A
git commit -m "Open on the datasource list when there is nothing, or more than one, to open

A machine with no configuration starts here and leaves with one written.
dv open <name> still goes straight in."
```

---

### Task 7: 앱 안에서 `⌘⇧D`로 같은 목록을 연다

**Files:**
- Modify: `internal/ui/app.go`, `internal/ui/datasource.go`, `internal/ui/datasource_integration_test.go`, `internal/keymap/action.go`, `cmd/dv/main.go`, `internal/ui/app_integration_test.go`

**Interfaces:**
- Consumes: `newDSPicker`, `(a *App) switchTo`, `(a *App) adopt`.
- Produces: `ui.Deps.ConfigPath string`, `Deps.Secrets secret.Store`, `Deps.Probe func(...)`. `showDataSources()`가 컴포넌트를 `pageDataSource`에 띄운다. `dataSourceChoices()`는 사라진다. `ActionSwitchDataSource`의 설명은 `"open the datasource list: connect, add, edit, delete"`.

- [ ] **Step 1: 통합 테스트를 고쳐 쓴다**

`internal/ui/datasource_integration_test.go`에서 검색 상자 목록을 검증하던 테스트(`dataSourceChoices`, `searchItem`을 읽는 것)를 지우고 다음을 추가한다. `switchTo`/`adopt`를 검증하는 기존 테스트는 남긴다.

```go
func TestTheDatasourceKeyOpensTheListWithTheCurrentOneMarked(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	h.do(keymap.ActionSwitchDataSource)
	h.waitFor("the list", func(a *App) bool { return a.pages.HasPage(pageDataSource) })
	if !h.waitForScreen("connected") {
		t.Errorf("the connected datasource is not marked; screen:\n%s", h.text())
	}
	h.press(tcell.KeyEscape)
	h.waitFor("the list closed", func(a *App) bool { return !a.pages.HasPage(pageDataSource) })
}

func TestAddingADatasourceInSessionSavesTheFile(t *testing.T) {
	h := newHarness(t, config.EnvDev)
	path := filepath.Join(t.TempDir(), "config.yaml")
	h.app.configPath = path

	h.do(keymap.ActionSwitchDataSource)
	h.waitFor("the list", func(a *App) bool { return a.pages.HasPage(pageDataSource) })
	h.inject(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone))
	h.waitFor("the form", func(a *App) bool { return a.picker != nil && a.picker.pages.HasPage(pickerForm) })

	for _, r := range "second" {
		h.inject(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	h.press(tcell.KeyTab)
	for _, r := range "db2" {
		h.inject(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	h.press(tcell.KeyTab) // port
	h.press(tcell.KeyTab) // user
	for _, r := range "u" {
		h.inject(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	for i := 0; i < 10; i++ {
		h.press(tcell.KeyTab)
	}
	h.press(tcell.KeyEnter) // Save

	h.waitFor("the file", func(a *App) bool {
		saved, err := config.Load(path)
		return err == nil && len(saved.DataSources) == 2
	})
	h.waitFor("the list again", func(a *App) bool { return len(a.cfg.DataSources) == 2 && !a.picker.pages.HasPage(pickerForm) })
}
```

`filepath` import를 추가한다.

- [ ] **Step 2: 실패를 확인한다**

```sh
go vet -tags integration ./internal/ui/
```

기대: `a.configPath`, `a.picker` undefined.

- [ ] **Step 3: `App`을 고친다**

`internal/ui/app.go`:
- `Deps`에 추가한다.

```go
	// ConfigPath is where the datasource dialog saves. Empty means the
	// dialog can connect but not add or edit, and says so.
	ConfigPath string
	// Secrets stores passwords typed into the dialog. Nil means they cannot
	// be stored here, which the form says instead of pretending.
	Secrets secret.Store
	// Probe answers the dialog's Test button.
	Probe func(ctx context.Context, ds *config.DataSource, password string) (string, error)
```

- `App`에 필드 `configPath string`, `secrets secret.Store`, `probe func(...)`, `picker *dsPicker`를 추가하고 `New()`에서 대입한다.
- `dispatch()`의 `ActionSwitchDataSource` case는 그대로 `a.showDataSources()`.

`internal/ui/datasource.go`: `showDataSources()`와 `dataSourceChoices()`를 지우고 다음을 넣는다. `switchTo`, `openDataSource`, `adopt`, `paintSpine`은 그대로.

```go
// showDataSources opens the list: connect to another, or change the file.
func (a *App) showDataSources() {
	if a.connect == nil {
		a.notice("this session cannot switch datasource")
		return
	}

	save := func() error {
		if a.configPath == "" {
			return errors.New("no configuration file to save to")
		}
		return config.Save(a.configPath, a.cfg)
	}
	a.picker = newDSPicker(a.app, pickerDeps{
		cfg:     a.cfg,
		current: a.conn.DataSource().Name,
		save:    save,
		secrets: a.secrets,
		probe:   a.probe,
		close:   a.closeDataSources,
		connect: func(ds *config.DataSource) {
			a.closeDataSources()
			a.switchTo(ds)
		},
	})
	a.pages.AddPage(pageDataSource, centred(a.picker.Primitive(), 80, 24), true, true)
	a.app.SetFocus(a.picker.Primitive())
}

func (a *App) closeDataSources() {
	a.pages.RemovePage(pageDataSource)
	a.picker = nil
	a.app.SetFocus(a.editor)
}
```

`errors` import를 추가하고, `match`·`complete` import가 안 쓰이면 지운다(`adopt`가 `complete`를 쓴다).

`switchTo`에서 `if ds.Name == a.conn.DataSource().Name { return }`를 `{ a.notice("already on " + ds.Name); return }`로 바꾼다.

`internal/keymap/action.go`의 `descriptions[ActionSwitchDataSource]`를 `"open the datasource list: connect, add, edit, delete"`로 바꾼다.

`cmd/dv/main.go`의 `openSession`에서 `ui.Deps`에 `ConfigPath: path, Secrets: secrets(), Probe: probe`를 더한다.

`internal/ui/app_integration_test.go`의 `harnessWith`에서 `Deps`에 `Secrets: secret.NewMemory(), Probe: func(context.Context, *config.DataSource, string) (string, error) { return "test", nil }`를 더한다. `ConfigPath`는 테스트가 필요할 때 `h.app.configPath`로 직접 준다.

- [ ] **Step 4: 통합 테스트를 돌린다**

```sh
go test -tags integration ./internal/ui/ -run 'DatasourceKey|AddingADatasourceInSession|Switch' -v
```

기대: PASS.

- [ ] **Step 5: 검증과 커밋**

```sh
go build ./... && go vet -tags integration ./... && go test ./... && gofmt -l .
git add -A
git commit -m "The datasource key opens the same list the launcher shows

Connect, add, edit and delete without leaving the session. Saving goes
through config.Save; the password field goes to the keychain."
```

---

### Task 8: 문서 — README, CLAUDE.md, CHANGELOG, 릴리스 노트

**Files:**
- Modify: `README.md`, `CLAUDE.md`, `CHANGELOG.md`, `.goreleaser.yaml`

**Interfaces:** 없음. 이 태스크는 사용자에게 보이는 글이다.

- [ ] **Step 1: README를 다시 쓴다**

`README.md`를 다음 구성으로 전면 교체한다. 남길 절은 "Install"(Homebrew, 스크립트, `go install`, quarantine 안내, attestation)뿐이고, 그것도 `dv init` 언급을 `dv`로 바꾼다.

```
# datavase

A terminal MySQL/MariaDB client. Single static binary, no runtime, no IDE.

Four things, and no more:

- **Datasources.** Keep as many as you like; connect, add, edit and delete them from inside.
- **Look at a table.** Double-click one in the tree, or Enter on one in the tables tab, for its first hundred rows.
- **Run a statement** with ⌘↩ (Ctrl+↩, or F5).
- **Read the answer** in a grid, and copy the whole of it as Markdown or JSON with ⌘⇧C.

Also there, because the four need them: an SSH tunnel to a bastion, table and column completion,
a searchable history of what you ran, passwords in the OS keychain, TLS.

Not there, on purpose: a modal editor, a command palette, a production guard, an explain plan,
the process list, sessions that outlive the terminal. v0.8.x is the last release with those.

## Install
(기존 절 유지; `dv init`→`dv`)

## First run
`dv` with no configuration opens the datasource list. `a` adds one; the form asks for host, user,
password and the rest, `Test` tries it, `Save` writes ~/.config/datavase/config.yaml and puts the
password in the keychain. Enter on an entry connects.

## Configure
(config.yaml 예시: name/host/port/user/database/tls/tunnel, `defaults:` 4개. `env`는 언급하지 않는다.)
The file is written by the datasource dialog, so comments in it do not survive a save.
Passwords: `dv auth`, `DATAVASE_PASSWORD_<NAME>` — 기존 문단 유지.
An absent `tls:` means `preferred`. Anyone coming from 0.8 with `env: prod` and no `tls:` keeps
`required`, and should write `tls: required` down before `env` stops being read.

## Use
dv / dv open <name> / dv ls / dv auth / dv check / dv version

### Keys
(스펙의 키 표를 그대로. `⌘`와 `Ctrl`이 같다는 문단, F키 대체 문단, "modified keys need the extended
keyboard protocol; F5 always runs" 한 문단.)

### The screen
(트리·테이블 탭·편집기·격자·상태 바 한 문단씩. 마우스: 클릭으로 창 이동, 헤더 클릭 정렬, 더블클릭 행 보기,
테이블 더블클릭 미리보기, `copy` 클릭. `mouse: false`.)

## Checking where a download came from
(기존 절 유지)
```

기존 README에서 지울 절: Status의 "Built/Not built" 목록, `dv init` 절, Keys 표의 삭제된 행, "When something else has taken the key", "Making your terminal deliver them", "Changing the keys", "The session outlives the terminal", worktree·explain·procs·guard·vim에 관한 모든 문단.

- [ ] **Step 2: `CLAUDE.md`를 고친다**

- "Commands" 절: `dv keys` 언급 삭제. `internal/testssh`는 유지.
- "Architecture › The dependency rule": `guard`, `vim` 항목을 지우고, `sqlparse` 항목에 `AutoLimit`을 더한다. `ui`의 파일 목록을 `editor.go`/`edit.go`/`motion.go`(text), `status.go`/`topbar.go`, `tree.go`/`tables.go`/`preview.go`(schema pane), `grid.go`/`gridcopy.go`/`copyresult.go`(results), `searchbox.go`/`history.go`/`useschema.go`/`dspicker.go`/`dsform.go`/`launch.go`(dialogs)로 바꾼다.
- "Two connections per datasource", "Streaming and cancellation", "Local schema cache" 절은 그대로.
- "Keyboard presets" 절을 지우고 다음 한 절로 바꾼다.

```
### One key map

`keymap.Default()` is the map; configuration cannot change it. Every action
must appear exactly once in `helpGroups` (`internal/ui/dialog_test.go`),
and an action's description must not be a substring of another's, because
the rendered help is checked by counting them.
```

- "The first ten minutes" 절을 지우고 다음으로 바꾼다.

```
### The first run

`dv` with no configuration opens the datasource list without a session
(`ui.Launch`). It is a separate small tview application rather than a
state of `App`, because `App` assumes a connection in nearly every
method. `config.Save` is the file's only writer; comments in the file do
not survive it.
```

- "Conventions › UI tests": `newVimHarness` 문장을 지운다. `h.do`, `h.waitFor`, `h.buffer` 문단은 유지.
- "Things that are checked automatically": `vim.Reference()` 항목을 지운다.
- "Impossible key combinations" 절은 유지.

- [ ] **Step 3: CHANGELOG에 항목을 쓴다**

`CHANGELOG.md`의 `## v0.8.0` 위에 넣는다.

```markdown
## v0.9.0 — Unreleased

**This release removes most of what v0.8 did.** Anyone who uses the modal
editor, the command palette, the production guard, explain, the process
list, a directory of SQL files or a session that survives closing the
terminal should stay on v0.8.x — it is the last release with them, and
Homebrew will offer this one as an upgrade.

What is left is a datasource list, a schema tree, an editor, a grid, and
four things they do: keep several datasources and manage them in place,
show a table's first hundred rows on a double-click, run a statement with
⌘↩, and copy the whole result as Markdown or JSON with ⌘⇧C.

**Before upgrading, check two things in your config.**

- `env:` no longer drives anything but the default for `tls:`. If a
  production datasource has `env: prod` and no `tls:`, write `tls:
  required` down now; `env` stops being read in a later release.
- `keymap:` is ignored. dv says so once on startup. Delete the block.

### Added

**The datasource dialog.** `dv` with no configuration opens it; `⌘⇧D`
opens it in a session. Add, edit, delete, test and connect. It writes the
config file, which means comments in the file do not survive a save.

**Copy the whole result.** `⌘⇧C`, `F3` or the `copy` label on the result
header asks Markdown or JSON and puts the lot on the clipboard. The notice
states the size, because a terminal's OSC 52 ceiling is exceeded silently.

**Table preview.** Double-click a table in the tree, or Enter on one in
the tables tab, for `SELECT * … LIMIT 100`. The editor is not touched.

### Removed

The session server and detach, the worktree and `--dir`, explain and
analyze, the process list and locks, the definition tab, the production
guard and `unlock writes`, the command palette and `:` command line, the
right-click menu, the first-run card, the vim keyboard, keymap presets
and overrides, `dv init`, `dv keys`, `dv server`, `dv status`, `dv api`.
```

- [ ] **Step 4: 릴리스 노트 템플릿을 고친다**

`.goreleaser.yaml` 165행의 `Then \`dv init\` sets up the first connection, and \`dv version\` says which build you have.`를 `Then \`dv\` opens the datasource list on first run, and \`dv version\` says which build you have.`로 바꾼다.

- [ ] **Step 5: 릴리스 설정을 검증한다**

```sh
go run github.com/goreleaser/goreleaser/v2@latest check
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean
```

기대: 둘 다 성공. 스냅샷 산출물은 `dist/`에 생기며 커밋하지 않는다(`.gitignore`에 있는지 확인한다).

- [ ] **Step 6: 마지막 전체 검증**

```sh
make lint && make test
make db-up && make test-integration ; make db-down
make build && ./dv version && ./dv help
find internal cmd -name '*.go' ! -name '*_test.go' | xargs cat | wc -l
```

기대: 전부 초록. 마지막 줄이 10,000 아래인지 기록한다.

- [ ] **Step 7: 커밋**

```sh
git add README.md CLAUDE.md CHANGELOG.md .goreleaser.yaml
git commit -m "Document the simple version, and say what v0.8 was the last to do

The README describes four things and the few that serve them. The
changelog tells anyone upgrading which config keys to look at and that
0.8.x is where the rest of the features stay."
```

---

## Part 2 완료 조건 (스펙 "끝났을 때 참이어야 하는 것")

- `dv`를 설정 없이 실행하면 데이터소스 목록이 뜨고, `a`로 추가·저장하면 연결된다.
- 트리에서 테이블을 더블클릭하면 100행이 격자에 보이고 편집기는 그대로다.
- `⌘↩`가 커서 아래 문장을 실행하고, 무제한 SELECT에는 LIMIT이 붙는다(상태 바에 `LIMIT n added`).
- `⌘⇧C` → Markdown이 클립보드에 있고, 붙여넣으면 표로 렌더링된다.
- v0.8.0의 `config.yaml`(`env`, `keymap` 포함)이 그대로 열린다.
- `go list ./...`에 `guard`, `vim`, `daemon`이 없다.
- 프로덕션 코드가 10,000줄 아래다.

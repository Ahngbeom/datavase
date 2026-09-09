# 초단순 버전 Part 1: 삭제 — 구현 계획

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 데몬, worktree, explain/procs/DDL, 가드, 팔레트·커맨드라인·메뉴·소개 카드, vim, 키맵 프리셋을 저장소에서 지우고, 남는 것만으로 `dv`가 빌드·테스트·실행되게 한다.

**Architecture:** 패키지와 파일은 통째로 지우고, 모든 기능이 만나는 배선(`internal/ui/app.go`, `cmd/dv/main.go`, `internal/cli/cli.go`, `internal/keymap`)만 줄어든 의존성에 맞춰 고친다. 검증된 위젯 파일과 `internal/ui`의 테스트 하네스는 유지한다. 컴파일러가 체크리스트다: 파일을 지운 뒤 `go build`와 `go vet -tags integration`이 이름을 대는 참조를 없앤다. 각 태스크는 빌드·단위 테스트·lint가 초록인 커밋 하나로 끝난다.

**Tech Stack:** Go 1.26, tview/tcell, `go-sql-driver/mysql`, `modernc.org/sqlite`, `gopkg.in/yaml.v3`.

**Spec:** `docs/superpowers/specs/2026-09-08-simple-mode-design.md`

## Global Constraints

- CGO-free 단일 정적 바이너리(`CGO_ENABLED=0 go build -o dv ./cmd/dv`)는 유지한다.
- 유지 패키지: `db`, `session`, `tunnel`, `result`, `export`, `sqlparse`, `catalog`, `complete`, `history`, `secret`, `match`, `version`, `config`, `cli`, `keymap`(축소), `ui`(축소), `testmysql`, `testssh`.
- `Ctrl`과 `⌘`는 같은 뜻이고 F키 대체는 남긴다(`keymap/map.go`의 `ctrlAndCmd*` 헬퍼 유지).
- 통합 테스트는 `//go:build integration` 태그를 유지하고 포트 13306을 쓴다.
- 트랜잭션 제어(`BEGIN`/`COMMIT`/`ROLLBACK` 타이핑)와 `auto_limit`은 남긴다.
- v0.8.0의 `config.yaml`(`env`, `keymap` 포함)은 계속 열려야 한다. `env`는 TLS 기본값에만 쓰고, `keymap`은 무시하고 알린다.
- 주석은 "왜"만 쓴다. "예전에 X가 있었다" 같은 diff 서술 주석은 쓰지 않는다.
- 커밋 메시지 끝에 다음 두 줄을 붙인다.
  ```
  Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_01XNDczFdErnBrLyCKRymrkh
  ```

## 매 태스크 공통 검증 명령

`go test ./...`는 `//go:build integration` 파일을 컴파일하지 않는다. 통합 테스트 파일에 남은 죽은 참조는 `go vet -tags integration`이 잡는다. **모든 태스크의 검증은 이 네 줄이다.**

```sh
go build ./... && go vet ./... && go vet -tags integration ./...
go test ./...
gofmt -l .        # 출력이 없어야 한다
```

MariaDB가 떠 있으면(`make db-up`) 태스크 끝에 `go test -tags integration ./internal/ui/ ./internal/cli/`도 돌린다. 안 떠 있으면 계획의 마지막 태스크에서 한 번 돌린다.

## 파일 구조 (Part 1이 끝났을 때)

지워지는 것:

| 경로 | 이유 |
|---|---|
| `internal/daemon`, `internal/attach`, `internal/proto`, `internal/snapshot`, `internal/screen` | 세션 유지(detach/attach) |
| `cmd/dv/runtime.go`, `runtime_test.go`, `spawn_unix.go`, `spawn_windows.go` | 데몬 기동 |
| `internal/worktree`, `internal/recent` | SQL 디렉터리 |
| `internal/explain`, `internal/procs` | 실행 계획, 프로세스 목록 |
| `internal/guard` | 프로덕션 가드 (auto LIMIT만 `sqlparse`로 이전) |
| `internal/vim`, `internal/intro` | 모달 편집기, 첫 실행 카드 |
| `internal/ui/{vimedit,vimmotion,vimnav,vimobject,vimrepeat,palette,cmdline,menu,worktree,files,savefile,explain,sessions,ddl,intro,goto}.go` | 각 기능의 UI |
| `internal/keymap/{preset,snippet}.go` | 프리셋, 터미널 스니펫 |
| `internal/config/keymap.go` | 키맵 설정 |
| `internal/cli/{init,keys,keydebug}.go` | 위저드, `dv keys` |

새로 생기는 것:

| 경로 | 책임 |
|---|---|
| `internal/ui/history.go` | 히스토리 검색 다이얼로그 (`palette.go`에서 이전) |
| `internal/ui/confirm.go` | 되돌릴 수 없는 일 앞의 확인 다이얼로그 (`savefile.go`에서 이전) |
| `internal/sqlparse/limit.go`의 `AutoLimit` | 무제한 SELECT에 붙일 LIMIT 결정 |

---

### Task 1: 런타임 절단 — 데몬 없이 프로세스 안에서 UI를 띄운다

**Files:**
- Delete: `cmd/dv/runtime.go`, `cmd/dv/runtime_test.go`, `cmd/dv/spawn_unix.go`, `cmd/dv/spawn_windows.go`, `internal/daemon/`, `internal/attach/`, `internal/proto/`, `internal/snapshot/`, `internal/screen/`, `internal/ui/snapshot_integration_test.go`, `internal/keymap/detach_test.go`
- Modify: `cmd/dv/main.go`, `internal/cli/cli.go`, `internal/cli/cli_test.go`, `internal/ui/app.go`, `internal/ui/dialog.go`, `internal/keymap/action.go`, `internal/keymap/map.go`

**Interfaces:**
- Produces: `cli.App`에서 `Attach`, `RunServer`, `StopServer`, `ServerStatus`, `APISnapshot` 필드가 사라진다. `ui.Deps.Detach`가 사라진다. `keymap.ActionDetach`가 사라진다. UI 기동 경로는 `cli.open` → `App.OpenUI`(= `main.openUI`) 하나뿐이다.

- [ ] **Step 1: 파일과 패키지를 지운다**

```sh
git rm -r -q internal/daemon internal/attach internal/proto internal/snapshot internal/screen
git rm -q cmd/dv/runtime.go cmd/dv/runtime_test.go cmd/dv/spawn_unix.go cmd/dv/spawn_windows.go
git rm -q internal/ui/snapshot_integration_test.go internal/keymap/detach_test.go
```

- [ ] **Step 2: `cmd/dv/main.go`를 고친다**

`run()`(37–117행)에서:
- 46행 `noSession` 플래그 선언과 107–115행의 `if !*noSession { app.Attach = ... }` 블록을 지운다.
- 94–106행 `cli.App` 리터럴에서 `RunServer`, `StopServer`, `ServerStatus`, `APISnapshot` 네 필드를 지운다.
- 200–275행 `sessionAdapter`, `closingSession`과 276–382행 `buildSession`을 통째로 지운다.
- import에서 `daemon`, `proto`를 지운다. `context`, `errors`, `io`, `fs`, `strconv`, `sync` 등은 컴파일러가 "imported and not used"라고 말하는 것만 지운다.

- [ ] **Step 3: `internal/cli/cli.go`를 고친다**

- `App` 구조체(29–63행)에서 `Attach`(47–52), `RunServer`(55), `StopServer`(58), `ServerStatus`(60), `APISnapshot`(62)를 지운다.
- `Run` 디스패치(96–126행)에서 `server`, `status`, `api` 세 case를 지운다. (`keys`는 Task 7에서.)
- `open()`(211–257행)의 `if a.Attach != nil { ... }` 블록(234–240)을 지운다. 남는 흐름은 키체인에서 password를 읽어 `a.OpenUI(...)`를 부르는 것뿐이다.
- `server()`(296–329), `serverStatus()`(331–344), `api()`(346–363) 메서드를 지운다.
- `usage()`(128–158행)에서 `dv status`, `dv api snapshot`, `dv --no-session`, `advanced:` 절 전체를 지운다.

- [ ] **Step 4: `internal/cli/cli_test.go`에서 다섯 테스트를 지운다**

`TestOpenPrefersAttachAndDoesNotPrompt`, `TestWithoutARuntimeOpenUIStillRuns`, `TestStatusReportsWithNoServer`, `TestServerStopPassesForceOnlyWhenTyped`, `TestServerStopReportsWhatStopServerSaid`.

- [ ] **Step 5: `internal/ui/app.go`를 고친다**

- `App` 구조체에서 `detachFn`(53행), `runningSQL`(211), `startedAt`(216)을 지운다. `start()`의 `a.runningSQL = sql`, `a.startedAt = started`와 `consume()`의 같은 대입(1434–1435)을 지운다.
- `Deps.Detach`(283–286행)와 `New()`의 `detachFn: deps.Detach`(316)를 지운다.
- `detach()`(1124–1135), `RuntimeState`(1656–1661), `State()`(1663–1690), `Snapshot()`(1692–1709), `snapshot()`(1711–1772), `SwitchTo()`(1780–1790)를 지운다.
- `dispatch()`의 `case keymap.ActionDetach:`(1031)를 지운다.
- import에서 `snapshot`과 `strings`(1748행에서만 쓰였다)를 지운다.

- [ ] **Step 6: `keymap`에서 `ActionDetach`를 지운다**

`internal/keymap/action.go`: 상수(91행), `actionNames`의 `"detach"`(142), `descriptions`(193), `familiar` 맵의 항목, `order` 배열의 항목. `internal/keymap/map.go`: `m.bind(ActionDetach, ...)`(336–338).

`internal/ui/dialog.go`의 `helpGroups` 마지막 그룹을 다음으로 바꾼다.

```go
	{
		title:   "Other",
		actions: []keymap.Action{keymap.ActionHelp, keymap.ActionQuit},
	},
```

- [ ] **Step 7: 빌드하고, 컴파일러가 대는 나머지 참조를 지운다**

```sh
go build ./... && go vet ./... && go vet -tags integration ./...
```

기대: `keymap_test.go`의 `TestFallbackKeys`/`TestBindingLabel`, `app_integration_test.go`, `e4_integration_test.go`, `status_test.go` 등에서 `ActionDetach`, `Detach`, `Snapshot`, `RuntimeState`를 참조하는 테스트 함수(또는 테이블 행)가 에러로 나온다. **그 함수(행)만 지운다.** 파일 전체를 지우지 않는다.

- [ ] **Step 8: 테스트와 lint**

```sh
go test ./... && gofmt -l .
```

기대: 전부 PASS, gofmt 출력 없음.

- [ ] **Step 9: 커밋**

```sh
git add -A
git commit -m "Run the interface in-process; the session server is gone

Detach, attach, the socket protocol, the screen relay and the JSON
snapshot existed so a statement could outlive the terminal. The simple
version does not promise that, so dv opens the interface in its own
process the way it did before the runtime split."
```

---

### Task 2: worktree 절단 — 파일, 저장, 디렉터리, 테이블 점프

**Files:**
- Delete: `internal/worktree/`, `internal/recent/`, `internal/ui/worktree.go`, `internal/ui/files.go`, `internal/ui/savefile.go`, `internal/ui/goto.go`, `internal/ui/worktree_integration_test.go`, `internal/cli/opendir_test.go`
- Create: `internal/ui/confirm.go`
- Modify: `cmd/dv/main.go`, `internal/cli/cli.go`, `internal/ui/app.go`, `internal/ui/topbar.go`, `internal/ui/topbar_test.go`, `internal/ui/dialog.go`, `internal/ui/tables.go`, `internal/keymap/action.go`, `internal/keymap/map.go`, `internal/keymap/file_test.go`

**Interfaces:**
- Consumes: Task 1 이후의 `ui.Deps`.
- Produces: `ui.Deps`에서 `Worktree`, `Recent`가 사라진다. `cli.UIOptions`가 사라지고 `App.OpenUI`의 시그니처가 `func(ctx context.Context, ds *config.DataSource, password string, cfg *config.Config) error`가 된다. `keymap.ActionGoToTable`, `ActionFindFile`, `ActionSaveFile`이 사라진다. `(a *App) confirmDiscard(text, proceedLabel string, proceed func())`는 `confirm.go`로 옮겨져 남는다.

- [ ] **Step 1: `confirmDiscard`를 `confirm.go`로 옮긴다**

`internal/ui/confirm.go`를 만든다. `savefile.go` 65–84행의 함수를 그대로 가져온다.

```go
package ui

// confirmDiscard asks before an action that throws work away.
//
// There is a way through, because this is the user's own work — the point is
// that it takes a decision rather than a reflex.
func (a *App) confirmDiscard(text, proceedLabel string, proceed func()) {
	modal := newModal().
		SetText(text).
		AddButtons([]string{"Cancel", proceedLabel}).
		SetDoneFunc(func(_ int, label string) {
			a.closeDialog()
			if label == proceedLabel {
				proceed()
			}
		})

	modal.SetTextColor(colourNotice)
	a.openDialog(modal)
}
```

- [ ] **Step 2: 파일과 패키지를 지운다**

```sh
git rm -r -q internal/worktree internal/recent
git rm -q internal/ui/worktree.go internal/ui/files.go internal/ui/savefile.go internal/ui/goto.go
git rm -q internal/ui/worktree_integration_test.go internal/cli/opendir_test.go
```

- [ ] **Step 3: `internal/ui/app.go`를 고친다**

- 구조체 필드 `wt`(189), `wtSnap`(190), `openFile`(192), `recentDirs`(196)를 지운다.
- `Deps.Worktree`(273–275), `Deps.Recent`(276–278)와 `New()`의 `wt:`, `recentDirs:` 대입(322–323), `a.rescan()` 호출(350–352)을 지운다.
- `buildWidgets()`의 `a.editorRegion = newTabbed().watch(a.editorDetail)`를 `a.editorRegion = newTabbed()`로 바꾸고 `editorDetail()`(612–620)을 지운다.
- `currentTopBar()`(719–728)에서 `branch: a.worktreeLabel()` 줄을 지운다.
- `quitWithUnsavedFile()`(1156–1165)을 지우고, `quit()`과 `confirmDiscardTransaction()`에서 그것을 부르던 자리를 `a.forceQuit()`로 바꾼다.
- `dispatch()`에서 `ActionGoToTable`(1022), `ActionFindFile`(1024), `ActionSaveFile`(1026) case를 지운다.
- `pageFiles`(845), `pageAttach`(846), `pageGoTo`(843) 상수를 지운다.
- import에서 `worktree`, `recent`를 지운다.

- [ ] **Step 4: `internal/ui/topbar.go`에서 branch를 지운다**

`topBarState.branch` 필드, `topBarForm.branch` 필드, `topBarForms`의 branch 항목, `line()`의 `if form.branch && t.branch != ""` 블록을 지운다. `topBarForms`는 다음이 된다.

```go
var topBarForms = []topBarForm{
	{helpKey: true, dsName: true},
	{dsName: true},
	{},
}
```

`topbar_test.go`에서 `branch`를 넣는 테스트 케이스는 그 필드만 지운다. branch가 잘려 나가는 순서를 검증하는 테스트가 있으면 그 테스트를 지운다.

- [ ] **Step 5: `internal/ui/tables.go`의 테이블 선택을 임시로 비운다**

151행의 `a.openTable(schema, table.Name)`(goto.go에 있던 함수)을 `a.notice("table preview is not built yet")`로 바꾼다. Part 2의 미리보기 태스크가 이 자리를 채운다.

- [ ] **Step 6: `cli`와 `main`에서 `--dir`을 지운다**

`internal/cli/cli.go`:
- `UIOptions` 타입(70–74)을 지운다.
- `OpenUI` 필드 타입에서 `opt UIOptions` 인자를 뺀다.
- `Run()`의 `a.open("", UIOptions{})`를 `a.open("")`로, `open(name string, opt UIOptions)`을 `open(name string)`으로 바꾼다.
- `openCmd()`(182–205)를 다음으로 바꾼다.

```go
func (a *App) openCmd(args []string) int {
	switch len(args) {
	case 0:
		return a.open("")
	case 1:
		return a.open(args[0])
	default:
		fmt.Fprintf(a.Err, "dv open takes one datasource name; got %q and %q\n", args[0], args[1])
		return exitUsage
	}
}
```

- `usage()`에서 `dv open <name> --dir <path>` 두 줄을 지운다.

`cmd/dv/main.go`:
- `openUI`의 시그니처에서 `opt cli.UIOptions`를 빼고, 170–178행의 worktree 블록을 지운다. `ui.Deps{...}`에서 `Worktree: wt`, `Recent: recents`를 지우고 152–157행의 `recent.Open` 블록을 지운다.

- [ ] **Step 7: `keymap`에서 세 액션을 지운다**

`action.go`: `ActionGoToTable`(60), `ActionFindFile`(62), `ActionSaveFile`(64)와 각각의 `actionNames`(129–131), `descriptions`(180–182), `familiar`, `order` 항목. `map.go`: `GoToTable`(251), `FindFile`(255–257), `SaveFile`(261–263) 바인딩. `file_test.go`: `TestFindFileIsBoundToCommandShiftO`, `TestSaveFileIsBoundToCommandS`, `TestLegacyControlCodesReachTheFileActions`를 지운다. `TestNoTwoActionsShareABinding`은 남긴다.

`internal/ui/dialog.go`의 `helpGroups`에서 `ActionSaveFile`("Editing"), `ActionGoToTable`, `ActionFindFile`("Finding things")을 지운다.

- [ ] **Step 8: 빌드하고, 컴파일러가 대는 나머지 참조를 지운다**

```sh
go build ./... && go vet ./... && go vet -tags integration ./...
```

기대: `app_integration_test.go`(`Recent:` 대입, `recent.Open` 블록 — 하네스의 `harnessWith`에서 그 블록과 필드를 지운다), `cli_test.go`(`UIOptions`), `keymap_test.go`(`TestFallbackKeys`의 F2/F8 행), `dialog_test.go`, `e4_integration_test.go`, `searchbox_integration_test.go`(go-to-table 테스트), `mouse_integration_test.go`, `panel_integration_test.go`(editor detail이 파일명을 보이는 테스트) 등에서 에러가 난다. 함수 단위로 지운다.

- [ ] **Step 9: 테스트와 lint**

```sh
go test ./... && gofmt -l .
```

- [ ] **Step 10: 커밋**

```sh
git add -A
git commit -m "Drop the worktree: no files, no save, no directory, no table jump

A directory of SQL, the file finder, save, the recent-directory list and
go-to-table all served the workflow of editing a repository of queries.
The simple version is an editor and a grid, so the editor's header no
longer names a file and quitting no longer asks about one."
```

---

### Task 3: explain / procs / DDL 절단

**Files:**
- Delete: `internal/explain/`, `internal/procs/`, `internal/ui/explain.go`, `internal/ui/sessions.go`, `internal/ui/ddl.go`, `internal/ui/explain_integration_test.go`, `internal/ui/sessions_integration_test.go`, `internal/ui/sessions_test.go`
- Modify: `internal/ui/app.go`, `internal/ui/gridcopy.go`, `internal/ui/gridcopy_test.go`, `internal/ui/datasource.go`, `internal/ui/dialog.go`, `internal/ui/mouse.go`, `internal/keymap/action.go`, `internal/keymap/map.go`

**Interfaces:**
- Produces: 결과 영역의 탭은 `tabResults` 하나가 된다. `keymap.ActionExplain`, `ActionAnalyze`, `ActionSessions`, `ActionKillSession`, `ActionLocks`가 사라진다. `ActionInspect`는 격자의 행 보기만 한다. `copyContext`는 `running`, `onGrid`, `hasSelection` 세 필드만 갖는다.

- [ ] **Step 1: 파일과 패키지를 지운다**

```sh
git rm -r -q internal/explain internal/procs
git rm -q internal/ui/explain.go internal/ui/sessions.go internal/ui/ddl.go
git rm -q internal/ui/explain_integration_test.go internal/ui/sessions_integration_test.go internal/ui/sessions_test.go
```

- [ ] **Step 2: `internal/ui/gridcopy.go`를 줄인다**

`copyIntent`에서 `intentDefinition`, `intentPlan`을, `copyContext`에서 `onDDL`, `onPlan`을 지운다. `resolve()`는 다음이 된다.

```go
func (c copyContext) resolve() copyIntent {
	if c.running {
		return intentCancel
	}
	switch {
	case c.hasSelection:
		return intentSelection
	case c.onGrid:
		return intentCell
	}
	return intentNothing
}
```

`gridcopy_test.go`에서 `onDDL`/`onPlan`/`intentDefinition`/`intentPlan`을 쓰는 케이스를 지운다.

- [ ] **Step 3: `internal/ui/app.go`를 고친다**

- 구조체 필드 `ddlView`(170), `ddlText`(171), `planView`(175), `sessionsView`(178)를 지운다.
- `buildWidgets()`에서 `a.resultTabs.add(tabDDL, ...)`, `add(tabPlan, ...)`, `add(tabSessions, ...)` 세 줄을 지운다. 상수 `tabDDL`, `tabPlan`, `tabSessions`(831–833)와 `pageKill`(850)을 지운다.
- `resultPrimitive()`(867–878)를 `return a.grid`만 남긴다.
- `dispatch()`에서 `ActionExplain`, `ActionAnalyze`, `ActionSessions`, `ActionKillSession`, `ActionLocks` case를 지운다.
- `copyOrCancel()`(1319–1348)에서 `onDDL:`, `onPlan:` 줄과 `intentDefinition`, `intentPlan` case를 지운다.
- `start()`의 `go a.consume(stream, sql, started, stmt.PlansAsJSON())`를 `go a.consume(stream, sql, started)`로 바꾸고, `consume()`의 `plansJSON bool` 인자와 1451–1453행의 `if plansJSON && ... { a.showPlanFrom(a.buf) }` 블록을 지운다.
- `inspect()`(1648–1654)를 다음으로 바꾼다.

```go
func (a *App) inspect() {
	if a.app.GetFocus() != a.grid {
		a.notice("select a result row first")
		return
	}
	a.showRow()
}
```

- import에서 `procs`를 지운다.

- [ ] **Step 4: `datasource.go`와 `dialog.go`, `keymap`을 고친다**

- `datasource.go`의 `adopt()`에서 `a.ddlText = ""`, `a.ddlView.SetText("")` 두 줄을 지운다.
- `dialog.go`의 `helpGroups`에서 `ActionExplain`, `ActionAnalyze`("Running")와 `ActionSessions`, `ActionKillSession`, `ActionLocks`("Results")를 지운다. "Results" 그룹은 `keymap.ActionSortColumn` 하나가 된다.
- `keymap/action.go`: 다섯 상수(78–87), `actionNames`(136–140), `descriptions`(187–191), `familiar`, `order` 항목. `ActionInspect`의 설명(184)을 `"show the selected result row in full"`로 바꾼다. `keymap/map.go`: 바인딩 304, 308, 318, 323, 326행.
- `mouse.go`의 `gridVisible()` 주석은 DDL/plan/sessions를 언급하는데, 함수는 그대로 두고 주석을 "탭이 하나뿐이어도 `resultTabs.current()` 검사를 남기는 이유"로 줄이거나 그냥 지운다. 함수 본문은 유지한다.

- [ ] **Step 5: 빌드하고 나머지 참조를 지운다**

```sh
go build ./... && go vet ./... && go vet -tags integration ./...
```

기대: `app_integration_test.go`, `m6_integration_test.go`(DDL 탭 테스트), `panel_integration_test.go`(탭 순환 테스트 중 DDL/plan을 세는 것), `resulthint_test.go`(DDL 탭 힌트), `keymap_test.go`, `dialog_test.go`, `e4_integration_test.go`에서 에러. 함수·케이스 단위로 지운다. `resultHint()`의 `if s.tab != tabResults` 분기는 탭이 하나뿐이라 죽은 코드가 되므로 지우고, `resultState.tab`과 `editorFocused` 필드도 함께 지운다. `resulthint_test.go`의 관련 케이스도 지운다.

- [ ] **Step 6: 테스트와 lint, 커밋**

```sh
go test ./... && gofmt -l .
git add -A
git commit -m "Drop explain, the process list and the definition tab

The result region holds one tab now. Inspect shows a row and nothing
else, and the copy key resolves between the editor's selection and the
grid's cell without a plan or a definition to consider."
```

---

### Task 4: 가드 → `sqlparse.AutoLimit`; `env`는 TLS 기본값에만

**Files:**
- Delete: `internal/guard/`, `internal/ui/guard_integration_test.go`
- Modify: `internal/sqlparse/limit.go`, `internal/sqlparse/limit_test.go`, `internal/ui/app.go`, `internal/ui/dialog.go`, `internal/ui/status.go`, `internal/ui/status_test.go`, `internal/ui/mouse.go`, `internal/ui/zone.go`, `internal/ui/palette.go`, `internal/ui/cmdline.go`, `internal/ui/datasource.go`, `internal/ui/topbar.go`, `internal/ui/topbar_test.go`, `internal/ui/spine.go`, `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces:**
- Produces: `sqlparse.AutoLimit(stmt Statement, n int) int`. `(a *App) start(stmt sqlparse.Statement, limit int)`. `status.writesEnabled`, `zoneStatusWrites`가 사라진다. `config.DataSource.Env`는 비어 있어도 된다. `topBarState.env`가 사라지고 칩은 데이터소스 이름을 보인다. `spine.go`는 `spineColour` 하나만 갖는다.

- [ ] **Step 1: `AutoLimit`의 실패하는 테스트를 쓴다**

`internal/sqlparse/limit_test.go`에 추가한다(파일이 없으면 만든다).

```go
func TestAutoLimitProposesALimitForAnUnboundedSelect(t *testing.T) {
	stmt := Parse("SELECT * FROM users")
	if got := AutoLimit(stmt, 1000); got != 1000 {
		t.Errorf("AutoLimit = %d, want 1000", got)
	}
}

func TestAutoLimitLeavesAnExplicitLimitAlone(t *testing.T) {
	stmt := Parse("SELECT * FROM users LIMIT 5")
	if got := AutoLimit(stmt, 1000); got != 0 {
		t.Errorf("AutoLimit = %d, want 0; an explicit LIMIT must win", got)
	}
}

func TestAutoLimitIgnoresALimitInsideASubquery(t *testing.T) {
	stmt := Parse("SELECT * FROM (SELECT id FROM users LIMIT 5) u")
	if got := AutoLimit(stmt, 1000); got != 1000 {
		t.Errorf("AutoLimit = %d, want 1000; the subquery's LIMIT does not bound the outer SELECT", got)
	}
}

func TestAutoLimitIsOffWhenTheSettingIsZero(t *testing.T) {
	stmt := Parse("SELECT * FROM users")
	if got := AutoLimit(stmt, 0); got != 0 {
		t.Errorf("AutoLimit = %d, want 0", got)
	}
}

func TestAutoLimitOnlyAppliesToSelect(t *testing.T) {
	for _, sql := range []string{"SHOW TABLES", "DELETE FROM users", "INSERT INTO t VALUES (1)", ""} {
		if got := AutoLimit(Parse(sql), 1000); got != 0 {
			t.Errorf("AutoLimit(%q) = %d, want 0", sql, got)
		}
	}
}
```

- [ ] **Step 2: 실패를 확인한다**

```sh
go test ./internal/sqlparse/ -run TestAutoLimit
```

기대: `undefined: AutoLimit`으로 빌드 실패.

- [ ] **Step 3: `AutoLimit`을 구현한다**

`internal/sqlparse/limit.go`의 `AppendLimit` 위에 추가한다.

```go
// AutoLimit is the LIMIT to append to stmt, or zero to leave it alone.
//
// Only a SELECT with no LIMIT of its own at the top level gets one. A LIMIT
// inside a subquery bounds nothing about the outer result, which is why the
// check is HasTopLevelLimit rather than a search for the keyword.
func AutoLimit(stmt Statement, n int) int {
	if n <= 0 || stmt.IsEmpty() || stmt.Kind() != StmtSelect || stmt.HasTopLevelLimit() {
		return 0
	}
	return n
}
```

- [ ] **Step 4: 통과를 확인하고, 구현을 변형해 테스트가 무는지 본다**

```sh
go test ./internal/sqlparse/ -run TestAutoLimit -v
```

기대: 5개 PASS. 확인 후 `stmt.HasTopLevelLimit()` 조건을 잠시 빼고 다시 돌려 `TestAutoLimitLeavesAnExplicitLimitAlone`이 FAIL하는지 본 뒤 되돌린다.

- [ ] **Step 5: `guard`를 지우고 실행 경로를 다시 쓴다**

```sh
git rm -r -q internal/guard
git rm -q internal/ui/guard_integration_test.go
```

`internal/ui/app.go`:
- `runStatement()`(1196–1216)를 다음으로 바꾼다.

```go
// runStatement sends one statement, or opens or ends a transaction when
// that is what it says.
func (a *App) runStatement(stmt sqlparse.Statement) {
	if stmt.IsEmpty() {
		a.notice("nothing to run")
		return
	}
	// Transaction control opens or ends the pinned connection rather than
	// running on one, so it never becomes a Stream.
	if opensOrEndsTransaction(stmt) {
		a.transactionControl(stmt.Verb())
		return
	}
	a.start(stmt, sqlparse.AutoLimit(stmt, a.cfg.Defaults.AutoLimit))
}
```

- `advanceBatch()`(1248–1271)에서 `guard.Evaluate`와 Deny/Confirm 분기를 지우고, 다음 문장을 `a.start(stmt, sqlparse.AutoLimit(stmt, a.cfg.Defaults.AutoLimit))`로 보낸다. 트랜잭션 제어 문장이 배치 안에 있으면 `a.transactionControl(stmt.Verb())` 후 `a.advanceBatch()`를 이어 부른다.
- `policy()`(1350–1357)를 지운다.
- `start(stmt sqlparse.Statement, decision guard.Decision)`을 `start(stmt sqlparse.Statement, limit int)`로 바꾸고, 본문의 `decision.InjectLimit`을 `limit`으로 바꾼다(`if limit > 0 { sql = sqlparse.AppendLimit(stmt, limit) }`, `a.status.limitInjected = limit`).
- import에서 `guard`를 지운다.

`internal/ui/dialog.go`: `refusalText`, `unlockHint`, `refuse`, `confirm`, `confirmWithButtons`, `confirmByTyping`(24–140행 근방)을 지운다. `pageConfirm` 상수는 `openDialog`/`closeDialog`가 쓰므로 남긴다.

`internal/ui/status.go`: `writesEnabled` 필드(39)와 렌더 267–271행을 지운다. `field.target` 주석(84–86)에서 "unlocked writes" 언급을 지운다. `status_test.go`에서 `writesEnabled`를 쓰는 케이스를 지운다.

`internal/ui/zone.go`: `zoneStatusWrites`를 지운다. `mouse.go`의 `mouseAction()`에서 `case zoneStatusWrites:` 블록을 지운다.

`internal/ui/palette.go`: `enableWrites`, `disableWrites` 함수와 그것을 부르는 `paletteCommands()`의 항목("unlock writes", "lock writes")을 지운다. `cmdline.go`에서 `unlock writes`의 전체 이름 규칙(약어 거부)을 다루는 코드를 지운다. (팔레트와 커맨드라인 자체는 Task 5에서 지운다. 여기서는 가드에 닿는 부분만 뺀다.)

`internal/ui/datasource.go`: `adopt()`의 `a.status.writesEnabled = false`와 그 주석을 지운다. `dataSourceChoices()`의 `detail` 문자열에서 `ds.Env`를 빼 `fmt.Sprintf("%s@%s:%d", ds.User, ds.Host, ds.Port)`로 바꾼다. `adopt()` 끝의 notice를 `fmt.Sprintf("switched to %s", ds.Name)`로 바꾼다.

- [ ] **Step 6: 상단 바의 env 칩을 데이터소스 칩으로 바꾼다**

`internal/ui/spine.go`: `spineProd`, `spineStage`, `spineTextLoud`, `envStyle`, `envStyleFor`를 지운다. 남는 색은 이름을 바꾼다.

```go
var (
	spineColour = tcell.NewRGBColor(0x2E, 0x33, 0x3A)
	spineText   = tcell.NewRGBColor(0x9A, 0xA3, 0xAD)
)
```

`datasource.go`의 `paintSpine()`는 `a.spine.SetBackgroundColor(spineColour)`가 된다. `New()`나 `buildLayout()`에서 스파인 색을 정하는 곳도 같은 상수를 쓴다.

`internal/ui/topbar.go`:
- `topBarState.env` 필드를 지운다. import에서 `config`를 지운다.
- `topBarForm`에서 `dsName`을 지우고 `topBarForms`를 `{{helpKey: true}, {}}`로 바꾼다.
- `chip()`을 다음으로 바꾼다.

```go
// chip is the datasource name, filled rather than merely coloured, so that
// which server this is survives on a terminal of any width.
func (t topBarState) chip() string {
	return fmt.Sprintf("[%s] %s [-:-]", colourTag(spineText, spineColour), result.EscapeTags(t.dsName))
}
```

- `line()`에서 `if (form.dsName && t.dsName != "") || t.schema != ""` 블록을 다음으로 바꾼다. 칩 전체가 데이터소스 존이다.

```go
	line := ""
	var zones []zone
	mark := func(before string, target zoneTarget) { /* 그대로 */ }

	before := line
	line += t.chip()
	mark(before, zoneDataSource)

	if t.schema != "" {
		line += " @"
		before := line
		line += result.EscapeTags(t.schema)
		mark(before, zoneSchema)
	}
```

- `renderWidth()`의 마지막 fallback 주석에서 "environment"를 "datasource"로 고친다.

`app.go`의 `currentTopBar()`에서 `env:` 대입을 지운다.

`topbar_test.go`: `env:`를 넣는 케이스에서 그 필드를 지우고, 칩 텍스트가 대문자 env(`DEV`, `PROD`)이길 기대하는 단언을 데이터소스 이름으로 바꾼다. "env는 절대 잘리지 않는다"를 검증하던 테스트는 "데이터소스 이름은 절대 잘리지 않는다"로 이름과 단언을 바꾼다.

- [ ] **Step 7: `config`에서 `env`를 선택 필드로 만든다**

`internal/config/config.go`:
- `Env` 타입과 상수, `DefaultTLSMode`는 그대로 둔다. 타입 주석(16–18행)을 다음으로 바꾼다.

```go
// Env is kept from earlier configurations for one reason: an absent "tls:"
// defaults by it, and dropping that would quietly let a production
// credential cross the wire in clear text. Nothing else reads it.
```

- `DataSource.Env`의 태그를 `yaml:"env,omitempty"`로 바꾼다.
- `DataSource.validate()`의 env switch를 다음으로 바꾼다.

```go
	switch d.Env {
	case "", EnvProd, EnvStage, EnvDev:
	default:
		return fmt.Errorf("datasource %q: env must be one of %q, %q, %q (got %q)",
			d.Name, EnvProd, EnvStage, EnvDev, d.Env)
	}
```

`internal/config/config_test.go`: `TestParseMinimalDataSource`가 `env`를 요구하면 그 요구를 없애고, `TestParseRejectsInvalidConfig`의 테이블에서 "env missing"류 케이스를 지운다(잘못된 값 케이스는 남긴다). `TestTLSDefaultsFollowTheEnvironment`에 "env가 없으면 preferred" 케이스를 하나 추가한다.

```go
	{name: "no env", env: "", want: TLSPreferred},
```

- [ ] **Step 8: 빌드하고 나머지 참조를 지운다**

```sh
go build ./... && go vet ./... && go vet -tags integration ./...
```

기대: `dialog_test.go`(refusalText 테스트), `status_test.go`, `m6_integration_test.go`/`app_integration_test.go`(unlock, confirm 다이얼로그, `newHarness(t, config.EnvProd)`로 거부를 검증하는 테스트), `palette_test.go`, `cmdline_test.go`, `topbar_test.go`, `mouse_integration_test.go`(writes 존 클릭), `theme_test.go`/`theme_integration_test.go`(env 색), `datasource_integration_test.go`(env 표시) 등. 함수·케이스 단위로 지운다. `newHarness(t, env config.Env)` 시그니처는 그대로 두되 `ds.Env = env` 대입은 남겨도 무해하다(Part 2에서 정리).

- [ ] **Step 9: 테스트와 lint, 커밋**

```sh
go test ./... && gofmt -l .
git add -A
git commit -m "Replace the guard with an auto LIMIT; env only picks the TLS default

Every statement runs. The one thing kept from the guard is the LIMIT on
an unbounded SELECT, which now lives beside AppendLimit in sqlparse. The
env field stays readable because an absent tls: defaults by it, and a
production config would otherwise drop to clear text without a word."
```

---

### Task 5: 팔레트 · 커맨드라인 · 메뉴 · 소개 카드 절단

**Files:**
- Delete: `internal/intro/`, `internal/ui/palette.go`, `internal/ui/cmdline.go`, `internal/ui/menu.go`, `internal/ui/intro.go`, `internal/ui/palette_test.go`, `internal/ui/cmdline_test.go`, `internal/ui/cmdline_integration_test.go`, `internal/ui/menu_test.go`, `internal/ui/menu_integration_test.go`, `internal/ui/intro_test.go`, `internal/ui/intro_integration_test.go`, `internal/ui/discoverable_test.go`, `internal/ui/discoverable_integration_test.go`
- Create: `internal/ui/history.go`
- Modify: `cmd/dv/main.go`, `internal/ui/app.go`, `internal/ui/dialog.go`, `internal/ui/mouse.go`, `internal/ui/zone.go`, `internal/ui/status.go`, `internal/ui/app_integration_test.go`, `internal/keymap/action.go`, `internal/keymap/map.go`, `internal/keymap/file_test.go`

**Interfaces:**
- Produces: `keymap.ActionCommandPalette`가 사라진다. `ui.Deps.IntroPath`가 사라진다. `(a *App) showHistory()`는 `history.go`에 있다. 상태 바 힌트는 시작 문구만이고 `refreshHints`는 없다. 우클릭은 아무것도 하지 않는다.

- [ ] **Step 1: 히스토리 다이얼로그를 옮긴다**

`internal/ui/history.go`를 만들고 `palette.go` 812–856행의 `showHistory`와 `oneLineSQL`을 그대로 옮긴다.

```go
package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/Ahngbeom/datavase/internal/result"
)

// showHistory opens a searchable list of previously run statements.
func (a *App) showHistory() {
	if a.history == nil {
		a.notice("query history is unavailable")
		return
	}

	box := a.newSearchBox("search: ", " history ", pageHistory, func(term string) []searchItem {
		ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
		defer cancel()

		entries, err := a.history.Search(ctx, term, 100)
		if err != nil {
			return []searchItem{message("search failed", err.Error())}
		}
		if len(entries) == 0 {
			if term == "" {
				return []searchItem{nothingHere("nothing has been run yet",
					"statements are remembered once they finish")}
			}
			return []searchItem{noMatch("statement", term)}
		}

		items := make([]searchItem, len(entries))
		for i, e := range entries {
			entry := e
			items[i] = searchItem{
				primary: oneLineSQL(entry.SQL),
				secondary: fmt.Sprintf("%s · %d rows · %s",
					entry.At.Local().Format("2006-01-02 15:04"), entry.Rows, entry.DataSource),
				accept: func() {
					a.closeSearchBox(pageHistory)
					a.editor.SetText(entry.SQL, true)
				},
			}
		}
		return items
	})

	a.pages.AddPage(pageHistory, centred(box, 90, 24), true, true)
}

// oneLineSQL flattens a statement so each history entry occupies one row.
func oneLineSQL(sql string) string {
	return result.Truncate(strings.Join(strings.Fields(sql), " "), 110)
}
```

- [ ] **Step 2: 파일과 패키지를 지운다**

```sh
git rm -r -q internal/intro
git rm -q internal/ui/palette.go internal/ui/cmdline.go internal/ui/menu.go internal/ui/intro.go
git rm -q internal/ui/palette_test.go internal/ui/cmdline_test.go internal/ui/cmdline_integration_test.go \
  internal/ui/menu_test.go internal/ui/menu_integration_test.go internal/ui/intro_test.go \
  internal/ui/intro_integration_test.go internal/ui/discoverable_test.go internal/ui/discoverable_integration_test.go
```

- [ ] **Step 3: `internal/ui/app.go`를 고친다**

- 필드 `introPath`(200), `visitedRegions`(138), `hintBoundary`(146)를 지운다. `Deps.IntroPath`(287–290)와 `New()`의 `introPath:` 대입(324), `a.hintBoundary = ...`(337), `a.showIntroOnce()`(355)를 지운다.
- `dispatch()`의 `case keymap.ActionCommandPalette:`(1012)를 지운다. `ActionSearchHistory` case는 그대로 `a.showHistory()`를 부른다.
- `refreshHints()`(1074–1118)와 `hintsShown`(1122)을 지운다. `refreshHints()`를 부르던 곳(`mouse.go`의 `mouseAction`, `cycleFocus`, `mouseLeftDoubleClick` 등)에서 호출 줄만 지운다.
- `focusedContext()`(902–914)를 지운다.
- `openingClauses(a)`(380–401)에서 `paletteHint` 계산과 `paletteHint:`, `modal:`, `presetAssumed:`, `presetName:` 인자를 지운다. `opening` 구조체에서 `modal`, `paletteHint`, `presetAssumed`, `presetName` 필드와 `openingClauses(o)`의 해당 분기를 지운다. `advice`는 Task 7에서 지운다.
- 상수 `pagePalette`(841), `pageCommand`(848), `pageIntro`(851), `pageMenu`(852)를 지운다.

- [ ] **Step 4: `dialog.go`, `mouse.go`, `zone.go`, `status.go`를 고친다**

`dialog.go`:
- `startHere`에서 `keymap.ActionCommandPalette`를 `keymap.ActionSwitchDataSource`로 바꾼다.
- `helpText()`에서 `commandHelpText(...)`, `a.vimHelp()`, `a.modalEscapeHatch()` 호출과 `⌘ bindings need the terminal...` 블록(`dv keys`를 안내한다)을 지운다. `commandHelpText`(340–355)를 지운다. (`vimHelp`, `modalEscapeHatch`는 Task 6에서.)
- `helpGroups`의 "Finding things"에서 `keymap.ActionCommandPalette`를 지운다.

`mouse.go`: `mouseRightClick`(165–182)과 `bindMouse()`의 `case tview.MouseRightClick:`, `menuOpen()`(79–82)과 그것을 쓰는 더블클릭 재해석 블록(44–50)을 지운다. `mouseAction()`의 `case zoneStatusMode:` 블록을 지운다. `zone.go`에서 `zoneStatusMode`를 지운다.

`status.go`: `field.target` 주석에서 "mode" 언급을 지운다. `status.hints`는 남긴다(시작 문구).

- [ ] **Step 5: `keymap`과 `main`을 고친다**

`keymap/action.go`: `ActionCommandPalette`(59), `actionNames`(128), `descriptions`(179), `familiar`, `order` 항목. `keymap/map.go`: 248–250행 바인딩. `keymap/file_test.go`: `TestThePaletteHasAKeyNoHostCanClaim`, `TestPlainF2DoesNotDisturbCancel`을 지운다.

`cmd/dv/main.go`: `openUI`의 162–165행 `intro.DefaultPath()` 블록과 `Deps.IntroPath`를 지우고 import에서 `intro`를 지운다.

`internal/ui/app_integration_test.go`: `newHarnessWithIntro`를 지우고 `newHarness`가 `harnessWith(t, sess, ds, false)`를 직접 부르게 한다. `harnessWith`의 `introMarker string` 인자와 `IntroPath:` 대입을 지운다. `harness.runCommand()`(602행 근방)를 지운다.

- [ ] **Step 6: 빌드하고 나머지 참조를 지운다**

```sh
go build ./... && go vet ./... && go vet -tags integration ./...
```

기대: `dialog_test.go`(`TestEveryPaletteCommandAppearsOnTheHelpScreen`, `commandHelpText` 테스트), `searchbox_integration_test.go`(팔레트를 통한 검색 테스트 다수), `hints_integration_test.go`(영역 방문 힌트), `status_test.go`(`hints`를 팔레트 힌트로 채우는 케이스), `mouse_integration_test.go`(우클릭, 모드 존), `e0/e1/e4_integration_test.go`, `m6_integration_test.go`, `panel_integration_test.go`의 `runCommand` 사용처. 함수 단위로 지운다. `hints_integration_test.go`는 전부 `refreshHints`에 관한 것이면 파일째 지운다.

- [ ] **Step 7: 테스트와 lint, 커밋**

```sh
go test ./... && gofmt -l .
git add -A
git commit -m "Drop the palette, the command line, the context menu and the intro card

With a dozen keys and a help screen generated from them, a second way to
reach every command by name is a second thing to learn. The history
search moves out of the palette file and keeps its key."
```

---

### Task 6: vim 절단

**Files:**
- Delete: `internal/vim/`, `internal/ui/vimedit.go`, `internal/ui/vimmotion.go`, `internal/ui/vimnav.go`, `internal/ui/vimobject.go`, `internal/ui/vimrepeat.go`, `internal/ui/vimmotion_test.go`, `internal/ui/vimobject_test.go`, `internal/ui/e3_integration_test.go`, `internal/ui/preset_integration_test.go`
- Modify: `internal/ui/app.go`, `internal/ui/editor.go`, `internal/ui/tables.go`, `internal/ui/search.go`, `internal/ui/dialog.go`, `internal/ui/status.go`, `internal/ui/status_test.go`

**Interfaces:**
- Produces: `App`에서 `vim`, `register`, `registerLinewise`, `listPending`, `lastChange` 필드가 사라진다. `status.vimMode`/`vimPending`이 사라진다. `keymap.Map.Modal()`의 호출처가 UI에서 사라진다(메서드 자체는 Task 7에서 지운다).

- [ ] **Step 1: 파일과 패키지를 지운다**

```sh
git rm -r -q internal/vim
git rm -q internal/ui/vimedit.go internal/ui/vimmotion.go internal/ui/vimnav.go internal/ui/vimobject.go internal/ui/vimrepeat.go
git rm -q internal/ui/vimmotion_test.go internal/ui/vimobject_test.go internal/ui/e3_integration_test.go internal/ui/preset_integration_test.go
```

- [ ] **Step 2: 호출처를 고친다**

- `app.go`: 필드 `vim`(69), `register`(70), `registerLinewise`(71), `listPending`(73), `lastChange`(219)와 `New()`의 `vim: vim.New()`(326)를 지운다. `currentStatus()`(1519–1527)의 vim 분기(1520–1524)를 지운다. `editorPlaceholder()`(514–533)의 `a.keys.Modal()` 분기(527–531)를 지운다. import에서 `vim`을 지운다.
- `editor.go` 22–24행: `if a.keys.Modal() { return a.vimKey(ev) }` 블록을 지운다.
- `tables.go` 92–95행: `ev = a.vimListKey(ev); if ev == nil { return nil }`를 지운다.
- `search.go` 156, 223행: `a.vimSelecting()`을 `false`로 바꾼다. 그 결과 `moveCursor(fn, false)`가 되면, `moveCursor`의 두 번째 인자가 항상 false인지 확인하고 그렇다면 인자를 없앤다.
- `dialog.go`: `vimHelp()`(357–380), `modalEscapeHatch()`(382–395)를 지운다. import에서 `vim`을 지운다.
- `status.go`: `vimMode`, `vimPending` 필드와 렌더 256–265행을 지운다. `field.target`은 이제 어떤 필드도 발행하지 않으므로 `field.target` 필드와 `zoneTarget` 발행 경로(`renderWidth`에서 zone을 만드는 부분)를 지운다. 상태 바의 `record`(app.go 607행 `a.statusBar.record = a.hits.set`)도 지운다. `status_test.go`의 vim/zone 케이스를 지운다.

- [ ] **Step 3: 빌드하고 나머지 참조를 지운다**

```sh
go build ./... && go vet ./... && go vet -tags integration ./...
```

기대: `keymap.Modal()` 호출이 UI에서 남아 있으면 그 줄을 지운다. `motion_test.go`/`motion_integration_test.go`에 vim 모션 케이스가 섞여 있으면 케이스만 지운다. `edit_test.go`의 vim 참조 2곳도 케이스 단위로.

- [ ] **Step 4: 테스트와 lint, 커밋**

```sh
go test ./... && gofmt -l .
git add -A
git commit -m "Drop the modal editor

One keyboard, and typing types."
```

---

### Task 7: 키맵을 고정 맵 하나로; `dv init`·`dv keys`·위저드 삭제; `keymap` 설정은 무시하고 알린다

**Files:**
- Delete: `internal/keymap/preset.go`, `internal/keymap/preset_test.go`, `internal/keymap/snippet.go`, `internal/keymap/snippet_test.go`, `internal/config/keymap.go`, `internal/config/keymap_test.go`, `internal/cli/init.go`, `internal/cli/init_test.go`, `internal/cli/init_integration_test.go`, `internal/cli/keys.go`, `internal/cli/keydebug.go`, `internal/cli/keys_test.go`
- Modify: `internal/keymap/action.go`, `internal/keymap/action_test.go`, `internal/keymap/map.go`, `internal/keymap/binding.go`, `internal/keymap/display.go`, `internal/keymap/display_test.go`, `internal/keymap/keymap_test.go`, `internal/config/config.go`, `internal/config/config_test.go`, `internal/cli/cli.go`, `internal/cli/cli_test.go`, `cmd/dv/main.go`, `internal/ui/app.go`, `internal/ui/dialog.go`, `internal/ui/dialog_test.go`, `internal/ui/app_integration_test.go`

**Interfaces:**
- Produces: `keymap.Default() *Map`이 유일한 생성자다. `Map.Lookup`, `Map.Bindings`, `Map.DisplayBindings`, `Binding.Label`, `PadLabel`, `AllActions`, `Action.String/Describe/Reserved`만 공개 API로 남는다. `config.Config.Ignored() []string`이 무시된 최상위 키를 돌려준다. `ui.Deps.Keys`, `Deps.PresetAssumed`가 사라진다(맵은 항상 `keymap.Default()`).

- [ ] **Step 1: 무시되는 `keymap` 키의 실패하는 테스트를 쓴다**

`internal/config/config_test.go`에 추가한다.

```go
func TestAnOlderKeymapBlockIsIgnoredRatherThanRefused(t *testing.T) {
	cfg, err := Parse(strings.NewReader(`
datasources:
  - name: local
    host: 127.0.0.1
    user: root
keymap:
  preset: vim
  actions:
    run: ["f5"]
`))
	if err != nil {
		t.Fatalf("Parse() error = %v; a v0.8 config must still open", err)
	}
	if got := cfg.Ignored(); len(got) != 1 || got[0] != "keymap" {
		t.Errorf("Ignored() = %v, want [keymap]", got)
	}
}

func TestAConfigWithNoKeymapIgnoresNothing(t *testing.T) {
	cfg, err := Parse(strings.NewReader("datasources:\n  - name: local\n    host: h\n    user: u\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Ignored(); len(got) != 0 {
		t.Errorf("Ignored() = %v, want none", got)
	}
}
```

- [ ] **Step 2: 실패를 확인한다**

```sh
go test ./internal/config/ -run 'Ignored|OlderKeymap'
```

기대: `cfg.Ignored undefined`로 빌드 실패.

- [ ] **Step 3: `config`를 고친다**

```sh
git rm -q internal/config/keymap.go internal/config/keymap_test.go
```

`internal/config/config.go`의 `Config`를 다음으로 바꾼다.

```go
// Config is the root of the configuration file.
type Config struct {
	DataSources []DataSource `yaml:"datasources"`
	Defaults    Defaults     `yaml:"defaults"`

	// Keymap is read and discarded. Earlier versions wrote it, and refusing
	// the whole file over a key that no longer does anything would lock out
	// exactly the people upgrading.
	Keymap map[string]any `yaml:"keymap,omitempty"`

	ignored []string
}

// Ignored names the top-level keys that were present and did nothing, so
// the caller can say so once rather than leave a setting silently dead.
func (c *Config) Ignored() []string { return c.ignored }
```

`Parse()`에서 `cfg.applyDefaults()` 앞에 추가한다.

```go
	if cfg.Keymap != nil {
		cfg.ignored = append(cfg.ignored, "keymap")
		cfg.Keymap = nil
	}
```

- [ ] **Step 4: 통과를 확인한다**

```sh
go test ./internal/config/
```

기대: 전부 PASS.

- [ ] **Step 5: `keymap`을 고정 맵 하나로 줄인다**

```sh
git rm -q internal/keymap/preset.go internal/keymap/preset_test.go internal/keymap/snippet.go internal/keymap/snippet_test.go
```

`internal/keymap/map.go`:
- `Map` 구조체에서 `preset Preset` 필드를 지운다.
- `Default()`(143–151)를 다음으로 바꾼다.

```go
// Default is the key map. There is one.
func Default() *Map {
	return baseMap()
}
```

- `Apply()`(46–79), `clear()`(84–89), `sortedActionNames()`(91–98)를 지운다.

`internal/keymap/binding.go`: `ParseBinding`(141–189), `modifierNames`(100), `namedKeys`(113), `functionKeys`(134)를 지운다. `keyAliases`가 `ParseBinding`에서만 쓰이면 함께 지운다. `Label`, `keyLabel`, `keyLabels`, `normalize`, `sortBindings`, `Event`는 남긴다.

`internal/keymap/action.go`:
- `familiar` 맵(216–268)과 `Action.Familiar()`(271)를 지운다.
- `actionByName`(314–320)을 지운다.
- `order`는 남은 액션만 담는지 확인한다(Task 1–5에서 이미 정리됨).

`internal/keymap/display.go`: `SplitByFamiliarity`(65–74), `PackFamiliar`(82–108), `LabelWidth`가 `PadLabel`에서만 쓰이면 비공개로 바꾸거나 그대로 둔다. `spaces`는 `PadLabel`이 쓰면 남긴다.

테스트:
- `action_test.go`: `TestEveryActionIsClassified`, `TestTheKeyReferenceDoesNotSilentlyStartTeachingTheWrongList`, `TestTheJudgementCallsHold`를 지운다. 남는 것이 없으면 파일을 지운다.
- `display_test.go`: `TestTerminalAdviceNamesTheControlBinding`, `TestTheSplitKeepsEveryActionExactlyOnce`, `TestTheSplitPreservesTheOrderItWasGiven`을 지운다.
- `keymap_test.go`: `TestApplyOverridesReplacesBindings`, `TestApplyRejectsUnknownActionNames`, `TestApplyRejectsUnparsableBindings`, `TestApplyIsAtomic`, `TestParseBinding`, `TestParseBindingRejectsNonsense`, `TestParsedBindingsRoundTripThroughLookup`을 지운다.

- [ ] **Step 6: `ui`에서 프리셋·터미널 조언·키 오버라이드를 지운다**

`internal/ui/app.go`:
- 필드 `presetAssumed`(92)와 `Deps.Keys`(270), `Deps.PresetAssumed`(291–294)를 지운다. `New()`에서 `keys := deps.Keys; if keys == nil {...}` 블록을 `keys := keymap.Default()`로 바꾸고 `presetAssumed:` 대입을 지운다.
- `openingClauses(a)`에서 `term := os.Getenv("TERM")`, `advice: keymap.TerminalAdviceShort(term, a.keys)` 인자를 지우고, `opening.advice` 필드와 `openingClauses(o)`의 `if o.advice != ""` 분기를 지운다. 남는 인자는 `serverVersion`, `helpKey`, `sidebarKey`다. 이 함수 위의 긴 주석은 남은 세 절의 순서 이유만 남기고 줄인다.
- `keymap.SupportsExtendedKeys`를 쓰는 곳(506, 521행 근방)을 지운다. 그 결과 `os` import가 안 쓰이면 지운다.

`internal/ui/dialog.go`:
- `helpText()`에서 `keymap.TerminalAdvice(...)` 블록을 지우고, 그 자리에 고정 한 줄을 넣는다.

```go
	b.WriteString("\n" + tag(colourMuted, "If ⌘↩ or Ctrl+↩ does nothing here, F5 runs: some terminals keep modified keys.") + "\n")
```

- `helpReference()`를 그룹을 그대로 출력하는 형태로 바꾼다.

```go
func helpReference(km *keymap.Map) string {
	var b strings.Builder
	for _, group := range helpGroups {
		fmt.Fprintf(&b, "\n%s\n", headingTag(group.title))
		for _, action := range group.actions {
			b.WriteString(keyReferenceLine(km, action))
		}
	}
	return b.String()
}
```

- `familiarWidth` 상수를 지운다.

`internal/ui/dialog_test.go`: `TestTheHelpScreenPacksTheKeysAReaderAlreadyKnows`, `TestTheFamiliarBlockFitsTheDialogAtEightyColumns`, `TestAGroupWithNothingLeftToTeachLeavesNoHeading`을 지운다. `TestEveryActionAppearsOnTheHelpScreen`과 `TestEveryActionIsStillOnTheRenderedHelpScreen`은 남긴다.

`internal/ui/app_integration_test.go`의 `harnessWith`: `keymap.ForPreset(...)` 블록과 `Keys: keys`, `PresetAssumed:` 대입을 지운다. `newHarnessAssumingPreset`을 지우고 `harnessWith`의 `presetAssumed bool` 인자를 없앤다.

- [ ] **Step 7: `cli`와 `main`에서 `dv init`, `dv keys`, 위저드를 지운다**

```sh
git rm -q internal/cli/init.go internal/cli/init_test.go internal/cli/init_integration_test.go \
  internal/cli/keys.go internal/cli/keydebug.go internal/cli/keys_test.go
```

`internal/cli/cli.go`:
- `Run()`에서 `case "keys":`를 지운다.
- `usage()`를 다음으로 바꾼다.

```go
func (a *App) usage() {
	fmt.Fprint(a.Out, `datavase — terminal MySQL client

usage:
  dv [open <name>]      open the interface
  dv ls                 list configured datasources
  dv auth <name>        store a datasource password in the keychain
  dv auth -rm <name>    remove a stored password
  dv check <name>       verify that the datasource is reachable
  dv version            print the version
  dv help               show this message
`)
}
```

- `open()`에서 데이터소스가 0개인 경우를 먼저 다룬다(211–220행의 분기 앞에).

```go
	if name == "" && len(a.Config.DataSources) == 0 {
		fmt.Fprintln(a.Err, "no datasources are configured; add one to the config file")
		return exitUsage
	}
```

(Part 2에서 이 자리가 데이터소스 다이얼로그로 바뀐다.)

`cmd/dv/main.go`:
- `run()`의 63–68행 `cli.HandleInit` 블록과 81–91행의 위저드 분기를 지운다. 설정 파일이 없을 때는 다음 두 줄로 종료한다.

```go
	if errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "no configuration at %s\n", path)
		return 1
	}
```

- 설정을 읽은 직후에 무시된 키를 알린다.

```go
	for _, key := range cfg.Ignored() {
		fmt.Fprintf(os.Stderr, "config: %q is no longer used and was ignored\n", key)
	}
```

- `openUI`에서 `keymap.FromConfig` 블록(125–128)과 `Deps.Keys`, `Deps.PresetAssumed`를 지운다. import에서 `keymap`을 지운다.
- 464–575행의 `runWizard`, `stdin`, `ask`, `choose`, `printGettingStarted`를 지운다. **`readPassword`(449–462)는 `dv auth`가 쓰므로 남긴다.**

`internal/cli/cli_test.go`: `TestUsageMentionsTheVersionCommand`(`version_test.go`)가 새 usage에서도 통과하는지 확인한다. `HandleInit`·`keys`를 참조하는 테스트를 지운다.

- [ ] **Step 8: 빌드하고 나머지 참조를 지운다**

```sh
go build ./... && go vet ./... && go vet -tags integration ./...
```

기대: `status_test.go`의 `TestTerminalAdviceOutranksAContextHint`, `e4_integration_test.go`의 assumed-preset 테스트, `app_integration_test.go`의 `keys.Lookup(binding.Event())` 검증(남겨도 되지만 `Bindings()`가 남아 있으므로 통과해야 한다), `emptystate_*`의 modal placeholder 케이스. 함수·케이스 단위로 지운다.

- [ ] **Step 9: 테스트와 lint**

```sh
go test ./... && gofmt -l .
```

MariaDB가 있으면:

```sh
make db-up && go test -tags integration ./... ; make db-down
```

기대: 전부 PASS.

- [ ] **Step 10: 남은 죽은 코드를 훑는다**

```sh
rg -n 'palette|Palette|vim|worktree|guard|explain|procs|detach|preset|intro' --glob '*.go' internal cmd | grep -v _test | head
```

기대: 남는 것은 `intro`가 포함된 무관한 단어(`introduce` 등)나 `guard`가 포함된 무관한 식별자뿐이다. 기능을 가리키는 주석이 남아 있으면 지운다.

- [ ] **Step 11: 커밋**

```sh
git add -A
git commit -m "One key map, no presets, no wizard, no dv keys

keymap.Default is the map; configuration cannot change it, and a keymap
block left over from an earlier version is read, ignored and mentioned
once. dv init goes with the wizard: the datasource dialog that replaces
it is the next plan."
```

---

## Part 1 완료 조건

- `go build ./...`, `go vet -tags integration ./...`, `go test ./...`, `gofmt -l .`가 초록.
- `internal/` 아래에 `daemon`, `attach`, `proto`, `snapshot`, `screen`, `worktree`, `recent`, `explain`, `procs`, `guard`, `vim`, `intro` 디렉터리가 없다.
- `dv`를 v0.8.0의 `config.yaml`로 실행하면 `config: "keymap" is no longer used and was ignored` 한 줄이 나오고 인터페이스가 뜬다.
- `F1`의 도움말이 남은 액션을 모두 보이고, `dialog_test.go`의 exactly-once 테스트가 통과한다.
- 프로덕션 코드 줄 수를 기록한다: `find internal cmd -name '*.go' ! -name '*_test.go' | xargs cat | wc -l`.

# 초단순 버전 (simple mode)

**Status:** design approved, not yet implemented
**Date:** 2026-09-08

## 목표

`dv`를 네 가지 일만 하는 도구로 줄인다.

1. 데이터소스를 여러 개 저장하고, 앱 안에서 추가·수정·삭제·전환한다.
2. 마우스로 테이블을 골라 `LIMIT 100` 미리보기를 본다.
3. `⌘↩`(`Ctrl+↩`)로 편집기의 문장을 실행한다.
4. 결과를 격자로 보고, 전체를 Markdown 또는 JSON으로 클립보드에 복사한다.

이 네 가지를 위해 필요한 것(SSH 터널, 스키마 자동완성, 쿼리 히스토리,
키체인, TLS, auto LIMIT)은 남기고, 나머지는 저장소에서 지운다. 같은
바이너리 안의 "모드"가 아니라 코드 삭제다. 되돌리려면 `v0.8.x` 태그로 간다.

## 왜 삭제인가

v0.8.0은 프로덕션 코드 약 24,500줄, 30개 패키지다. 그중 위 네 가지와 무관한
것이 절반이 넘는다. 데몬/attach 세션 유지, worktree, explain, procs, vim
모달 편집기, 프로덕션 가드, 팔레트와 커맨드라인, 키맵 프리셋과 오버라이드.

플래그로 숨기면 코드는 그대로이고 분기만 늘어난다. 별도 바이너리를 만들면
UI에서 이미 겪고 고친 함정(격자 히트맵, OSC 52, `TextArea` 캐럿 타이밍)을
다시 밟는다. 그래서 **패키지와 파일은 통째로 지우고, 모든 기능이 만나는
배선(`internal/ui/app.go`, `cmd/dv/main.go`)만 줄어든 의존성에 맞춰 다시
쓴다.** 검증된 위젯 파일과 UI 테스트 하네스는 유지한다.

## 범위

**남기는 것.**

- 패키지: `db`, `session`, `tunnel`, `result`, `export`, `sqlparse`,
  `catalog`, `complete`, `history`, `secret`, `match`, `version`, `config`,
  `cli`, `keymap`(축소), `ui`(축소), `testmysql`, `testssh`.
- CLI: `dv`, `dv open <name>`, `dv ls`, `dv auth`, `dv check`, `dv version`,
  `dv help`.
- 편집기 편의 기능: 단어·줄 이동, 주석 토글, 줄 복제·삭제, 찾기, 자동완성.
  이미 있고 유지 비용이 없다.
- 결과: 스트리밍, 정렬, 행 자세히 보기, 셀·행 복사, 화면 검색.
- 트랜잭션 제어(`BEGIN`/`COMMIT`/`ROLLBACK`을 타이핑해서 실행).

**지우는 것.**

- 패키지: `daemon`, `attach`, `proto`, `snapshot`, `screen`, `worktree`,
  `recent`, `explain`, `procs`, `vim`, `guard`, `intro`.
- `cmd/dv/runtime.go` 전체. `main.go`의 세션 어댑터, 위저드, `server`·
  `status`·`api`·`keys`·`init` 서브커맨드, `--dir`.
- `internal/ui`: `vimedit.go`, `vimmotion.go`, `vimnav.go`, `vimobject.go`,
  `vimrepeat.go`, `palette.go`, `cmdline.go`, `menu.go`, `worktree.go`,
  `files.go`, `savefile.go`, `explain.go`, `sessions.go`, `ddl.go`,
  `intro.go`, `goto.go`. 각각의 테스트 파일도 함께.
- `keymap`: `snippet.go`, 프리셋 3종, `FromConfig`·`Apply` 오버라이드,
  `Presets()`, `dv keys`가 쓰던 출력. 액션은 47개에서 아래 표의 것만 남는다.
- `config`: `keymap` 블록과 `Env`의 가드 의미. `Mouse` 설정은 남긴다.
- 프로덕션 가드의 `Deny`/`Confirm`/`unlock writes` 흐름 전부.
- 첫 실행 카드(`intro`)와 `startHere`.

**하지 않는 것.**

- 격자 편집, 생성된 `UPDATE`. 이전과 같다.
- 미리보기 행 수 설정. 100은 상수다.
- 키 재배치. 키맵은 고정이다.
- 재접속. 이전과 같다.

## 화면과 키

화면 구성은 지금과 같다. 왼쪽에 스키마 트리와 테이블 탭, 오른쪽 위 편집기,
오른쪽 아래 결과 격자, 맨 아래 상태 바. 팔레트, 커맨드라인, 모드 표시는 없다.

| 동작 | 키 | 마우스 |
|---|---|---|
| 커서 아래 문장 실행 | `⌘↩` · `Ctrl+↩` · `F5` | |
| 전부 실행 | `⌘⇧↩` · `Ctrl+Shift+↩` · `Shift+F5` | |
| 취소 | `⌘F2`, 실행 중이면 `⌘C` | |
| 테이블 미리보기 (`LIMIT 100`) | 트리·테이블 탭에서 `↩` | 트리 더블클릭, 테이블 탭 클릭 |
| 셀 복사 / 행 복사 | `⌘C` | |
| 결과 전체 복사 (Markdown / JSON) | `⌘⇧C` | 결과 탭 헤더의 `copy` |
| 데이터소스 목록 | `⌘⇧D` · `F11` | 상단 바의 데이터소스 이름 |
| 스키마 선택 | `⌘⇧N` · `F7` | 상단 바의 스키마 이름 |
| 트리 새로고침 / 사이드바 토글 | `⌘R` / `⌘B` | |
| 창 이동 | `⇥` / `⇧⇥` | 창 이름 클릭 |
| 탭 전환 | `Ctrl+⇥` · `F6` | 탭 클릭 |
| 자동완성 | `^Space` | |
| 찾기 / 다음 / 이전 | `⌘F` / `⌘G` / `⌘⇧G` | |
| 히스토리 검색 | `⌘⇧F` · `F9` | |
| 행 자세히 보기 | `⌘I` · `F4` | 격자 더블클릭 |
| 정렬 | `⌘⇧S` · `F12` | 헤더 클릭 |
| 도움말 | `F1` | 상단 바의 `?` |
| 종료 | `⌘Q` · `F10` | |

`⌘`과 `Ctrl`은 같은 뜻이고, 커서 이동만 macOS 관례를 따른다(지금과 같다).
편집기의 잘라내기·붙여넣기·전체 선택·단어 이동·주석·줄 복제·줄 삭제는
지금의 바인딩 그대로다. F키 대체는 남긴다. 확장 키보드 프로토콜이 없는
터미널에서 `⌘↩`가 닿지 않는 문제는 여전하고, 그때의 답은 `F5`다.

`⌘⇧C`는 v0.8.0에서 어떤 액션에도 매여 있지 않다.

## 데이터 흐름

### 실행

```
editor text ─ sqlparse.StatementAt ─▶ Statement
                                        │
                        sqlparse.AutoLimit(stmt, cfg.Defaults.AutoLimit)
                                        │
                              db.Conn.Query(ctx, sql, Options) ─▶ *Stream
                                        │
                              consume → result.Buffer → grid
```

`guard.Evaluate`가 하던 일 중 남는 것은 "무제한 SELECT에 LIMIT을 붙인다"
하나다. `guard.Policy.autoLimitFor`를 `sqlparse.AutoLimit(stmt, n) int`로
옮긴다. `HasTopLevelLimit`와 `AppendLimit`이 이미 `sqlparse`에 있으므로
자연스러운 자리이고, `guard`는 통째로 사라진다. `ui`의 `runStatement`는
Deny/Confirm 분기 없이 `start`로 직행하고, 상태 바의 `limitInjected`
표시는 유지한다.

### 미리보기

트리의 테이블 노드를 더블클릭하거나 `↩`를 누르면, 또는 테이블 탭의 항목을
클릭하거나 `↩`를 누르면:

```
SELECT * FROM `schema`.`table` LIMIT 100
```

을 **편집기를 건드리지 않고** 실행 경로에 넣는다. 편집기에 있는 내용은
사용자의 것이고, 미리보기가 그것을 덮어쓰면 클릭 한 번으로 작업을 잃는다.
실행한 SQL은 상태 바에 보인다. 이미 실행 중이면 지금의 "a statement is
already running" 알림과 같다. 식별자는 `sqlparse.QuoteIdentifier`로 감싼다.

트리의 단일 클릭과 `↩`의 의미는 나뉜다. 단일 클릭은 지금처럼 컬럼을
펼치고, `↩`는 미리보기다. 컬럼 노드의 `↩`는 지금처럼 이름을 편집기에
넣는다.

### 결과 전체 복사

```
⌘⇧C ─▶ chooser(Markdown | JSON) ─▶ export.{Markdown,JSON}(buf) ─▶ OSC 52
```

원본은 화면이 아니라 `result.Buffer`다. 격자는 긴 값을 자르고 마크업용
대괄호를 두 배로 만들기 때문이다(`gridcopy.go`가 같은 이유를 든다).
정렬이 걸려 있으면 표시 순서(`content.bufferRow`)를 따른다. 버퍼가
`buffer_max`에서 잘렸으면 알림에 그 사실을 함께 말한다.

`export.Markdown(w, columns, rows)`를 추가한다. 헤더 행, 구분 행, 값 행.
값 안의 `|`는 `\|`로, 개행은 `<br>`로 바꾼다. NULL은 빈 칸이다. 컬럼 이름이
겹치면 JSON은 지금처럼 `uniqueNames`로 구별하고, Markdown은 헤더에 그대로
쓴다(표는 키가 아니라 위치로 읽는다).

OSC 52는 터미널마다 받아주는 크기가 다르다(tmux 기본은 작다). 잘려도
경고가 없으므로, 복사 알림에 행 수와 바이트 수를 함께 적는다:
`1,000 rows copied as Markdown (212 KB)`.

### 데이터소스 관리

두 화면이다.

**목록** — 이름, host, user, 터널 여부. `↩`/클릭은 연결, `a` 추가, `e`
수정, `d` 삭제(확인 후), `Esc` 닫기. 현재 연결된 것은 표시한다.

**폼** — `tview.Form`. name, host, port, user, password, database, tls
(드롭다운: preferred · required · verify-ca · verify-identity · disabled),
tls_ca, tunnel host / port / user / identity. 터널 host를 비우면 터널은
없다. 버튼은 `Test`, `Save`, `Cancel`.

- `Test`는 `dv check`가 쓰는 probe를 재사용해 서버 버전을 보여준다.
- `Save`는 이름 중복과 빈 host·user를 폼 안에서 거부한다. 통과하면
  `config.Save`로 파일을 쓰고, password가 비어 있지 않으면
  `secret.Store.Set`으로 키체인에 넣는다. 파일에는 절대 쓰지 않는다.
- 수정 폼의 password는 비워 두면 "바꾸지 않음"이다.
- 삭제는 파일에서 항목을 빼고, 키체인의 비밀번호도 지운다.

`config.Save(path string, c *Config) error`를 추가한다. 같은 디렉터리에
임시 파일을 0600으로 쓰고 `rename`한다. **파일의 주석은 저장 시 사라진다.**
`dv`가 파일을 소유하며, 손으로 편집한 파일도 계속 읽힌다. README에 적는다.

`config` 패키지는 지금까지 읽기 전용이었고 유일한 쓰기 경로는 `cli.Wizard`
였다. 이제 `config.Save`가 유일한 쓰기 경로이고, 위저드는 없다.

### 설정 없는 첫 실행

`dv`를 설정 파일 없이 실행하면 UI가 뜨고 곧바로 데이터소스 목록(비어
있음)이 열린다. 이를 위해 `App`이 세션 없이 시작할 수 있어야 한다.
편집기와 트리는 비활성, 상태 바는 `no datasource`, 실행 키는 "connect
first" 알림. 연결이 되면 지금의 `adopt(sess)` 경로로 합류한다.

`dv open <name>`은 지금처럼 바로 연결하고, 실패하면 종료 코드로 말한다.

### 설정 파일 호환

`config.Parse`는 모르는 키를 거부한다. 지우는 키가 그대로 거부되면 v0.8.x
설정 파일은 전부 열리지 않는다. 그래서:

- `keymap` 블록은 파싱하되 무시하고, 시작 시 한 줄로 알린다.
- `env`는 파싱하되 **TLS 기본값을 정하는 데만 쓴다** (`prod` →
  `required`, 나머지 → `preferred`). 지우면 `env: prod`인 기존 설정이
  조용히 평문을 허용하는 쪽으로 내려간다. 가드 정책으로는 더 이상 쓰지
  않고, 새 폼은 `env`를 쓰지 않는다. CHANGELOG에 "`tls:`를 명시하라"고
  적는다.

## 패키지 경계

- `sqlparse`는 여전히 토크나이저다. `AutoLimit`은 `HasTopLevelLimit`의
  결과에 숫자를 붙이는 것뿐이고, 정책을 알지 못한다.
- `export`는 여전히 `io.Writer`와 `[]string`, `[][]any`만 안다. `result`나
  `ui`를 import하지 않는다.
- `config`는 여전히 `io.Reader`로 파싱하고, `Save`는 `io.Writer`로
  직렬화하는 `Write(w, c)`와 파일을 원자적으로 바꾸는 `Save(path, c)`로
  나뉜다. 전자는 파일 없이 테스트한다.
- `ui`는 여전히 tview를 아는 유일한 패키지다. 세션 없는 시작은 `App`의
  상태이지 새 타입이 아니다.
- `keymap.Default()`가 유일한 맵이다. `FromConfig`는 없다.

## 테스트

지우는 파일의 테스트는 함께 지운다. `app_integration_test.go`의 하네스와
`h.do(action)`·`h.waitFor` 방식은 유지한다.

새로 쓰는 테스트는 실패를 먼저 본다.

- `sqlparse.AutoLimit`: 이미 LIMIT이 있으면 0, 서브쿼리의 LIMIT은 무시,
  `n <= 0`이면 0, SELECT가 아니면 0.
- `export.Markdown`: `|`·개행 이스케이프, NULL 빈 칸, 컬럼 0개, 행 0개.
- `config.Write`/`Save`: 라운드트립, 임시 파일 후 rename, 실패 시 원본
  보존, 0600, `keymap`·`env` 호환 읽기.
- 폼 검증: 이름 중복, 빈 host, 터널 host 없이 터널 user만 있는 경우.
- 미리보기: 생성 SQL, 실행 중 거부, 편집기 내용 불변.
- 결과 전체 복사: 정렬 순서 반영, 잘린 버퍼 알림, 빈 결과.
- 설정 없는 시작: 목록이 열리고, 실행 키가 "connect first"를 말한다.
- 유지하는 자동 검사: 모든 액션이 `helpGroups`에 정확히 한 번.

통합 테스트는 `make test-integration`(MariaDB, 13306) 그대로.

## 단계

각 단계 끝에 `make build`, `make test`, `make lint`가 초록이다.

1. 런타임 절단. `daemon`·`attach`·`proto`·`snapshot`·`screen`,
   `runtime.go`, `Detach` 액션, `server`·`status`·`api` 서브커맨드.
   `dv`가 프로세스 안에서 UI를 직접 띄운다.
2. 주변 기능 절단. `worktree`·`recent`·`explain`·`procs`와 `ui`의
   해당 파일, 액션, `--dir`.
3. 입력 계층 절단. `vim`·`intro`, `ui`의 palette/cmdline/menu/intro/goto,
   `keymap` 단일 맵화, `dv keys`·`dv init` 제거, `config.keymap` 무시.
4. `guard` → `sqlparse.AutoLimit`. `env` 축소.
5. `export.Markdown`과 결과 전체 복사.
6. 테이블 미리보기.
7. `config.Save`, 데이터소스 목록·폼, 설정 없는 첫 실행.
8. 문서. README 전면 개정, `CLAUDE.md`의 아키텍처 절, CHANGELOG,
   `goreleaser check`.

삭제를 먼저 하는 이유: 새 기능이 단순해진 `app.go` 위에 얹히게 하려는
것이다. 반대로 하면 새 기능이 곧 지울 팔레트·가드에 배선된다.

## 끝났을 때 참이어야 하는 것

- `dv`를 설정 없이 실행하면 데이터소스 폼이 뜨고, 저장하면 연결된다.
- 트리에서 테이블을 더블클릭하면 100행이 격자에 보이고 편집기는 그대로다.
- `⌘↩`가 커서 아래 문장을 실행하고, 무제한 SELECT에는 LIMIT이 붙는다.
- `⌘⇧C` → Markdown이 클립보드에 있고, 붙여넣으면 표로 렌더링된다.
- v0.8.0의 `config.yaml`(`env`, `keymap` 포함)이 그대로 열린다.
- `go build ./...`의 결과에 `guard`, `vim`, `daemon`이 없다.
- 프로덕션 코드가 10,000줄 아래다.

## 크기와 릴리스

24,500줄 → 약 9,000줄. 테스트는 27,000줄 → 약 10,000줄.

`v0.9.0`으로 낸다. CHANGELOG에 "v0.8.x가 전체 기능판의 마지막"임을 적어
Homebrew 사용자가 업그레이드 전에 알 수 있게 한다.

## 위험과 미룬 결정

- **OSC 52 크기 한도.** 큰 결과가 조용히 잘릴 수 있다. 알림에 크기를
  적는 것으로 시작하고, 파일로 내보내기가 필요해지면 그때 `export`의
  파일 경로를 쓴다.
- **저장 시 주석 소실.** `config.Save`는 YAML을 다시 직렬화한다. 주석을
  보존하려면 `yaml.Node` 수준 편집이 필요한데, 첫 버전에서는 하지 않는다.
- **`env` 잔존.** TLS 기본값 때문에 한 릴리스 남긴다. v0.10에서 지운다.
- **세션 없는 `App`.** 지금의 `App`은 `conn != nil`을 전제한다. 7단계에서
  그 전제가 닿는 곳을 모두 찾아야 하며, 테스트 하네스가 세션 있는 시작만
  지원하면 그것도 넓힌다.

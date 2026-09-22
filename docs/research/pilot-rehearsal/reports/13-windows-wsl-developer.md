# dv 파일럿 사전 QA 리허설 — Interview 13 페르소나 (박도윤, Windows/WSL Backend Developer)

**Synthetic. 실제 파일럿 참가자의 경험이 아니며, issue #102 게이트 숫자에 반영되지 않는다.**

페르소나: 박도윤, 32세, B2B SaaS Backend Developer 7년차. 회사 지급 Windows 11 노트북에서
Windows Terminal + WSL2 Ubuntu로 일하고, 장애 대응은 WSL의 `ssh`/`kubectl`/`mysql`로,
정기 분석은 Windows의 DataGrip으로 한다. "Windows 지원"이라는 한 줄보다 실제 검증된
shell·clipboard·credential store·설치 경로를 요구하는 사람.

## Stop-condition 관찰: 없음

(a) 화면/내보낸 값이 틀림 — 관찰 안 됨. (b) read-only 세션이 아닌데 read-only로 표시되거나
잘못된 datasource/schema 표시 — 관찰 안 됨. (c) credential 노출 — 관찰 안 됨. 아래 "쓰기
시도"와 "관찰" 항목에 근거 있음.

## 설치~첫 쿼리

README의 "Other ways in → From a clone: make build" 경로로 이미 빌드된 바이너리를 받았다고
가정. `dv version` → `dv (devel)`, sha256 확인. README의 Install/First run/Configure 섹션만
읽고 진행.

- README를 열람함(설치, First run, Configure, Keys, The screen 섹션 전체).
- 이 페르소나는 인터뷰에서 "결과가 어디에 저장됐는지 봐야 한다"며 interactive 등록보다 파일을
  선호한다고 밝혔으므로, `config.yaml`을 손으로 작성(`$XDG_CONFIG_HOME/datavase/config.yaml`,
  `read_only: true`, `tls: disabled` — 로컬 컨테이너용).
- `dv auth pilot`을 파이프로 시도 → **`reading password: stdin is not a terminal; run dv auth
  interactively`** — TTY가 아니면 즉시, 명확한 이유와 함께 거부됨(합리적인 동작). 이 페르소나가
  실제로 자동화·헤드리스 세션에서 우려했던 지점과 정확히 일치하며, README가 이미 안내하는
  `DATAVASE_PASSWORD_<NAME>` 경로로 전환.
- `export DATAVASE_PASSWORD_PILOT=...` → `dv check pilot` → `pilot is reachable — server
  11.4.12-MariaDB-ubu2404`. 환경변수가 키체인보다 우선한다는 README의 서술과 일치.
- 소요 시간: README 정독 + config 작성 + `dv check` 성공까지 체감 10분 이내. (본 리허설
  자체는 TUI를 tmux로 원격 조작하느라 더 걸렸지만, 이는 리허설 방법의 마찰이지 페르소나가
  겪을 마찰이 아님 — "한계" 항목에 명시.)

## 실행한 쿼리와 관찰

tmux(-L 별도 소켓) 안에서 `dv open pilot`을 띄우고 `F5`로 실행(`Super+↩` 라벨은 아래
"Windows/WSL 문서 격차" 참고).

1. `SELECT id, email, created_at FROM customers ORDER BY id LIMIT 5;` → 4 rows · 13ms, 값
   정상.
2. `SELECT customer_id, ...` — 존재하지 않는 컬럼명으로 오타 → `Error 1054 (42S22): Unknown
   column 'customer_id' in 'SELECT'`. 서버 오류가 그대로, 명확하게 전달됨.
3. `SELECT * FROM orders LIMIT 3;` → 실제 컬럼(`id, customer, total, status, placed_at`) 확인.
4. `SELECT status, COUNT(*) AS n, SUM(total) AS total FROM orders GROUP BY status ORDER BY n
   DESC;` → `paid 3 / 1145.75`, `shipped 2 / 385.00`, `refunded 1 / 19.99` — 3 rows · 7ms ·
   `LIMIT 1000 added`. `auto_limit`가 GROUP BY에도 정확히 적용됨.
5. 결과 grid로 `Tab`, `Ctrl+C`로 셀 복사(id=1) → 상태줄에 `value copied`, 로컬 clipboard에
   실제로 `1`이 들어감(macOS `pbcopy` 경로 확인). 정확한 동작.

## 쓰기 시도 결과

`UPDATE orders SET status = 'cancelled' WHERE id = 1;` → **`Error 1142 (42000): UPDATE command
denied to user 'pilot_ro'@'192.168.65.1' for table `shop`.`orders``**. 곧바로
`SELECT id, status FROM orders WHERE id = 1;`로 재확인 → `1, shipped` — 실제로 적용되지
않았음을 검증.

관찰: 이 거부는 MySQL 자체의 GRANT 거부(1142)이며, dv가 "read_only 설정이 막았다"고 직접
이름 붙이는 문장은 화면에 없었다. README의 "A refused statement says which setting refused
it"이라는 서술은 이 케이스(서버 grant 레벨 거부)에는 적용되지 않는 것으로 보인다 — dv의
세션 read-only 확인이 만드는 거부(`SET SESSION TRANSACTION READ ONLY` 위반, 다른 에러
번호가 될 것으로 예상)와는 다른 경로다. 결과 자체는 안전했지만(쓰기가 막혔고 그렇게
보였다), 문구의 약속과 실제 화면 사이에 미세한 차이가 있다.

## 5줄 다이어리 (1주차, 도윤 말투)

1. 이번 주에 dv를 쓴 날: 1일 — 오늘 이 사전 점검에서만 썼습니다.
2. dv 대신 mysql이나 GUI로 돌아간 순간: 아직은 없습니다. 다만 이건 제 실제 WSL 환경이
   아니라 macOS 셸에서 대신 시험한 것이라, WSL이었다면 clipboard가 실제로 Windows
   쪽까지 갔는지 확인하는 순간에 멈췄을 가능성이 큽니다.
3. 화면이 틀린 것을 보여준 적: 없습니다. read-only 표시, 계정·서버 표시, 값 모두 실제와
   일치했습니다.
4. 막혔는데 물어보지 않고 넘어간 것: `dv auth`가 TTY 아니면 거부된다는 걸 알고
   `DATAVASE_PASSWORD_*`로 바로 넘어갔습니다 — 이건 README에 답이 있어서 넘어간 거지 막힌
   채 방치한 건 아닙니다. 다만 CSV를 저장했을 때 어느 절대 경로에 쓰였는지 화면에 안 나와서,
   실제였다면 그걸 찾느라 한 번 멈췄을 겁니다(아래 참고).
5. 해당 없음(1일은 사용함).

## Windows/WSL 관점에서 README·CHANGELOG가 부족한 지점

README/CHANGELOG 전체에서 `windows`, `wsl`, `appdata`, `winget`, `clip.exe`,
`credential manager`, `powershell`, `\mnt\c` 를 검색한 결과, "Windows"라는 단어 자체는
딱 4번만 등장하고 그중 하나는 **"The macOS and Windows binaries are built on every change
and run by nobody."** (README `## What is actually tested`) — 즉 문서 스스로 Windows
바이너리가 실행 검증되지 않았다고 명시한다. `WSL`이라는 단어는 한 번도 나오지 않는다.

구체적으로 인터뷰에서 도윤이 요구했던 것과 실제 문서를 대조하면:

- **설치 경로**: macOS/Linux는 `curl | sh`, Homebrew, `.deb/.rpm/.apk`가 있는데 Windows는
  "release 페이지의 `.zip`"뿐이다. `winget`, Scoop, MSI 언급이 전혀 없다. 서명/SmartScreen에
  대한 안내도 없다(`gh attestation verify`는 checksum 이상을 확인해 주지만 SmartScreen
  경고 자체를 다루지는 않는다).
- **config 경로**: README `## Configure`와 `## What this program leaves on the machine`은
  둘 다 `~/.config/datavase/config.yaml` 또는 `$XDG_CONFIG_HOME`만 말한다. Windows native
  바이너리가 `%APPDATA%`를 쓰는지, WSL의 Linux 바이너리가 WSL 홈의 `~/.config`를 쓰는지
  전혀 명시되지 않는다. 실제 경로를 출력하는 `dv config path` 같은 명령도 없다(`dv help`로
  확인).
- **credential**: `## Configure`가 "headless Linux server runs no D-Bus Secret Service"는
  구체적으로 언급하지만(WSL도 보통 D-Bus 세션이 없어 이 문장이 실질적으로 도움은 된다),
  "OS keychain"이 Windows에서 Credential Manager를 의미하는지, WSL의 Linux 바이너리에서는
  무엇을 쓰는지 이름이 없다. 도윤이 인터뷰에서 정확히 이 지점("여기서 말하는 OS가 무엇인지부터
  명확해야 합니다")을 지적했다.
- **clipboard**: `## The screen` → "Copying" 절은 로컬 세션의 두 번째 경로로 `pbcopy`,
  `wl-copy`, `xclip`만 나열한다. **`clip.exe`는 어디에도 없다.** WSL은 macOS도 아니고
  X11/Wayland도 아니므로, 이 세 도구 중 아무것도 없는 게 보통이다. 그 경우 남는 것은 첫
  번째 경로(터미널에 OSC 52로 요청)뿐인데, README는 이 경로가 거부될 수 있는 구체적 터미널로
  Ghostty·iTerm2·tmux·Terminal.app만 예시로 들고 **Windows Terminal은 한 번도 언급하지
  않는다.** 오늘 이 리허설에서 로컬 macOS 세션의 `Ctrl+C` 셀 복사가 실제로 `pbcopy` 경로를
  타고 성공한 것을 확인했지만(상태줄 `value copied`, `pbpaste`로 검증), 이건 WSL의 답이 될
  수 없다 — WSL이었다면 같은 "value copied" 메시지가 뜨고도 Windows clipboard가 비어
  있었을 가능성을 문서가 배제하지 못한다. 이는 도윤이 "돌아가는 순간"으로 꼽은 바로 그
  실패 모드다.
- **CSV export 경로**: 실제로 `F3` → `c` → 파일명 확인 화면에는 **`pilot-20260922-120042.csv`
  라는 상대 이름만 보이고 절대 경로가 전혀 없었다.** 저장 후 상태줄도 `1 row written to
  pilot-....csv (20 B)`로 같은 상대 이름만 반복했다. 실제로 파일은 `dv`를 실행한 현재
  작업 디렉터리(이번 경우 저장소 루트)에 써졌다 — 이는 도윤이 정확히 걱정한 지점
  ("WSL의 `/home/...`에 저장되면 Windows Explorer나 Teams에서 바로 첨부하기 불편합니다")과
  일치하며, 문서는 이 동작(cwd에 쓴다는 것, 절대 경로가 표시되지 않는다는 것)을 어디에도
  적지 않는다. **부수적 관찰**: 이 리허설 도중 저장소 작업 디렉터리에 다른 세션이 남긴
  것으로 보이는 `shop_ro-....csv`(683KB, 비정상적 카티전-곱 형태의 값)가 이미 존재하는 것을
  발견했다 — 여러 `dv` 프로세스가 같은 cwd를 공유하면 CSV export가 서로 충돌할 수 있다는
  것을 실제로 보여준 사례다(정리를 위해 두 파일 모두 삭제함, 저장소에 잔존하는 변경 없음).
- **키 라벨**: tmux 안에서 dv를 띄우자 실행 키 힌트가 README가 약속한 "tmux 안에서는 Ctrl
  스펠링을 보여준다"가 아니라 **`Super+↩`**로 표시됐다(`Super+Shift+C copy`,
  `Super+Shift+S sort`, `Super+I row` 등도 동일). Windows 키보드에는 물리적으로 "Super"라고
  적힌 키가 없고, README의 키 테이블에도 "Super"라는 이름은 전혀 안 나온다(`⌘`, `Ctrl`,
  `F`-키만 나온다). 실제로는 `F5`와 `Ctrl+C`가 문제없이 동작했으므로 기능은 살아있지만,
  화면에 뜨는 라벨 자체가 README의 키 테이블과도, README 자신의 tmux 약속과도 다른 문구를
  보여준 것은 도윤이 가장 먼저 언급한 "화면에 보이는 동작"을 믿을 수 없게 만드는 지점이다.
  (이 "Super" 라벨이 tcell의 확장 키보드 프로토콜 협상 방식에 따른 것인지, 이 특정 터미널
  환경 조합 때문인지는 이 리허설에서 특정하지 못했다 — 아래 "한계" 참고.)
- **테스트 대상 표**: `## What is actually tested`가 "MariaDB 11.4 / MySQL 8.4 /
  linux/amd64에서 스위트가 돈다"와 "macOS, Linux, Windows 빌드는 되지만 아무도 실행하지
  않는다"를 나란히 명시한 것 자체는 도윤이 요구한 정직함("Windows native와 WSL 중 어디에서
  무엇이 검증됐는지")에 부분적으로 부합한다 — 다만 "아무도 실행하지 않는다"는 문장은 정직한
  만큼 도윤에게는 "직접 실험해야 한다"는 신호로 읽혀, 인터뷰에서 그가 말한 "미룰 가능성이
  높다"는 반응을 그대로 유발할 문구다.

## 페르소나가 자연스럽게 요청했을 법한 기능

- `dv config path` (또는 `dv paths`) — 현재 플랫폼에서 실제로 쓰는 config/state/keychain
  경로를 한 줄씩 출력. 인터뷰에서 명시적으로 요청한 기능이고, 오늘 리허설에서도 "어디에
  쓰는지"를 알기 위해 소스를 확인해야 했던 지점(CSV export 경로, config 경로)과 직결된다.
- CSV export 확인 화면·완료 상태줄에 **절대 경로**를 표시(적어도 저장 직후 상태줄에서라도).
- README의 clipboard 절에 Windows Terminal + WSL(로컬/SSH/tmux 각각)의 실측 결과를 한 줄씩
  추가하거나, 최소한 `clip.exe`를 세 번째 로컬 helper로 언급하고 WSL에서의 한계(D-Bus 없음,
  OSC 52 의존)를 명시.
- README의 "Install" 표를 macOS/Linux처럼 Windows도 `winget`/서명 상태를 갖춘 표로 확장하거나,
  현재 상태(zip만 있음, 서명 없음)를 정직하게 한 줄로 적어 "직접 실험"을 줄여줄 것.

## 이 리허설의 한계

- **실제 WSL 환경이 아니다.** 이 세션은 macOS/darwin 셸에서 실행됐다. clipboard는 `pbcopy`
  경로로 성공을 확인했을 뿐, WSL의 `clip.exe`/OSC 52 조합은 실측하지 못했다. 위의 "clipboard"
  항목은 README 문구를 정독해 논리적으로 도출한 격차이지, 실제 실패를 재현한 것이 아니다.
- tmux 안에서 `Super+↩`가 뜬 것도 이 특정 macOS+Homebrew tmux 3.6b+터미널 조합의 산물일 수
  있다. Windows Terminal + WSL2 tmux 조합에서 같은 라벨이 뜨는지는 확인하지 못했다 — 다만
  README 자신이 약속한 "tmux에서는 Ctrl 스펠링"과 다른 결과가 나왔다는 사실 자체는 로컬에서도
  재현됐으므로, 이 라벨링 로직을 실제 WSL 터미널에서 검증할 가치가 있다는 신호로 남긴다.
- 자동 테스트 하니스(TUI를 tmux로 원격 조종)를 쓰느라 설치~첫 쿼리 소요 시간의 "체감" 추정은
  근사치다. 실제 사람이 겪을 마찰(단축키를 찾아 헤매는 시간 등)과 이 리허설 자체의 도구적
  마찰(tmux 소켓 설정 등)을 구분해서 적었다.
- `dv auth`의 키체인 실패는 "TTY 아님"이라는, 헤드리스 자동화에 대한 안전장치였지 WSL의 GUI
  세션 부재로 인한 Secret Service 잠금 실패는 아니다. 두 실패는 다른 원인이며, 이 리허설은
  후자를 재현하지 못했다.
- DB는 로컬 Docker 컨테이너(`shop` 스키마, `customers`/`orders`)이며 실제 회사 production
  규모나 VPN/bastion 경유가 아니다. SSH tunnel 관련 페르소나의 우려(SSH key 경로, `/mnt/c`
  vs WSL 홈)는 이번 시나리오에 tunnel 설정이 없어 전혀 실측하지 못했다.

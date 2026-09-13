## Interview 13 — 박도윤: Windows와 WSL을 오가는 Backend Developer

> 합성 페르소나 시뮬레이션이며 실제 사용자 인터뷰나 수요의 증거가 아니다.

### 페르소나

- 32세, B2B SaaS Backend Developer, 경력 7년
- 회사 지급 Windows 11 노트북에서 Windows Terminal과 WSL2 Ubuntu를 사용
- Java·Spring Boot 애플리케이션은 WSL에서 실행하고 IntelliJ IDEA와 DataGrip은 Windows에서 사용
- 로컬·개발 DB는 Docker Compose의 MySQL, staging·production은 VPN과 SSH bastion을 거쳐 접근
- 장애 대응 중에는 WSL의 `ssh`, `kubectl`, `mysql`을 사용하지만 정기 분석은 DataGrip에서 수행
- PowerShell과 WSL 사이의 경로·환경변수·credential 차이 때문에 CLI 설치 위치를 신중하게 선택함
- 새로운 단축키를 외우기보다 화면에 보이는 동작과 기존 terminal conventions를 선호
- “Windows 지원”이라는 표현보다 실제 지원하는 shell, clipboard, credential store, 설치 경로가 명시되기를 요구

### 인터뷰 목적

Windows Terminal과 WSL을 함께 사용하는 개발자에게 datavase의 single binary와 terminal workflow가 실제 이점을 주는지 검증한다. 특히 Windows·Linux 중 어느 쪽에 설치해야 하는지, modifier key와 clipboard가 일관되게 작동하는지, credential과 config가 어디에 저장되는지, 배포 및 업데이트 마찰이 채택을 막는지 확인한다.

### 인터뷰

**Interviewer:** 최근 terminal에서 데이터베이스를 직접 확인했던 상황을 처음부터 설명해주세요.

**도윤:** production에서 특정 고객의 배치 작업이 실패했습니다. Windows에서 VPN을 켜고 Windows Terminal의 WSL 탭을 열었어요. `kubectl`로 pod 로그를 확인한 뒤 주문 ID를 복사해서 bastion에 SSH로 접속하고 `mysql`에서 조회했습니다. 컬럼이 많아서 결과는 `\\G`로 다시 봤고, 필요한 값은 마우스로 선택해 Teams에 붙여넣었습니다.

**Interviewer:** DataGrip을 사용하지 않은 이유는 무엇입니까?

**도윤:** 이미 WSL과 SSH 안에서 로그를 보고 있었기 때문입니다. DataGrip의 tunnel 설정도 있지만 VPN이 연결된 상태에서 datasource를 찾고 창을 전환하는 것보다 바로 `mysql`을 여는 편이 빨랐어요. 복잡한 쿼리를 작성하는 일이었다면 DataGrip을 썼을 겁니다.

**Interviewer:** 그 과정에서 가장 불편했던 부분은 무엇이었나요?

**도윤:** 결과를 읽고 복사하는 부분입니다. 다만 그 불편 때문에 새 프로그램을 바로 설치할 정도는 아니에요. WSL에 설치할지 Windows에 설치할지부터 판단해야 하고, 회사 PC에서는 출처가 명확하지 않은 실행 파일이 차단될 수도 있습니다.

**Interviewer:** 평소 CLI 프로그램은 어디에 설치합니까?

**도윤:** 개발 도구는 대부분 WSL에 설치합니다. `apt`나 Linux용 binary를 쓰고요. Windows 전용 도구는 `winget`을 우선 봅니다. 같은 명령을 양쪽에 중복 설치하는 건 피합니다. 버전과 설정이 서로 달라져서 나중에 어느 것을 실행한 건지 헷갈리거든요.

**Interviewer:** datavase가 Windows와 Linux용 단일 binary를 제공한다면 충분할까요?

**도윤:** 다운로드할 수 있다는 것과 잘 지원된다는 것은 다릅니다. WSL에서 Linux binary를 실행하면 clipboard는 Windows와 연결되는지, 브라우저 없는 환경에서 credential prompt가 되는지, config가 Windows 쪽인지 WSL 홈에 생기는지 알아야 합니다. 문서가 그냥 “Windows supported”라면 직접 실험해야 해서 미룰 가능성이 높아요.

**Interviewer:** 설치한다면 어느 환경을 먼저 선택하겠습니까?

**도윤:** WSL입니다. SSH key와 shell alias, `kubectl` context가 전부 거기에 있으니까요. Windows binary를 쓰면 `C:\\Users`의 SSH 설정을 따로 보거나 WSL 파일 경로를 넘겨야 할 것 같아서 현재 흐름과 안 맞습니다.

**Interviewer:** WSL용 설치 명령이 tarball을 내려받아 PATH에 복사하는 방식이라면요?

**도윤:** 한 번 시험하는 데는 괜찮지만 계속 쓰려면 업데이트가 문제입니다. checksum 검증, 현재 버전 확인, 업그레이드 방법이 문서에 있어야 합니다. `curl | sh`만 제공하면 회사에서는 실행하기 어렵고요. GitHub Release에서 파일을 직접 받아도 되지만 반복 사용 도구라면 package manager 경로가 있으면 좋습니다.

**Interviewer:** Windows에서는 어떤 설치 경로를 기대합니까?

**도윤:** `winget`이 가장 자연스럽습니다. Scoop도 개발자 사이에서는 쓰지만 회사 표준은 아니에요. MSI installer까지 꼭 필요하다는 뜻은 아닙니다. 다만 zip을 풀고 PATH를 수동 수정하는 것만 있으면 시험 사용자 수가 줄 것 같아요. SmartScreen이나 백신 경고가 뜰 때 확인할 수 있는 서명과 checksum 정보도 중요합니다.

**Interviewer:** single binary라는 점은 매력적입니까?

**도윤:** runtime과 driver를 추가로 설치하지 않는 것은 좋습니다. 특히 WSL 배포판을 새로 만들 때 유리하죠. 하지만 single binary여도 config, keychain helper, clipboard helper가 별도로 필요하면 실제 설치 경험은 단순하지 않습니다. 그 의존성을 숨기지 않는 게 더 중요해요.

**Interviewer:** 결과 grid에서 현재 cell을 복사하는 단축키가 `Cmd+C`라고 안내되어 있습니다.

**도윤:** Windows 키보드에는 Cmd가 없습니다. `Ctrl+C`를 기대하겠지만 terminal에서는 프로세스 중단과 충돌할 수 있죠. 그 문구를 보면 macOS 우선 제품이라고 생각합니다. Windows와 Linux의 실제 키를 별도로 보여줘야 합니다.

**Interviewer:** `Ctrl+C`를 copy로 사용한다면 어떻습니까?

**도윤:** SQL 실행 중 취소와 충돌하지 않는지 먼저 궁금합니다. terminal에서 `Ctrl+C`는 너무 강한 관습이라 상태에 따라 의미가 바뀌면 실수할 수 있어요. 선택 모드에서는 copy, 실행 중에는 cancel처럼 동작한다면 화면에 현재 동작이 보여야 합니다. 마우스 클릭 메뉴가 있으면 단축키를 외우지 않고 시작할 수 있고요.

**Interviewer:** cell, row, 전체 result에 각각 다른 단축키가 있다면요?

**도윤:** 자주 쓰기 전에는 못 외웁니다. `Ctrl+Shift` 조합은 Windows Terminal 자체 단축키와 겹칠 수도 있고 키보드 배열에 따라 불편해요. 화면의 copy 메뉴에서 범위를 고르게 하고, 단축키는 옆에 힌트로 보여주는 게 낫습니다.

**Interviewer:** WSL에서 clipboard 복사가 동작하면 충분합니까?

**도윤:** 어떤 방식으로 동작하는지가 중요합니다. WSL에서 `clip.exe`로 Windows clipboard에 쓰는 방식은 익숙하지만, 원격 SSH나 tmux 안으로 들어가면 조건이 달라집니다. 로컬 WSL, SSH session, tmux 각각에서 실제로 검증됐는지 알고 싶어요. 복사 성공 메시지가 떴는데 Windows clipboard에는 없는 상황이 가장 싫습니다.

**Interviewer:** clipboard 복사가 실패하면 파일 export로 대체할 수 있습니다.

**도윤:** 데이터가 몇 개면 마우스로 복사하거나 SQL을 다시 실행하겠습니다. CSV라면 저장 경로가 더 복잡해져요. WSL의 `/home/...`에 저장되면 Windows Explorer나 Teams에서 바로 첨부하기 불편하고, `/mnt/c/Users/.../Downloads`로 저장하면 권한과 경로 표기가 달라집니다.

**Interviewer:** 기본 다운로드 폴더를 자동으로 선택하면 좋을까요?

**도윤:** 자동 선택보다 최종 경로를 분명히 보여주세요. 회사 OneDrive 때문에 실제 Windows 다운로드 경로가 일반적인 위치와 다를 수도 있습니다. WSL에서 Windows 경로를 추측해서 저장하는 것보다 최근 경로를 기억하고 Linux 경로와 Windows에서 접근할 경로를 함께 안내하는 게 낫습니다.

**Interviewer:** datasource password를 OS keychain에 저장한다면 신뢰할 수 있습니까?

**도윤:** 여기서 말하는 OS가 무엇인지부터 명확해야 합니다. Windows binary라면 Windows Credential Manager를 기대합니다. WSL의 Linux binary라면 Secret Service나 별도 keyring일 텐데, GUI session이 없으면 잠금 해제 prompt가 실패할 수 있어요. WSL에서 Windows Credential Manager를 쓴다는 뜻인지도 모르겠고요.

**Interviewer:** headless 환경에서는 environment variable도 지원합니다.

**도윤:** 그쪽이 오히려 예측 가능할 수 있습니다. 하지만 `.bashrc`에 password를 평문으로 넣으라는 방식이면 쓰지 않습니다. 1Password CLI, 회사 secret manager, 일시적인 environment variable 같은 흐름과 연결할 수 있는지 보고 싶어요. 지원하지 않는다면 password를 저장하지 않고 매번 입력하는 선택도 필요합니다.

**Interviewer:** credential 저장을 사용하지 않고 실행할 때마다 입력할 수 있다면요?

**도윤:** production에는 그 방식도 괜찮습니다. 주당 한두 번 접속한다면 약간의 불편보다 저장 위치가 명확한 편이 낫습니다. 중요한 것은 prompt 입력값이 shell history나 config에 남지 않는다는 보장입니다.

**Interviewer:** datasource config는 어디에 있어야 자연스럽습니까?

**도윤:** WSL binary라면 `$XDG_CONFIG_HOME`이나 `~/.config` 아래를 기대합니다. Windows binary라면 `%APPDATA%` 계열이겠죠. 문제는 README 예시가 한 플랫폼 경로만 보여주는 경우입니다. 실제 경로를 출력하는 `dv config path` 같은 명령이 있으면 백업이나 삭제할 때 편합니다.

**Interviewer:** Windows와 WSL에서 같은 config를 공유하면 편하지 않을까요?

**도윤:** 처음에는 편해 보이지만 권장하지 않습니다. SSH key 경로도 다르고 socket이나 helper 경로도 달라요. `/mnt/c`의 한 파일을 함께 읽으면 permissions가 Linux 기대와 다를 수도 있습니다. 공통 datasource 정보와 플랫폼별 secret·path override를 분리할 수 있다면 모르겠지만, 단순히 한 파일을 공유하면 디버깅 비용이 큽니다.

**Interviewer:** 설정 파일을 수동으로 편집해야 한다면 설치 의향이 낮아집니까?

**도윤:** YAML 몇 줄 자체는 괜찮습니다. 예제에 Windows와 WSL의 경로 표현, SSH key, 환경변수 참조가 정확히 있어야 해요. 백슬래시 escaping 때문에 실패하거나 `~`가 어느 홈인지 모호하면 바로 중단합니다. 설정 오류가 발생했을 때 문제 필드와 읽은 config 경로도 보여줘야 합니다.

**Interviewer:** interactive datasource 등록 화면과 config 파일 중 어느 쪽을 선호합니까?

**도윤:** 처음에는 interactive가 빠르지만 결과가 어디에 저장됐는지 볼 수 있어야 합니다. 팀에서 재현하려면 config 예시가 필요하고요. 둘 중 하나만 고르라면 저는 파일을 택하겠지만, Windows 사용자를 넓게 잡는다면 화면 등록이 진입 장벽은 낮출 겁니다.

**Interviewer:** datasource의 `read_only: true`가 Windows·WSL 선택에 영향을 줍니까?

**도윤:** 제품 가치에는 영향을 주지만 플랫폼 선택 문제를 해결하지는 않습니다. production에서 UPDATE를 막아주는 것은 좋습니다. 그래도 clipboard가 안 되거나 SSH key를 못 찾으면 `mysql`로 돌아갑니다. read-only 하나가 기본 workflow의 마찰을 상쇄하지는 못해요.

**Interviewer:** 현재 mysql CLI와 비교해 전환 조건은 무엇입니까?

**도윤:** WSL의 기존 SSH 설정을 그대로 쓰고, 복사 결과가 Windows clipboard에 확실히 들어가고, 설정과 credential 위치를 설명할 수 있어야 합니다. 업데이트도 어렵지 않아야 하고요. 이 네 가지가 되면 조회 결과를 보기 위해 사용할 이유가 있습니다.

**Interviewer:** 어떤 실패가 한 번만 발생해도 돌아갈 가능성이 높습니까?

**도윤:** clipboard가 성공했다고 표시했는데 붙여넣기가 비어 있는 경우입니다. 그다음은 keychain이 잠겼다며 매번 이상한 GUI prompt를 띄우는 경우, WSL 재시작 후 config를 못 찾는 경우예요. 장애 중에는 원인 분석 대상이 하나 더 생기는 도구를 쓰지 않습니다.

**Interviewer:** DataGrip을 대체할 수 있습니까?

**도윤:** 아닙니다. IntelliJ와 함께 쓰는 DataGrip은 schema 탐색과 쿼리 작성에 계속 필요합니다. datavase가 대체할 수 있는 것은 WSL에서 실행하는 짧은 `mysql` 조회입니다. Windows용 GUI와 경쟁한다고 설명하면 관심이 줄어듭니다.

**Interviewer:** README 첫 화면에서 무엇을 확인하고 싶습니까?

**도윤:** 지원 플랫폼 표가 먼저 필요합니다. Windows native, WSL2, Windows Terminal, SSH, tmux에서 무엇을 테스트했는지요. 설치 demo라면 WSL에서 실행해 cell을 복사하고 Windows 앱에 붙여넣는 장면이 설득력 있습니다. 단축키도 macOS 화면만 보여주면 안 되고요.

**Interviewer:** Windows와 WSL 중 하나만 공식 지원한다면 어느 쪽이 낫습니까?

**도윤:** 제 workflow에는 WSL이 낫습니다. 애매하게 둘 다 지원한다고 하는 것보다 WSL2 Ubuntu와 Windows Terminal 조합을 명시하고 테스트하는 편이 신뢰됩니다. 반대로 일반 Windows 개발자를 넓게 노린다면 `winget`, Credential Manager, PowerShell까지 별도 품질 기준이 필요할 겁니다.

**Interviewer:** 지금 상태에서 설치해볼 의향이 있습니까?

**도윤:** README에 WSL 설치, 실제 config 경로, credential fallback, clipboard 지원 범위가 있으면 테스트 환경에서는 해보겠습니다. “single binary니까 어디서나 됩니다” 정도라면 보류합니다. 제가 직접 Windows와 Linux의 경계를 디버깅해야 할 가능성이 높기 때문입니다.

### 결정적 발언

> “Windows 지원이라는 한 줄보다 Windows native와 WSL 중 어디에서 무엇이 검증됐는지가 필요합니다.”

> “복사 성공 메시지가 떴는데 Windows clipboard가 비어 있으면, 그 순간 다시 `mysql`로 돌아갑니다.”

> “single binary여도 config와 credential이 어느 운영체제에 저장되는지 모르면 설치가 단순한 것이 아닙니다.”

### Interview 13 결과

Windows와 WSL을 함께 사용하는 개발자에게 datavase는 terminal DB 조회 경험을 개선할 가능성이 있다. 이 페르소나는 이미 장애 대응을 WSL의 SSH·kubectl·mysql 안에서 수행하므로, 제품이 경쟁해야 할 대상도 DataGrip이 아니라 그 흐름 속의 raw `mysql`이다.

그러나 “Windows 지원”은 단일한 조건이 아니다. Windows native binary와 WSL Linux binary는 설치, config 경로, SSH 자산, credential store, clipboard, export 경로가 서로 다르다. 두 환경을 모두 제공하는 것만으로는 채택 장벽이 낮아지지 않으며, 검증된 권장 경로와 실패 동작을 명확히 설명해야 한다.

| 영역 | 기대하는 가치 | 전환을 막는 위험 |
|---|---|---|
| terminal fit | WSL의 기존 SSH·kubectl workflow 유지 | Windows binary와 WSL 도구 사이의 context 분리 |
| keybindings | keyboard 중심의 빠른 결과 조작 | `Cmd` 안내, `Ctrl+C` cancel 충돌, terminal shortcut 중복 |
| clipboard | WSL 결과를 Windows 앱에 즉시 전달 | SSH·tmux 경계에서 조용히 실패하거나 다른 clipboard 사용 |
| credential | password의 평문 저장 방지 | Windows Credential Manager와 Linux keyring의 모호한 선택 |
| config | datasource 재사용과 환경 재현 | `%APPDATA%`, XDG, `/mnt/c` 사이의 위치·권한 혼동 |
| CSV export | 조회 결과의 업무 전달 단축 | Linux 경로가 Windows 앱에서 접근하기 어렵거나 민감 파일 잔존 |
| distribution | runtime 없는 간단한 설치 | 수동 PATH 설정, SmartScreen·백신 경고, 불명확한 업데이트 절차 |
| read-only | production accidental write 감소 | 기본 연결·복사 실패를 보완하지 못하며 보안 경계로 오해 가능 |

이 사용자의 채택 조건은 새로운 DB 기능이 아니라 **WSL에서 시작한 작업이 Windows clipboard와 파일 전달까지 끊기지 않는 것**이다. 따라서 플랫폼 지원 범위를 넓게 선언하기보다 권장 조합 하나를 end-to-end로 검증하고, 다른 조합의 제한을 공개하는 편이 신뢰 형성에 유리하다.

초기 ICP로는 조건부 적합하다. Windows 회사 환경에서도 개발 workflow를 WSL에 집중한 Backend·Platform Engineer는 terminal fit이 높다. 다만 Windows native와 WSL을 동시에 우선 지원하려 하면 작은 팀의 테스트 범위가 급격히 커질 수 있으므로, 실제 설치·재사용 데이터를 통해 우선순위를 정해야 한다.

### 검증 후보

다음 항목은 이 페르소나의 가정을 실제 사용자 인터뷰와 제품 테스트로 검증하기 위한 후보이다.

1. Windows 개발자가 DB CLI를 Windows native, PowerShell, Git Bash, WSL 중 어디에서 실제 실행하는지
2. WSL2 Ubuntu와 Windows Terminal 조합에서 설치부터 첫 query까지 걸리는 시간과 중단 지점
3. 로컬 WSL, WSL→SSH, WSL→SSH→tmux 각각에서 cell·row·result clipboard 복사가 실제 Windows clipboard로 전달되는지
4. `Ctrl+C`, `Ctrl+Shift+C`, terminal 자체 keybinding, query cancellation 사이에 충돌이 발생하는지
5. mouse/menu 기반 copy 발견 가능성과 플랫폼별 shortcut 힌트의 이해도
6. Windows Credential Manager, Linux Secret Service, 매회 prompt, environment variable 중 WSL 사용자가 신뢰하는 credential 방식
7. GUI keyring이 없거나 잠긴 WSL session에서 credential 접근 실패가 빠르고 이해 가능한 메시지로 나타나는지
8. Windows native와 WSL의 실제 config·history·log·export 경로를 사용자가 찾고 완전히 삭제할 수 있는지
9. `$XDG_CONFIG_HOME`, `~/.config`, `%APPDATA%`, `/mnt/c` 및 공백이 포함된 Windows 경로에서 설정 파싱이 일관적인지
10. `winget` 또는 Linux package manager 제공 여부가 최초 설치와 30일 후 업데이트율에 미치는 영향
11. unsigned binary, SmartScreen, 회사 백신·application control이 설치를 차단하는 실제 비율
12. README의 WSL end-to-end demo와 일반적인 “Windows supported” 문구 중 어느 쪽이 설치 시도를 더 잘 유도하는지

---

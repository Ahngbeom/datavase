## Interview 19 — 최도윤: 기존 CLI를 직접 고쳐 쓰는 오픈소스 파워 유저

> 합성 페르소나 시뮬레이션이며 실제 사용자 인터뷰나 수요의 증거가 아니다.

### 페르소나

- 36세, Senior Backend Engineer, 경력 11년
- 개발자 60명 규모의 커머스 회사에서 Python·Go 서비스와 MySQL 운영을 담당
- macOS와 Linux를 오가며 `tmux`, shell alias, dotfiles를 적극적으로 관리
- 일상적인 조회에는 `mycli`, 자동화·장애 복구에는 표준 `mysql` client를 사용
- 복잡한 분석은 DataGrip으로 넘기지만 간단한 조회는 terminal 안에서 끝내는 편
- completion, pager, output format, history 동작을 자신의 workflow에 맞게 조정해 사용
- 새 CLI를 발견하면 source, release workflow, dependency와 maintainer 응답 이력을 먼저 확인
- 유용한 작은 도구에는 issue나 작은 patch를 보내지만, 장기 유지 가능성이 불명확하면 빠르게 이탈
- 기능 수보다 예측 가능한 동작과 기존 도구에서 옮겨갈 수 있는 명확한 이유를 중시

### 인터뷰 목적

이미 `mycli`와 `mysql`을 능숙하게 조합하는 사용자가 datavase로 전환할 이유가 있는지 확인한다. configurability와 단순성 사이의 경계, binary와 release의 provenance가 신뢰에 미치는 영향, 사용자가 issue·patch를 제출하거나 장기적으로 남게 되는 조건을 검증한다.

### 인터뷰

**Interviewer:** 최근 MySQL을 terminal에서 확인했던 상황을 처음부터 설명해주세요.

**도윤:** 배포 뒤 주문 상태가 일부 늦게 바뀐다는 제보가 있었습니다. 이미 `tmux`에서 로그를 보고 있었고 주문 ID가 있었어요. read replica용 alias로 `mycli`를 열어 주문과 비동기 작업 테이블을 조회했습니다. 결과가 길어서 vertical output으로 바꾸고 두세 번 더 실행한 뒤 종료했어요.

**Interviewer:** 왜 표준 mysql client가 아니라 mycli를 사용했습니까?

**도윤:** 사람이 직접 탐색할 때는 completion과 syntax highlight, history search가 편합니다. 테이블 이름이나 컬럼을 전부 기억할 필요가 없으니까요. 반면 pipe나 script에 연결하거나 장애 서버에 최소 도구만 있을 때는 `mysql`을 씁니다.

**Interviewer:** 두 도구를 번갈아 쓰는 것이 불편하지 않나요?

**도윤:** 특별히요. 역할이 나뉘어 있습니다. interactive 탐색은 `mycli`, 어디서나 재현되어야 하는 명령은 `mysql`입니다. 새 도구는 둘 중 하나보다 조금 예쁜 정도로는 자리를 얻기 어렵습니다.

**Interviewer:** mycli에서 실제로 바꿔 사용하는 설정은 무엇입니까?

**도윤:** pager, table format, multiline mode, key binding, history 위치 정도입니다. datasource별 접속 값은 별도 파일과 environment variable로 관리하고 shell function에서 조합해요. 모든 설정을 매일 만지는 건 아니고, 처음 맞춘 뒤 거의 그대로 씁니다.

**Interviewer:** 설정 가능한 항목이 많을수록 새 CLI를 더 높게 평가합니까?

**도윤:** 아닙니다. 제가 바꾸는 몇 가지가 정확히 동작하면 됩니다. 옵션이 많다는 건 호환성 표면과 문서가 늘어난다는 뜻이기도 해요. 기본값이 좋고, 반복해서 부딪히는 지점만 열려 있는 게 낫습니다.

**Interviewer:** 그렇다면 설정을 거의 제공하지 않는 제품도 괜찮습니까?

**도윤:** 핵심 workflow와 맞으면요. 하지만 terminal 도구가 pager, 색상, key binding, history 경로 같은 기본적인 환경 결정을 강제로 가져가면 오래 쓰기 어렵습니다. 단순함은 선택권이 전혀 없다는 뜻은 아니죠.

**Interviewer:** datavase는 datasource 관리, table 확인, SQL 실행, 결과 반출에 집중한 단일 binary입니다. 첫인상은 어떻습니까?

**도윤:** 범위가 명확한 건 좋습니다. 그런데 지금 설명만으로는 `mycli` 대신 설치할 이유가 보이지 않습니다. grid가 더 좋을 수는 있지만, completion과 history가 익숙한 도구에서 옮기는 비용도 있습니다.

**Interviewer:** datasource에 `read_only: true`를 설정하면 connection을 session-level read-only로 시작합니다.

**도윤:** production용 profile에 넣는 guardrail로는 유용합니다. 다만 저는 이미 replica와 read-only 계정을 우선 사용합니다. 그것만으로 전환하지는 않겠지만, 잘못된 endpoint에 접속했을 때 한 겹 더 막아주는 차이는 있습니다.

**Interviewer:** 화면에 read-only라고 표시되면 충분히 신뢰하겠습니까?

**도윤:** 표시보다 실제 session 상태를 확인하겠습니다. 어떤 SQL을 실행했고 reconnect 뒤에도 적용됐는지, transaction이나 server 설정에 따라 무엇이 달라지는지가 중요해요. 제품 문구만으로 권한 경계라고 믿지는 않습니다.

**Interviewer:** CSV export와 cell·row copy는 어떻습니까?

**도윤:** ad hoc 전달에는 편하겠지만 `mysql --batch`와 shell redirection으로 이미 가능합니다. 사람에게 보여줄 작은 결과라면 grid copy가 빠를 수 있어요. 반복 작업이면 여전히 script를 만들 겁니다.

**Interviewer:** datavase가 결과를 더 읽기 쉽게 보여주면 mycli를 대체할 수 있습니까?

**도윤:** 며칠은 써볼 수 있습니다. 대체 여부는 예쁜 첫 화면보다 long text, NULL, binary, Unicode, 큰 result set에서 값이 정확히 보이는지에 달렸어요. pager가 느리거나 copy 결과가 화면과 다르면 바로 돌아갑니다.

**Interviewer:** 설치 전에 무엇을 확인합니까?

**도윤:** source repository, license, 최근 release와 commit, binary를 누가 만들었는지, checksum이나 signature가 있는지 봅니다. 설치 script가 어디서 무엇을 내려받는지도 확인하고요. DB credential을 다루는 프로그램이니 일반적인 편의 CLI보다 기준이 높습니다.

**Interviewer:** GitHub Actions가 release binary를 만들고 checksum을 제공하면 충분합니까?

**도윤:** 좋은 출발입니다. workflow가 repository에 있고 tag와 binary가 연결되는지 확인할 수 있어야 합니다. 가능하면 reproducible build나 provenance attestation이 있으면 좋지만, 작은 프로젝트에 처음부터 완벽한 공급망 체계를 요구하지는 않아요.

**Interviewer:** single static binary라는 점은 강한 장점입니까?

**도윤:** 설치와 삭제가 쉬운 건 확실한 장점입니다. `mycli`의 Python 환경이나 dependency 충돌을 피할 수 있으니까요. 다만 binary 하나라는 설명이 그 binary를 신뢰할 근거까지 제공하지는 않습니다.

**Interviewer:** Homebrew 설치가 제공되면 바로 사용하시겠습니까?

**도윤:** 임시 데이터베이스부터 시험할 가능성은 높습니다. 하지만 production datasource를 등록하기 전에는 config와 secret 저장 방식, network 동작, release provenance를 확인할 겁니다.

**Interviewer:** README demo에서 어떤 장면이 가장 중요합니까?

**도윤:** 단순 SELECT grid보다 `mycli`와 다른 순간을 보여줘야 합니다. production datasource가 실제 session read-only로 열리고, reconnect 뒤에도 유지되며, 결과를 정확하게 복사하는 흐름이요. animation만 있고 재현 가능한 명령이나 sample database가 없으면 평가하기 어렵습니다.

**Interviewer:** 제품이 지원하는 설정을 넓히면 power user를 더 쉽게 확보할 수 있지 않을까요?

**도윤:** 사용자별 config를 계속 추가하면 다른 CLI의 복제품이 됩니다. 먼저 어떤 설정이 없어서 실제 작업이 막혔는지를 issue로 모으는 편이 낫습니다. 저도 option이 없다는 이유만으로 요청하지 않고, 반복해서 우회해야 할 때 구체적인 사례를 냅니다.

**Interviewer:** 첫 issue를 제출하는 조건은 무엇입니까?

**도윤:** 도구가 기본적으로 쓸 만하고 maintainer가 재현 정보를 받을 준비가 되어 있을 때입니다. issue template, debug log에서 secret이 제거되는지, supported version이 명확한지를 봅니다. 한 번 실행하고 별로인 도구에는 issue를 쓰는 대신 삭제합니다.

**Interviewer:** bug를 발견하면 바로 patch를 보냅니까?

**도윤:** 작은 수정이고 local에서 검증할 수 있으면요. 먼저 issue나 discussion으로 의도를 확인합니다. architecture를 크게 바꾸는 patch를 갑자기 보내진 않습니다. test 방법과 contribution guide가 없으면 수정 비용이 급격히 올라가고요.

**Interviewer:** contribution guide가 있어야만 기여합니까?

**도윤:** 반드시 긴 문서일 필요는 없습니다. build, test, formatting 명령과 scope 원칙 정도면 됩니다. PR #82처럼 의도적으로 기능을 줄였다면 그 결정이 기록돼 있어야 제가 삭제된 기능을 다시 만드는 실수를 피할 수 있습니다.

**Interviewer:** maintainer의 어떤 반응이 신뢰를 만듭니까?

**도윤:** 모든 요청을 받아주는 것보다 왜 scope에 맞거나 맞지 않는지 일관되게 설명하는 반응입니다. 작은 bug report에 재현 요청이 오고, release에 언제 포함됐는지 추적되는 것도 중요합니다. 반대로 roadmap 약속만 많고 release가 없으면 떠납니다.

**Interviewer:** issue 요청이 거절되면 사용을 중단합니까?

**도윤:** 이유에 따라 다릅니다. 제품의 작은 범위를 지키기 위한 거절이면 오히려 신뢰할 수 있어요. 제 workflow에 필수인 기능이 영구적으로 scope 밖이라면 불만 없이 다른 도구를 쓰겠죠.

**Interviewer:** plugin이나 scripting API를 제공하면 configurability 문제를 해결할 수 있습니까?

**도윤:** 아직은 과한 것 같습니다. plugin API는 호환성을 유지해야 하는 별도 제품 표면입니다. 먼저 export format, external editor, pager 같은 좁은 integration point가 실제로 필요한지 확인하세요. power user라는 이유로 무조건 plugin을 원하는 건 아닙니다.

**Interviewer:** query history는 어느 수준이어야 합니까?

**도윤:** session을 넘어 검색할 수 있어야 유용하지만 production query와 개인정보가 로컬에 남는 문제가 있습니다. 저장 위치, 비활성화, 보존 기간, file permission을 알아야 해요. 편의 기능을 조용히 켜두는 건 싫습니다.

**Interviewer:** datasource config를 Git으로 공유하는 방식은 어떻습니까?

**도윤:** secret이 분리되고 schema가 안정적이면 좋습니다. 어떤 값이 committed되어도 안전한지 예제가 분명해야 합니다. config format이 자주 바뀌거나 migration 없이 깨지면 팀에 추천하지 않을 겁니다.

**Interviewer:** 기존 dotfiles workflow와 맞추기 위해 가장 필요한 것은 무엇입니까?

**도윤:** config 경로를 명시할 수 있고 environment variable이나 keychain reference를 지원하는 것입니다. export 가능한 비밀 없는 예제도 필요하고요. 모든 UI 상태까지 dotfiles로 관리할 필요는 없습니다.

**Interviewer:** datavase를 일주일 사용한 뒤 유지할지 판단한다면 무엇을 보겠습니까?

**도윤:** `mycli`를 열려다가 datavase를 자연스럽게 선택한 횟수를 볼 겁니다. read-only production 조회와 넓은 결과 확인에서 반복적으로 더 빠르면 남습니다. shortcut을 기억하지 못해 매번 도움말을 열거나 output 정확성을 다시 확인해야 하면 지웁니다.

**Interviewer:** 한 달 뒤에도 남아 있을 최소 조건은 무엇입니까?

**도윤:** production 조회에서 실패 없이 썼고, release update가 예측 가능하며, config가 깨지지 않는 것입니다. 최소 주 2~3회는 실제 작업에서 선택해야 합니다. 한 달에 한 번 demo처럼 실행하는 정도면 별도 도구를 유지하지 않아요.

**Interviewer:** mycli에서 datavase로 완전히 옮겨야만 성공이라고 보나요?

**도윤:** 아닙니다. production read는 datavase, 자유로운 local 탐색은 mycli처럼 역할이 나뉠 수 있습니다. 오히려 모든 용도를 대체한다고 주장하면 비교 항목만 늘어납니다.

**Interviewer:** 팀 동료에게 추천하는 조건은 무엇입니까?

**도윤:** 제가 몇 주 사용한 뒤 동일한 datasource config를 secret 없이 공유할 수 있고, macOS와 Linux에서 같은 동작을 확인했을 때입니다. 그리고 read-only가 safety guarantee가 아니라 session guardrail이라는 설명도 함께 전달할 겁니다.

**Interviewer:** 유료라면 사용할 의향이 있습니까?

**도윤:** 개인용 CLI subscription에는 부정적입니다. 회사에서 표준 도구가 되고 support나 signed release가 제공된다면 비용을 검토할 수 있어요. 개인적으로는 꾸준히 쓰는 프로젝트에 sponsor하거나 code contribution을 하는 쪽이 자연스럽습니다.

**Interviewer:** 지금 설치하겠습니까?

**도윤:** Homebrew formula와 checksum, sample database가 준비돼 있다면 시험은 해보겠습니다. 다만 `mycli`보다 낫다는 기대 때문이 아니라 production read-only workflow가 분명히 분리되는지 확인하려는 목적입니다. 첫 주에 그 차이를 체감하지 못하면 삭제할 가능성이 높습니다.

### 결정적 발언

> “새 도구는 mycli보다 조금 예쁜 정도로는 자리를 얻기 어렵습니다.”

> “단순함은 선택권이 전혀 없다는 뜻이 아니라, 반복해서 막히는 지점만 신중하게 여는 것에 가깝습니다.”

> “binary 하나라는 설명은 설치 편의를 말해줄 뿐, 그 binary를 신뢰할 근거까지 제공하지는 않습니다.”

> “한 번 실행하고 별로인 도구에는 issue를 쓰는 대신 삭제합니다.”

### Interview 19 결과

이 페르소나는 datavase의 terminal workflow와 높은 행동 적합도를 보이지만, 이미 `mycli`와 `mysql`을 목적에 맞게 조합하므로 전환 기준도 높다. grid, completion, CSV 같은 개별 편의는 기존 workflow에서 대체 가능하다. 차별화 가능성이 있는 영역은 production read에 한정된 datasource-level guardrail, 명확한 context, 정확한 결과 처리다.

| 영역 | 현재 workflow | datavase의 잠재 가치 | 전환·잔존을 막는 조건 |
|---|---|---|---|
| 대화형 탐색 | `mycli` completion·history·pager | 읽기 쉬운 grid와 datasource context | 출력 개선만 있고 반복 작업이 더 느림 |
| 자동화·복구 | 표준 `mysql` + shell | 목표 범위가 아님 | pipe·script 호환까지 무리하게 확장 |
| 운영 보호 | replica·read-only 계정 | session-level 추가 guardrail | 권한 경계처럼 과장하거나 reconnect 후 해제 |
| 설정 | dotfiles와 소수의 선별 옵션 | config path·secret reference | 옵션 부족으로 반복 우회하거나 설정 표면이 과대화 |
| 결과 정확성 | vertical output·batch mode | wide result·copy 개선 | NULL, binary, Unicode, 큰 결과에서 불일치 |
| 공급망 신뢰 | source·release workflow 확인 | single binary와 checksum | tag·source·binary 관계가 불투명함 |
| 기여 | 재현 가능한 작은 issue·patch | 초기 contributor 가능성 | build/test 방법이나 scope 원칙 부재 |
| retention | 주 2~3회 자연스러운 선택 | production read 역할 분리 | 첫 주에 기존 습관보다 나은 순간이 없음 |

이 사용자를 확보하기 위해 `mycli`의 모든 설정과 기능을 복제하는 것은 적절하지 않다. 필요한 것은 power user라는 추상적 범주를 만족시키는 무제한 확장성이 아니라, 실제로 작업을 막는 소수의 integration point를 증거에 따라 여는 것이다. plugin system, scripting API, 광범위한 UI customization은 현재 범위를 다시 키우고 장기 호환성 의무를 만든다.

또한 single binary는 설치 마찰을 낮추지만 신뢰 그 자체는 아니다. database credential을 다루는 오픈소스 CLI에서는 source와 tag, release artifact의 관계, checksum, secret이 제거된 diagnostics가 adoption의 전제에 가깝다. 이는 기능 추가와 별개로 검증할 수 있는 distribution 품질이다.

초기 ICP 적합도는 중간 이상이다. 다만 완전한 `mycli` 대체가 아니라 **production read 전용 도구**로 명확한 역할을 획득할 때 가능성이 높다. 시험 설치는 비교적 쉽게 일어나지만 retention과 contribution은 첫 주의 실제 반복 사용, output 정확성, release 신뢰, maintainer의 scope 일관성을 통과한 뒤에만 발생한다.

### 검증 후보

다음 항목은 이 페르소나의 가정을 실제 사용자 인터뷰와 제품 테스트로 검증하기 위한 후보이다.

1. `mycli`와 표준 `mysql`을 병행하는 사용자가 각각을 선택하는 실제 상황과 빈도
2. 기존 CLI 사용자가 datavase를 자연스럽게 선택하게 되는 첫 반복 workflow가 무엇인지
3. grid, completion, history, pager 중 전환에 필수인 요소와 단순 선호인 요소의 구분
4. production read-only profile이 기존 read-only 계정·replica 위에서도 추가 adoption 동기가 되는지
5. reconnect, transaction, connection pool 변화 뒤 session read-only가 유지되고 확인 가능한지
6. NULL, empty string, binary, Unicode, multiline text, wide row, 큰 result set의 표시·copy·CSV 일치 여부
7. 실제 사용을 막은 설정 요청과 단지 익숙해서 원하는 customization을 구분하는 기준
8. config path, environment variable, keychain reference만으로 dotfiles workflow를 충족할 수 있는지
9. query history의 opt-in 여부, 저장 위치, permission, retention 정책에 대한 사용자 기대
10. source commit, release tag, binary, checksum 또는 provenance 연결 정보가 설치 의향에 미치는 영향
11. sample database와 재현 가능한 demo가 첫 시험 사용까지 걸리는 시간을 줄이는지
12. issue template와 secret-redacted diagnostic가 bug report 제출률과 품질을 높이는지
13. build·test·formatting 명령과 scope 원칙만 담은 짧은 contribution guide가 첫 patch를 촉진하는지
14. 기능 요청을 일관된 scope 근거로 거절하는 maintainer 반응이 신뢰와 이탈에 미치는 영향
15. 첫 7일 동안 필요한 실제 사용 횟수와 30일 retention 사이의 관계
16. datavase가 `mycli` 전체를 대체하는 경우와 production read 역할만 맡는 경우의 지속 사용률 차이
17. Homebrew 설치 후 삭제·업데이트·rollback 과정이 예측 가능하게 동작하는지
18. 사용자 기여가 없더라도 issue 확인, release cadence, changelog만으로 장기 유지 신뢰가 형성되는지

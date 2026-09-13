## Interview 15 — 박지민: bastion 없이 managed DB에 접속하는 클라우드 개발자

> 합성 페르소나 시뮬레이션이며 실제 사용자 인터뷰나 수요의 증거가 아니다.

### 페르소나

- 31세, Backend·Cloud Engineer, 경력 6년
- 개발자 18명 규모의 B2B SaaS 팀에서 Go와 Java 서비스 및 AWS 인프라를 함께 담당
- MySQL 호환 managed DB를 사용하며 public endpoint와 SSH bastion은 운영하지 않음
- 개발 DB는 로컬에서 직접 접속하고, staging·production은 회사 VPN과 DB proxy를 통해 접근
- 평소 IDE는 IntelliJ와 DataGrip, 터미널은 배포·로그·간단한 SQL 확인에 사용
- 로컬 터미널에서 `mysql`이나 사내 wrapper script를 실행하지만 서버에 SSH해 DB를 보는 workflow는 없음
- 운영 DB는 IAM 기반 단기 인증과 database-level read-only 계정을 사용
- 새 도구의 화면보다 인증 갱신, proxy 연결, TLS 설정이 기존 workflow와 맞는지를 먼저 평가

### 인터뷰 목적

SSH bastion이 없는 managed database 환경에서도 datavase의 terminal workflow가 의미가 있는지 확인한다. 특히 `ssh → mysql`이라는 기존 positioning이 이 사용자에게 전달되지 않을 때, 로컬 터미널 사용·VPN·proxy·단기 인증이 독립적인 adoption wedge를 만드는지 검증한다.

### 인터뷰

**Interviewer:** 최근 production 데이터베이스를 직접 확인했던 상황을 처음부터 설명해주세요.

**지민:** API 응답이 특정 고객에게만 늦다는 알람이 왔습니다. 회사 VPN이 연결돼 있는지 확인하고 사내 스크립트로 DB proxy 세션을 열었어요. 다른 터미널에서 IAM 토큰을 발급받은 다음 `mysql`로 접속해서 해당 고객의 job 상태를 조회했습니다. 원인을 찾고 나서는 proxy 프로세스와 터미널을 닫았습니다.

**Interviewer:** 서버에 SSH한 단계는 없었나요?

**지민:** 없습니다. 애플리케이션 서버도 직접 SSH를 허용하지 않아요. 로그는 observability 도구로 보고, DB는 로컬 노트북에서 VPN과 proxy를 거쳐 접속합니다.

**Interviewer:** 왜 DataGrip 대신 mysql CLI를 사용했습니까?

**지민:** 이미 터미널에서 proxy와 토큰을 열어둔 상태였고 확인할 값도 두세 개뿐이었어요. DataGrip에도 설정할 수 있지만 단기 토큰을 갱신하고 proxy port를 맞추는 과정이 매끄럽지 않습니다. 복잡한 분석이면 DataGrip을 쓰지만 장애 중 한두 쿼리는 CLI가 빠릅니다.

**Interviewer:** 이 workflow는 얼마나 자주 발생합니까?

**지민:** production은 한 달에 두세 번 정도입니다. staging은 일주일에 몇 번 보고요. 매일 쓰는 도구는 아닙니다.

**Interviewer:** 가장 불편한 단계는 무엇인가요?

**지민:** SQL 결과보다 접속 수명 관리입니다. VPN이 끊겼는지, proxy가 살아 있는지, 토큰이 만료됐는지에 따라 비슷한 접속 오류가 납니다. 어느 단계가 실패했는지 바로 알기 어렵습니다.

**Interviewer:** mysql 결과 화면은 불편하지 않습니까?

**지민:** 컬럼이 많으면 불편하죠. 다만 보통 필요한 컬럼만 골라 조회합니다. 결과 grid만 좋아지는 것은 설치 이유가 되지 않을 것 같아요.

**Interviewer:** datavase는 datasource를 저장하고 grid에서 결과를 확인할 수 있는 단일 binary입니다.

**지민:** 고정된 host와 password로 접속하는 환경이면 편할 수 있겠네요. 저희는 endpoint 앞에 proxy가 있고 credential이 짧게 살아서, 그 흐름을 지원하지 않으면 datasource를 저장해도 반쪽입니다.

**Interviewer:** datasource를 열기 전에 사용자가 별도 명령으로 proxy와 토큰을 준비하면요?

**지민:** 그러면 지금과 크게 다르지 않습니다. `mysql` 대신 다른 결과 화면을 쓰는 정도죠. 설치해서 가끔 쓰기에는 전환 이득이 약합니다.

**Interviewer:** `For the times you'd otherwise SSH in and run mysql`이라는 설명은 어떻게 들립니까?

**지민:** 제 상황은 아니라는 뜻으로 읽힙니다. SSH가 필요한 조직을 위한 제품이라고 생각하고 지나칠 것 같아요.

**Interviewer:** `A safer MySQL client for the terminal`은요?

**지민:** 무엇이 안전한지 먼저 궁금합니다. 저희는 이미 운영 계정 자체가 read-only이고 토큰도 만료됩니다. 클라이언트가 UPDATE를 막는 것보다 인증과 연결 상태를 틀리지 않게 다루는 게 중요해요.

**Interviewer:** datasource에 `read_only: true`를 설정하면 모든 connection을 session-level read-only로 시작합니다.

**지민:** 방어층이 하나 더 생기는 것은 좋습니다. 하지만 그것 때문에 도구를 바꾸지는 않을 것 같아요. DB 권한이 read-only인 계정에서는 중복이고, write가 필요한 계정은 애초에 별도 승인 절차로 씁니다.

**Interviewer:** session read-only가 현재 production 계정의 실제 권한을 대신한다고 오해할 가능성이 있습니까?

**지민:** 표현에 따라서는요. 화면에 read-only라고 보여도 DB 계정 권한이나 proxy 정책까지 보장하는 건 아니잖아요. session 설정과 server-side 권한은 구분해서 보여줘야 합니다.

**Interviewer:** CSV export는 가치가 있나요?

**지민:** 운영 데이터를 로컬 파일로 내보내는 건 저희 팀에서는 거의 금지에 가깝습니다. staging 데이터라면 가끔 쓰겠지만 DataGrip으로 충분합니다. CSV를 강하게 홍보하면 오히려 보안 정책을 확인하게 될 것 같아요.

**Interviewer:** 로컬 터미널을 이미 사용하니 TUI에는 잘 맞지 않나요?

**지민:** 터미널 친화성과 제품 적합성은 다른 문제 같습니다. 터미널을 쓰는 이유가 접속 도구들이 CLI로 제공되기 때문이지, DB 작업을 TUI로 옮기고 싶어서가 아니에요.

**Interviewer:** datavase가 proxy 프로세스를 직접 실행해주면 어떻습니까?

**지민:** 처음에는 편하겠지만 범위가 커집니다. AWS 방식, Cloud SQL Auth Proxy, Kubernetes port-forward처럼 팀마다 다르고 사내 wrapper도 있어요. 제품이 전부 내장하기보다는 접속 전후 hook이나 외부 명령 연동이 더 현실적일 수 있습니다.

**Interviewer:** 외부 명령 hook을 신뢰할 수 있을까요?

**지민:** 자동 실행되는 명령은 오히려 검토 대상입니다. config를 열었는데 임의 shell command가 실행되면 위험하죠. 명령과 인자를 명확히 보여주고, datasource별 허용과 timeout, 종료 처리가 있어야 합니다. 팀 config를 공유한다면 더 엄격해야 하고요.

**Interviewer:** credential을 environment variable로 주입할 수 있다면 충분합니까?

**지민:** 정적 password에는 충분할 수 있지만 IAM 토큰은 만료 시점이 있습니다. 도구가 시작할 때 한 번 읽고 reconnect에서 오래된 값을 재사용하면 실패합니다. 매 연결마다 credential provider를 호출하거나, 만료를 감지하고 다시 받아야 합니다.

**Interviewer:** 그 기능이 지원되면 mysql CLI를 대체할까요?

**지민:** 장애 대응 중 연결 실패 원인을 더 명확히 보여주고 토큰 갱신까지 안정적이면 시험할 수 있습니다. 하지만 월 몇 번 쓰는 production 작업만으로 습관이 바뀔지는 모르겠어요. staging에서 반복 사용해야 유지될 것 같습니다.

**Interviewer:** staging에서는 어떤 쿼리를 실행합니까?

**지민:** 배포 후 migration 결과를 확인하거나 비동기 job 상태를 봅니다. 로그에서 ID를 복사해 두세 번 조회하고 끝나요. 때로는 여러 replica나 schema 중 어디에 연결했는지 확인합니다.

**Interviewer:** 그 작업에서 datavase가 mysql보다 확실히 좋아질 수 있는 부분은 무엇입니까?

**지민:** 현재 datasource, proxy endpoint, database, token 만료 여부가 한 화면에 보이고 reconnect가 예측 가능하면 좋습니다. 결과 grid도 보조적으로 편하겠죠. 하지만 접속 준비를 그대로 두고 grid만 제공하면 큰 차이는 없습니다.

**Interviewer:** datasource를 선택하면 VPN 연결 여부까지 확인해야 할까요?

**지민:** 연결 실패 뒤에 일반적인 timeout만 보여주는 것보다는 낫습니다. 다만 VPN 제품을 직접 제어할 필요는 없고, endpoint reachability나 proxy port 상태를 구분해 알려주면 충분합니다.

**Interviewer:** 어떤 오류 구분이 필요합니까?

**지민:** DNS나 network unreachable, local proxy 미실행, TLS 실패, credential 만료, DB 권한 거부 정도입니다. 지금은 이런 문제가 모두 “접속 안 됨”처럼 보여서 여러 명령을 다시 실행합니다.

**Interviewer:** 연결 상태 진단이 제품의 핵심 기능이 되어야 한다고 보나요?

**지민:** 제 환경에는 grid보다 중요하지만 모든 사용자를 위해 제품이 cloud connectivity tool이 되어야 한다는 뜻은 아닙니다. 지원 범위가 불명확하면 신뢰를 잃을 수 있으니 작은 범위부터 검증해야 합니다.

**Interviewer:** SSH tunnel 지원은 이 환경에서 쓸모가 있습니까?

**지민:** 없습니다. 보안 정책상 bastion 자체가 없고 SSH key도 DB 접근에 사용하지 않아요. SSH 설정이 핵심 기능처럼 보이면 대상 사용자가 아니라고 판단할 겁니다.

**Interviewer:** 그렇다면 datavase가 해결하려는 문제가 존재하지 않는 건가요?

**지민:** 완전히 없지는 않습니다. mysql 출력, 쿼리 재사용, datasource context는 개선할 수 있어요. 다만 가장 큰 마찰은 DB client 바깥의 인증과 network lifecycle입니다. 그 부분을 건드리지 않으면 작은 편의 도구이고, 건드리면 제품 범위가 빠르게 커집니다.

**Interviewer:** 설치와 시험 사용을 결정하는 조건은 무엇인가요?

**지민:** Homebrew 한 줄, config에 secret을 저장하지 않아도 되는 것, 기존 proxy 명령과 충돌하지 않는 것, 실패해도 프로세스가 남지 않는 것입니다. 그리고 README에 동적 credential 지원 범위가 정확히 적혀 있어야 해요.

**Interviewer:** 사용을 중단하게 만드는 사건은 무엇입니까?

**지민:** 토큰 갱신 뒤 reconnect가 안 되거나, 종료 후 proxy가 남거나, TLS 옵션이 mysql CLI와 다르게 동작하는 경우입니다. 장애 중 한 번 실패하면 검증된 사내 script와 mysql로 바로 돌아갑니다.

**Interviewer:** 팀에 추천할 가능성은 있나요?

**지민:** staging에서 몇 주 안정적으로 쓴 뒤라면요. 개인적으로 grid가 마음에 든다는 이유로 production 도구를 추천하지는 않습니다. 인증 실패와 종료 동작을 재현하는 테스트가 있어야 합니다.

**Interviewer:** README demo에는 무엇을 보여줘야 합니까?

**지민:** SSH 장면 대신 이미 열린 local proxy를 통해 접속하고, 단기 credential이 갱신된 뒤 reconnect되는 장면이면 제 상황과 가깝습니다. 다만 모든 cloud를 지원하는 것처럼 보이면 안 됩니다. 외부에서 준비해야 하는 것과 datavase가 책임지는 범위를 명확히 보여주세요.

**Interviewer:** 지금 제품을 바로 설치하겠습니까?

**지민:** 호기심으로 설치할 수는 있지만 production workflow에 넣지는 않겠습니다. 현재 설명만 보면 SSH 사용자가 주 대상이고, 저희에게 필요한 proxy·credential lifecycle이 어디까지 지원되는지 알 수 없기 때문입니다.

### 결정적 발언

> “터미널을 쓰기는 하지만 SSH해서 mysql을 실행하는 사용자는 아닙니다.”

> “접속 준비를 그대로 두고 결과 grid만 바뀐다면 월 몇 번 쓰는 작업의 습관을 바꿀 이유가 약합니다.”

> “저희 환경에서 안전은 UPDATE 한 줄을 막는 것보다 proxy와 단기 credential의 상태를 틀리지 않게 다루는 것입니다.”

### Interview 15 결과

이 페르소나는 terminal 친화적이고 MySQL을 직접 조회하지만, `ssh → mysql` wedge에는 해당하지 않는다. managed database, VPN, local proxy, 단기 인증으로 구성된 workflow에서는 SSH tunnel과 session read-only의 차별성이 약하다. 특히 database-level read-only 계정을 이미 사용하는 조직에서는 client-side read-only가 유용한 추가 방어층이어도 adoption을 일으킬 핵심 동기가 되지 못한다.

동시에 terminal 사용 여부만으로 ICP를 판단해서는 안 된다는 반례를 제공한다. 사용자가 겪는 가장 큰 마찰은 결과 표현이 아니라 client에 도달하기 전후의 연결 lifecycle이다.

| 영역 | 현재 workflow | datavase의 잠재 가치 | 전환을 막는 조건 |
|---|---|---|---|
| 네트워크 접근 | 회사 VPN + local DB proxy | endpoint·proxy 상태 진단 | VPN·proxy를 단순 timeout으로만 표시 |
| 인증 | IAM 기반 단기 token | reconnect 시 credential 재취득 | 시작 시 읽은 만료 token 재사용 |
| DB 보호 | server-side read-only 계정 | session-level 추가 방어 | 기존 권한과 다른 안전 보장처럼 표현 |
| SQL 실행 | 로컬 mysql CLI | grid, datasource context | 접속 준비가 그대로 남아 전환 이득 부족 |
| SSH tunnel | 사용하지 않음 | 없음 | SSH 중심 메시지가 비대상 신호가 됨 |
| CSV export | production에서는 제한 | staging의 일시적 편의 | 보안 정책과 로컬 데이터 잔존 우려 |
| 장애 대응 | 검증된 사내 wrapper | 단계별 오류 구분 | proxy 누수, TLS 차이, 불안정한 reconnect |
| retention | staging에서 반복 확인 | production 전 신뢰 축적 | production 사용 빈도가 너무 낮음 |

따라서 SSH가 없는 환경을 즉시 지원 범위로 확장해야 한다는 결론은 아니다. cloud proxy와 credential provider를 폭넓게 내장하면 PR #82에서 줄인 범위를 다시 키울 위험이 있다. 우선 실제 사용자가 기존 외부 명령으로 연결을 준비한 뒤에도 grid와 datasource context만으로 전환하는지 확인해야 한다. 이 가치가 부족하다는 실제 증거가 모일 때만 제한된 hook 또는 credential provider interface를 검토하는 편이 적절하다.

초기 ICP로는 낮은 적합도다. 단, staging에서 로컬 CLI 조회 빈도가 높고 proxy·credential을 외부에서 안정적으로 준비할 수 있는 개발자는 보조 세그먼트가 될 수 있다. 이 경우 메시지는 SSH를 필수 전제로 만들지 않되, 지원하지 않는 cloud lifecycle까지 해결한다고 약속해서도 안 된다.

### 검증 후보

다음 항목은 이 페르소나의 가정을 실제 사용자 인터뷰와 제품 테스트로 검증하기 위한 후보이다.

1. managed MySQL 사용자가 production 접근에 SSH, VPN, local proxy, vendor connector 중 무엇을 실제로 사용하는지
2. SSH 없이 로컬 `mysql`을 실행하는 사용자의 주간·월간 조회 빈도와 staging·production 비중
3. DB client 실행 전 VPN·proxy·token 준비에 걸리는 시간과 실패 빈도
4. 결과 grid와 datasource context만으로 기존 mysql CLI에서 전환할 사용자가 존재하는지
5. database-level read-only 계정을 이미 사용하는 조직에서 session read-only가 추가 adoption 동기가 되는지
6. 단기 credential이 최초 연결, reconnect, idle timeout 이후에 안전하게 재취득되는지
7. network, proxy, TLS, credential, DB 권한 오류를 사용자가 구별할 수 있는 진단 방식
8. 외부 pre-connect command hook에 필요한 승인, 표시, timeout, 종료, config 공유 안전 조건
9. datavase 종료와 비정상 종료 뒤 local proxy 또는 child process가 남지 않는지
10. SSH 중심 문구와 SSH를 전제하지 않는 문구가 README 이탈률 및 설치 의향에 미치는 차이
11. staging에서 반복 사용한 경험이 낮은 production 사용 빈도를 보완해 retention으로 이어지는지
12. cloud integration 범위를 늘리지 않고 기존 proxy와 조합하는 문서만으로 adoption이 가능한지


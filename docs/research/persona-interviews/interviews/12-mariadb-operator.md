# Interview 12 — 박성진: 여러 세대의 MariaDB를 운영하는 데이터베이스 운영자

> 합성 페르소나 시뮬레이션이며 실제 사용자 인터뷰나 수요의 증거가 아니다.

### 페르소나

- 41세, 중견 커머스 기업 Database Operations Engineer, 경력 15년
- 주문·정산·회원 시스템의 MariaDB 10.3, 10.6, 10.11 클러스터를 운영하고 일부 레거시 MySQL 5.7도 담당
- Galera Cluster, MaxScale, 비동기 replication이 시스템별로 혼재하며 유지보수 일정 때문에 버전을 한꺼번에 통일하지 못함
- 평상시에는 HeidiSQL과 사내 모니터링 도구를 사용하고, bastion·컨테이너·장애 복구 환경에서는 `mariadb` 또는 `mysql` CLI를 사용
- 운영 계정은 권한을 분리하지만, 복제 상태·세션 변수·metadata를 확인하려면 일반적인 read-only 업무보다 넓은 권한이 필요할 때가 있음
- 새 DB 클라이언트의 UI보다 지원 버전, TLS·인증 방식, MariaDB 고유 문법과 결과 타입의 정확성을 먼저 확인
- “MySQL 호환”이라는 설명을 신뢰하지 않으며, 지원 범위와 실패 방식이 검증 가능해야 production 도구로 승인함

### 인터뷰 목적

여러 MariaDB 버전과 운영 기능을 다루는 사용자에게 datavase의 MariaDB 지원이 실제 workflow를 대체할 만큼 깊은지 탐색한다. 단순한 접속 성공과 기본 SELECT를 넘어 MariaDB 고유 문법, metadata, 타입 표현, TLS·인증, session read-only 동작과 버전별 차이가 신뢰 형성에 어떤 영향을 주는지 확인한다.

### 인터뷰

**Interviewer:** 최근 터미널에서 운영 데이터베이스에 접속했던 상황을 처음부터 설명해주세요.

**성진:** 새벽에 주문 DB replica 지연 알람이 왔습니다. VPN을 켜고 bastion에 들어간 뒤 `mariadb` 클라이언트로 replica에 접속했어요. `SHOW SLAVE STATUS` 결과와 processlist를 보고, primary와 replica의 GTID 위치를 비교했습니다. 원인은 오래 실행된 리포트 쿼리였고 세션을 종료한 뒤 지연이 줄어드는 것을 확인했습니다.

**Interviewer:** 그때 GUI 도구를 쓰지 않은 이유는 무엇입니까?

**성진:** 장애 대응은 이미 터미널에서 시작합니다. 모니터링 로그, SSH, 프로세스 확인을 오가는데 GUI를 새로 열 이유가 없어요. 그리고 보안망에 따라 로컬 PC에서 DB까지 직접 연결할 수 없는 시스템도 있습니다.

**Interviewer:** `mariadb` CLI에서 가장 불편했던 부분은 무엇이었나요?

**성진:** `SHOW SLAVE STATUS`처럼 컬럼이 많은 결과가 읽기 어렵습니다. `\G`로 세로 출력하면 볼 수 있지만 두 노드 값을 비교하기는 불편해요. 긴 query 결과를 복사하거나 장애 보고서용으로 정리하는 데도 시간이 듭니다.

**Interviewer:** 결과를 grid로 보여주는 terminal client라면 사용하고 싶습니까?

**성진:** 화면은 편하겠지만 그것만으로 운영 도구를 바꾸지는 않습니다. 지금 CLI는 못생겼어도 서버와 같은 패키지로 배포되고, 어떤 버전에서 어떻게 동작하는지 알고 있거든요. 새로운 grid가 값을 잘못 표시하면 편리함보다 위험이 큽니다.

**Interviewer:** 어떤 값이 잘못 표시될 가능성을 걱정합니까?

**성진:** `DECIMAL` 정밀도, 큰 `BIGINT`, binary 값, `NULL`과 빈 문자열 구분, microsecond가 있는 datetime, timezone이 개입된 값부터 봅니다. JSON도 MariaDB와 MySQL의 구현이 같지 않고요. 장애 때는 한 글자 차이로 판단이 달라질 수 있습니다.

**Interviewer:** datavase는 MySQL과 MariaDB를 지원한다고 설명합니다. 그 문구로 충분합니까?

**성진:** 전혀요. MariaDB 몇 버전부터 몇 버전까지인지, 어떤 driver로 시험했는지, TLS와 인증 plugin은 무엇을 지원하는지 알아야 합니다. “MySQL-compatible이니까 MariaDB도 된다”는 식이면 테스트 환경에도 등록하지 않을 겁니다.

**Interviewer:** 왜 호환성 설명에 그렇게 회의적입니까?

**성진:** 애플리케이션에서도 여러 번 겪었습니다. 기본 CRUD는 되다가 metadata query, JSON 함수, collation, `RETURNING`, sequence 같은 곳에서 차이가 납니다. 클라이언트도 접속과 `SELECT 1`이 된다는 것과 운영 업무를 지원한다는 것은 다른 말입니다.

**Interviewer:** 현재 동시에 운영하는 버전은 어떻게 다릅니까?

**성진:** 오래된 정산 시스템은 10.3이고, 주요 주문 시스템은 10.6, 신규 시스템은 10.11입니다. upgrade를 순차 진행하지만 vendor 제품과 replication 구성 때문에 몇 년 동안 여러 세대가 공존합니다. 그래서 최신 버전 하나에서만 확인한 도구는 의미가 없습니다.

**Interviewer:** 새 클라이언트를 검토할 때 처음 무엇을 실행합니까?

**성진:** 별도 sandbox에서 접속, TLS 정보, `SELECT VERSION()`, charset과 collation, session 변수를 확인합니다. 그다음 타입이 다양한 fixture table을 조회하고 `SHOW CREATE TABLE`, `SHOW INDEX`, processlist, replication 관련 명령을 봅니다. client가 끊긴 뒤 재접속과 timeout도 시험하고요.

**Interviewer:** datavase는 datasource를 선택하고 테이블을 확인한 뒤 SQL을 실행하고 결과를 저장하는 작은 범위에 집중합니다.

**성진:** 범위를 작게 잡는 것은 나쁘지 않습니다. 다만 “테이블 확인”이 `information_schema`를 어떻게 읽는지에 따라 버전 차이가 생길 수 있어요. 지원하지 않는 운영 명령이 있다면 없다고 명확히 써야 합니다. 작은 제품이라는 이유로 호환성 정의까지 작아져서는 안 됩니다.

**Interviewer:** DBA용 monitoring이나 schema editing은 제공하지 않습니다.

**성진:** 그러면 제 주 도구는 아닙니다. 장애 대응에서 processlist나 replication 상태를 보기 어렵다면 공식 CLI를 계속 써야 해요. 하지만 bastion에서 데이터를 조회하고 CSV로 정리하는 보조 도구로는 검토할 수 있습니다.

**Interviewer:** 일반 SQL editor에서 `SHOW PROCESSLIST`나 MariaDB 고유 명령을 직접 실행할 수 있다면요?

**성진:** 실행된다는 것만으로 부족합니다. 결과 column이 버전에 따라 달라도 깨지지 않는지, 긴 값이 잘리면 원문을 확인할 수 있는지 봐야 합니다. 클라이언트가 지원하지 않는 SQL이라고 미리 차단해서도 안 되고요.

**Interviewer:** SQL completion이 MySQL 문법을 기준으로 동작한다면 문제가 됩니까?

**성진:** 실행을 막지 않으면 치명적이지는 않습니다. 하지만 MariaDB에서 가능한 문법에 계속 오류 표시가 나오거나 MySQL 전용 문법을 추천하면 신뢰가 빠르게 떨어집니다. completion을 끌 수 있어야 하고, 지원 수준을 과장하면 안 됩니다.

**Interviewer:** production datasource에 `read_only: true`를 설정하면 모든 connection을 session-level read-only로 시작합니다.

**성진:** 정확히 어떤 statement를 실행하는지부터 확인하겠습니다. MariaDB 버전별로 지원되는 session variable과 권한 요구가 다를 수 있고, transaction read-only가 모든 변경을 막는 것도 아닙니다. temporary table이나 administrative statement까지 어떤 행동을 하는지 검증해야 해요.

**Interviewer:** 이 기능은 access control이 아니라 accidental write를 줄이는 guardrail입니다.

**성진:** 그렇게 표현하면 낫습니다. 하지만 UI에 read-only라고 표시하려면 실제 session 상태를 조회해서 보여줘야 합니다. 설정 파일에 `true`가 있었다는 사실만 표시하면 reconnect나 proxy 전환 뒤 실제 상태와 어긋날 수 있습니다.

**Interviewer:** 운영팀에서는 이미 DB 계정 권한을 분리한다고 했는데, client-side read-only가 추가 가치를 줍니까?

**성진:** 데이터 조회 계정에는 조금 줍니다. 사람이 잘못된 datasource를 선택해도 UPDATE가 한 번 더 막히니까요. 하지만 운영 계정은 replication 관리나 세션 종료 때문에 더 넓은 권한이 필요합니다. 그 계정에서 read-only 표시가 “안전”을 보증하는 것처럼 보이면 오히려 위험합니다.

**Interviewer:** MariaDB Galera 환경에서는 다른 고려가 있습니까?

**성진:** 어느 node에 연결됐는지 중요합니다. MaxScale endpoint 하나로 들어가도 실제 backend가 바뀔 수 있어요. `@@hostname`, server id, version, read-only 관련 실제 변수를 화면에서 확인할 수 있어야 합니다. datasource 이름만 보고 primary나 replica를 단정하면 안 됩니다.

**Interviewer:** 접속 화면에 host와 database, read-only 상태를 계속 표시하면 충분할까요?

**성진:** 직접 접속이면 기본은 됩니다. proxy나 Galera에서는 입력한 host와 실제 server가 다릅니다. 표시할 수 없다면 적어도 연결 후 확인 query를 쉽게 실행하고 결과를 고정해볼 수 있어야 해요.

**Interviewer:** TLS와 인증에서는 무엇을 확인합니까?

**성진:** CA 검증, client certificate, hostname 검증, 암호화 강제 여부입니다. “TLS 지원”만 써 있고 검증을 끈 상태로도 자물쇠가 표시되면 안 됩니다. 일부 환경은 unix socket을 쓰고, LDAP 연계나 특정 authentication plugin을 쓰기도 하니 미지원 조합은 명시해야 합니다.

**Interviewer:** 지원하지 않는 인증 조합에서 일반적인 connection error만 보여주면 어떻습니까?

**성진:** 원인을 찾는 데 시간을 쓰고 공식 CLI로 돌아갑니다. 서버가 거부한 것인지 client가 capability를 구현하지 않은 것인지 구분돼야 issue를 보고할 수 있어요. production 대응 중에는 한 번 실패하면 다시 시도하지 않을 가능성이 큽니다.

**Interviewer:** binary 하나로 설치된다는 점은 어떤 가치가 있습니까?

**성진:** bastion이나 임시 진단 컨테이너에 배포하기는 편합니다. 다만 외부 binary 반입 승인과 checksum 검증이 필요하고, 지원하는 CPU와 libc 조건도 봅니다. 공식 release artifact와 reproducible한 버전 정보가 있어야 해요.

**Interviewer:** Homebrew 설치만 제공해도 충분합니까?

**성진:** 개인 Mac에서 시험하기에는 충분합니다. 실제 운영망은 Linux이고 package manager 접근이 막혀 있을 수 있습니다. 정적 binary, checksum, signature, release note가 더 중요합니다.

**Interviewer:** 결과를 CSV로 저장하는 기능은 사용합니까?

**성진:** 장애 후 분석이나 데이터 요청 대응에서 씁니다. 하지만 CSV가 원본 타입을 보존한다고 생각하지는 않습니다. delimiter, newline, quote, character set, `NULL`, binary 값 처리 규칙이 문서화돼야 하고 Excel에서 큰 숫자가 변형되는 것도 조심해야 합니다.

**Interviewer:** 화면에 보이는 값을 그대로 export하면 자연스럽지 않습니까?

**성진:** 화면은 가독성을 위해 줄이거나 포맷할 수 있지만 export는 원래 값을 기대합니다. 화면에서 `1.20`으로 보인 값과 실제 DECIMAL scale, 긴 text의 생략 여부를 구분해야 합니다. export가 display layer를 통과하는지 raw result를 쓰는지 알고 싶습니다.

**Interviewer:** clipboard copy도 같은 문제입니까?

**성진:** 네. 셀 하나 복사했는데 개행이 사라지거나 binary가 Unicode로 손상되면 안 됩니다. 편의 기능일수록 사용자가 검증 없이 붙여넣기 쉬워서 정확성 요구가 더 높습니다.

**Interviewer:** 지원 버전별 integration test 결과를 공개하면 신뢰에 도움이 될까요?

**성진:** 가장 도움이 됩니다. MariaDB 10.3, 10.6, 10.11과 현재 지원 버전을 container로 띄워 접속·metadata·타입·TLS·read-only·export를 시험했다는 표가 있으면 직접 검증할 출발점이 됩니다. 단순 badge보다 어떤 assertion을 했는지가 중요하고요.

**Interviewer:** CI matrix가 모두 통과한다면 바로 production에서 사용하겠습니까?

**성진:** 아니요. 먼저 staging replica에서 실제 schema와 query로 봅니다. CI는 개발자가 차이를 알고 관리한다는 신호이지 우리 환경의 증명은 아닙니다. 그래도 matrix가 없으면 검토를 시작하지 않을 가능성이 큽니다.

**Interviewer:** compatibility 문제가 발견됐을 때 무엇을 기대합니까?

**성진:** 지원 범위를 숨기지 않고 issue template에서 server version, client version, charset, TLS, proxy 여부를 받았으면 합니다. regression이면 어느 release부터 발생했는지 찾을 수 있어야 하고요. “MariaDB는 MySQL과 같아서 재현되지 않는다”는 답을 받으면 제품 전체를 신뢰하지 않습니다.

**Interviewer:** 기능을 하나 추가할 수 있다면 무엇을 원합니까?

**성진:** 기능보다 compatibility contract를 먼저 원합니다. 지원 버전과 시험 항목을 문서로 고정하고 release마다 matrix를 돌리는 겁니다. 그다음이라면 현재 session과 실제 server identity를 보여주는 진단 정보가 좋겠습니다.

**Interviewer:** DataGrip이나 HeidiSQL을 대체할 수 있을까요?

**성진:** 제 데스크톱 작업은 대체하지 못합니다. schema 비교, 여러 결과 탭, 관리 기능이 필요하니까요. 다만 GUI가 없는 서버에서 공식 CLI보다 결과를 읽고 반출하기 편한 보조 client가 될 수는 있습니다.

**Interviewer:** 공식 `mariadb` CLI 대신 기본 도구가 될 가능성은 있습니까?

**성진:** 일반 조회에는 가능합니다. 그러나 미지원 문법, 인증, datatype을 한 번이라도 조용히 잘못 처리하면 공식 CLI로 돌아갑니다. 명확하게 실패하는 것은 고칠 수 있지만 틀린 값을 그럴듯하게 보여주는 것은 받아들일 수 없습니다.

**Interviewer:** README demo에서 무엇을 보고 싶습니까?

**성진:** 예쁜 grid나 UPDATE 차단 장면보다 실제 MariaDB 버전이 표시되고, `NULL`, 큰 DECIMAL, multiline text를 정확히 보여준 뒤 CSV로 저장하는 장면이 더 설득력 있습니다. 아래에는 시험한 버전 matrix 링크가 있어야 하고요.

**Interviewer:** 현재 설명만으로 설치할 의향이 있습니까?

**성진:** 개인 sandbox에서는 호기심으로 설치할 수 있습니다. production 검토는 별개입니다. 지원 matrix, TLS 동작, 사용 driver, 로컬에 저장되는 정보, 알려진 제한을 읽고 직접 fixture test를 통과해야 후보가 됩니다.

### 결정적 발언

> “접속과 `SELECT 1`이 된다는 것과 MariaDB를 지원한다는 것은 다른 말입니다.”

> “명확하게 실패하는 것은 고칠 수 있지만, 틀린 값을 그럴듯하게 보여주는 것은 받아들일 수 없습니다.”

> “작은 제품이라는 이유로 호환성 정의까지 작아져서는 안 됩니다.”

### Interview 12 결과

MariaDB 운영자에게 datavase의 single binary와 가독성 높은 결과 화면은 GUI가 없는 bastion·진단 환경에서 보조 도구가 될 가능성을 만든다. 그러나 이 사용자는 단순 접속 성공이나 MySQL과의 프로토콜 호환성을 MariaDB 지원의 증거로 인정하지 않는다.

핵심 전환 조건은 기능 수가 아니라 **버전과 기능별 compatibility contract를 검증할 수 있는가**이다. MariaDB 10.3·10.6·10.11처럼 여러 세대가 공존하고, Galera·MaxScale·replication·TLS·다양한 인증이 결합된 환경에서는 지원 범위를 모호하게 표현할수록 신뢰가 낮아진다.

| 영역 | 필요한 신뢰 근거 | 즉시 이탈하는 실패 |
|---|---|---|
| 지원 버전 | MariaDB 버전별 CI·integration matrix | “MySQL-compatible”이라는 포괄적 주장만 제공 |
| 결과 정확성 | DECIMAL, BIGINT, NULL, binary, datetime, multiline fixture 검증 | 값의 조용한 변형·잘림·오표시 |
| metadata·SQL | 버전별 schema 조회와 임의 SQL 통과 보장 | MariaDB 고유 문법을 client가 차단 |
| read-only | 연결 후 실제 session 상태 확인 | config 값만 보고 보호 상태로 표시 |
| server context | 실제 version·node·session 정보를 확인할 방법 | proxy endpoint를 실제 backend로 오인 |
| TLS·인증 | CA·hostname·client certificate 및 plugin별 지원표 | 미검증 연결을 안전한 TLS로 표시 |
| CSV·clipboard | raw value, NULL, encoding, quoting 처리 규칙 | display용 포맷이나 생략값을 그대로 반출 |
| 배포 | 정적 binary, checksum, signature, release note | 검증할 수 없는 artifact 또는 불명확한 platform 조건 |
| 장애 대응 | server와 client 오류의 구분, 재현 정보 안내 | 미지원 기능을 일반 connection error로 숨김 |

이 페르소나는 datavase의 핵심 ICP라기보다 **신뢰 경계와 호환성 품질을 드러내는 검증자**에 가깝다. 관리 기능이 없는 현재 범위로는 주력 DBA 도구를 대체하지 못하지만, 반복되는 terminal 조회·결과 반출에는 제한된 적합성이 있다. 이 집단을 겨냥해 DBA 기능을 확장하는 것보다, 현재 약속한 조회 workflow가 MariaDB에서 정확히 동작한다는 증거를 만드는 편이 제품 방향과 더 잘 맞는다.

또한 read-only의 의미를 MySQL과 MariaDB에 동일하게 추정해서는 안 된다. UI의 보호 표시는 datasource 설정이 아니라 실제 connection의 session 상태를 반영해야 하며, proxy 재연결이나 server 전환 뒤에도 다시 검증돼야 한다. 이것은 새로운 관리 기능 요구가 아니라 현재 제품이 내세우는 안전성 주장에 필요한 정확성 조건이다.

### 실제 사용자 검증 후보

1. 실제 MariaDB 사용자가 운영 중인 major·minor 버전 분포와 동시에 유지하는 버전 수를 조사한다.
2. MariaDB 10.3, 10.6, 10.11 및 현재 지원 release에서 접속·schema 탐색·임의 SQL·reconnect integration matrix를 실행한다.
3. DECIMAL scale, unsigned BIGINT, NULL, empty string, BLOB, UTF-8·multibyte text, microsecond datetime, multiline text의 화면·copy·CSV 값을 원본과 대조한다.
4. MariaDB 고유 JSON 동작, sequence, `RETURNING`, `SHOW CREATE TABLE`, `SHOW INDEX`, processlist와 replication 관련 결과가 client 가정 없이 표시되는지 확인한다.
5. 각 지원 버전에서 session-level read-only 설정 statement, 필요한 권한, 허용·거부되는 write 종류를 실제 계정으로 검증한다.
6. direct connection, MaxScale, Galera node 전환과 reconnect 이후 datasource 설정과 실제 server·session 상태가 어긋나는지 시험한다.
7. CA 검증, hostname 검증, client certificate, TLS 강제, unix socket과 실제 사용 authentication plugin별 지원·오류 메시지를 확인한다.
8. compatibility failure가 silent coercion, UI corruption, 명확한 오류 중 어떤 형태로 나타나는지 관찰하고 silent failure를 release blocker로 정의한다.
9. 공식 `mariadb` CLI와 datavase로 같은 query·export를 수행하게 한 뒤 정확성, 완료 시간, fallback 이유를 비교한다.
10. README의 일반적인 MariaDB 지원 문구와 버전·기능 matrix를 제시했을 때 운영자의 설치·staging 검토 의향이 실제로 달라지는지 확인한다.
11. release artifact의 checksum·signature·platform 조건과 dependency 정보를 운영망 반입 담당자가 검토할 수 있는지 확인한다.
12. 실제 MariaDB bug report에서 server version, proxy, charset, TLS, client version 정보가 재현에 충분한지 issue workflow를 시험한다.

---

## Interview 11 — 박서연: PostgreSQL-first 제품 개발자

> 합성 페르소나 시뮬레이션이며 실제 사용자 인터뷰나 수요의 증거가 아니다.

### 페르소나

- 31세, B2B SaaS Backend Engineer, 경력 7년
- 현재 회사와 직전 회사 모두 PostgreSQL을 기본 데이터베이스로 사용
- 로컬·개발 환경은 Docker Compose, 운영 환경은 AWS RDS PostgreSQL과 Kubernetes로 구성
- 평소에는 IntelliJ Database Tools와 `psql`을 사용하고, 터미널 자동완성이 필요할 때 `pgcli`도 사용
- 장애 대응 시 `kubectl`, 로그 검색, `psql`을 같은 터미널 세션에서 오가며 데이터 확인
- JSONB, array, enum, `RETURNING`, CTE, materialized view, `EXPLAIN (ANALYZE, BUFFERS)` 등 PostgreSQL 고유 기능을 일상적으로 사용
- 운영 DB의 사람 계정은 조회 전용 role을 기본으로 부여하며, 쓰기 작업은 별도 승격과 승인 절차로 수행
- MySQL은 과거 프로젝트에서 잠깐 사용했지만 최근 4년 동안 직접 운영한 경험은 없음
- 새로운 터미널 도구를 시험할 권한은 있으나, 기존 `psql` workflow보다 명확한 이점이 없으면 유지하지 않음

### 인터뷰 목적

PostgreSQL을 중심으로 일하는 개발자가 datavase의 문제 정의에는 공감하지만 MySQL·MariaDB 전용이라는 이유로 사용 대상에서 제외되는지 확인한다. PostgreSQL 지원 요구가 실제 제품 확장의 근거인지, 단순히 현재 ICP 밖 사용자의 자연스러운 요구인지 구분하고, 지원 범위를 넓힐 경우 유지해야 할 최소 품질과 기회비용을 탐색한다.

### 인터뷰

**Interviewer:** 최근 운영 데이터베이스에 직접 접속했던 상황을 처음부터 설명해주세요.

**서연:** 새 배포 이후 특정 고객사의 문서 검색이 느려졌다는 알림이 왔습니다. Grafana에서 tenant id와 느린 API를 확인하고, pod 로그에서 query 조건을 찾았어요. 같은 터미널에서 조회 전용 계정으로 `psql`에 접속해 대상 row 수와 실행 계획을 봤습니다. `EXPLAIN (ANALYZE, BUFFERS)` 결과에서 JSONB 조건이 예상한 index를 타지 않는 것을 확인했고요.

**Interviewer:** 그 작업에서 GUI 대신 `psql`을 사용한 이유는 무엇입니까?

**서연:** 이미 `kubectl`과 로그를 터미널에서 보고 있었고 query도 정해져 있었습니다. GUI를 열고 VPN과 connection을 다시 확인하는 것보다 명령 하나가 빨랐어요. 실행 계획 원문을 이슈에 붙이기도 편하고요.

**Interviewer:** `psql`에서 가장 불편한 점은 무엇인가요?

**서연:** 폭이 넓은 결과는 읽기 어렵고 `\x`를 켰다 껐다 해야 합니다. 여러 결과를 비교하거나 일부 cell만 복사할 때도 GUI보다 불편해요. 접속 문자열과 SSL 옵션을 기억하는 것도 귀찮지만 shell function으로 어느 정도 해결했습니다.

**Interviewer:** 그 불편 때문에 다른 터미널 클라이언트를 찾은 적이 있습니까?

**서연:** `pgcli`를 한동안 썼습니다. 자동완성은 좋았는데 긴급 상황에는 결국 기본 `psql`로 돌아왔어요. 어느 서버에도 있고 동작을 예측할 수 있다는 점이 큽니다. 결과가 예쁘다는 이유만으로 새 도구가 정착하지는 않았습니다.

**Interviewer:** 현재 접속 workflow를 바꿀 만큼 반복되는 문제는 무엇입니까?

**서연:** 회사별로 접속 명령과 tunnel 방식이 달라지는 문제는 반복됩니다. 다만 지금 회사에서는 SSM과 wrapper script로 표준화돼 있어요. 개인 클라이언트가 datasource를 관리하기보다 플랫폼 팀이 만들어준 명령을 실행하는 편이 안전합니다.

**Interviewer:** datavase는 datasource를 저장하고 테이블을 확인하며 SQL을 실행하고 결과를 복사하거나 CSV로 저장하는 터미널 클라이언트입니다. 첫인상은 어떤가요?

**서연:** 제가 불편하다고 말한 결과 탐색에는 맞아 보입니다. 하지만 어떤 데이터베이스를 지원하는지가 먼저예요. PostgreSQL이 아니면 제 workflow에서는 평가할 수 없습니다.

**Interviewer:** 현재는 MySQL과 MariaDB만 지원합니다.

**서연:** 그러면 설치하지 않습니다. 기능이 부족해서가 아니라 접속할 대상이 없어요. README에서 MySQL 전용이라고 명확히 쓰는 편이 서로 시간을 아껴줍니다.

**Interviewer:** PostgreSQL을 지원한다면 사용해볼 의향이 있습니까?

**서연:** “지원”의 범위에 따라 다릅니다. 접속과 단순 SELECT만 된다는 뜻이면 잠깐 demo는 볼 수 있어도 실제 장애 대응에는 쓰지 않을 겁니다. PostgreSQL syntax와 type, SSL, schema 검색 경로, 실행 계획이 제대로 동작해야 합니다.

**Interviewer:** 단순 조회가 주 용도라면 PostgreSQL 고유 기능이 모두 필요할까요?

**서연:** 단순 조회에도 고유 기능이 나옵니다. JSONB 연산자, array, `::type` cast, `ILIKE`, enum, interval을 자주 써요. 결과 렌더링도 bytea, timestamp with time zone, JSONB를 정확히 처리해야 하고요. dialect는 고급 기능을 쓸 때만 필요한 것이 아닙니다.

**Interviewer:** datavase의 SQL editor가 PostgreSQL 문법을 그대로 서버에 전달하고 결과만 표시한다면요?

**서연:** 첫 단계는 될 수 있습니다. 그래도 statement 분리, dollar-quoted string, multiline function, parameter 표시 같은 곳에서 parser가 개입하면 깨질 수 있어요. 제품이 SQL을 분석하지 않는다면 그 범위를 문서로 명확히 해야 합니다.

**Interviewer:** 운영 datasource에 `read_only: true`를 설정해 session을 read-only로 만들 수 있습니다. 이 기능은 가치가 있습니까?

**서연:** 방향은 이해하지만 저희는 운영 사람 계정 자체에 SELECT 권한만 줍니다. 클라이언트 설정은 실수로 해제되거나 우회될 수 있어서 권한 통제를 대신하지 못하죠. 계정 분리가 어려운 작은 팀에는 도움이 될 수 있지만 저희에게는 보조 표시 정도입니다.

**Interviewer:** PostgreSQL에서도 session-level read-only를 제공한다면 차별점이 될까요?

**서연:** 그것만으로는 아닙니다. PostgreSQL에는 `default_transaction_read_only`가 있지만 superuser나 충분한 권한을 가진 사용자는 바꿀 수 있고, sequence 같은 동작도 직관과 다를 수 있어요. “안전한 PostgreSQL 클라이언트”라고 부르려면 어떤 statement가 막히고 무엇은 막히지 않는지 정확해야 합니다.

**Interviewer:** 현재 `psql` 접속이 조회 전용 role이라면 datavase의 read-only는 중복입니까?

**서연:** 네, 대부분 중복입니다. 화면에 현재 role과 database, primary인지 replica인지 보여주는 것은 유용하지만 보호는 서버 권한에서 나와야 합니다. 클라이언트 read-only 때문에 도구를 바꾸지는 않을 것 같아요.

**Interviewer:** CSV export와 cell·row 복사는 어떻습니까?

**서연:** cell 복사는 `psql`보다 편할 수 있습니다. CSV는 `\copy`가 이미 있고 자동화에도 적합해서 새 기능은 아니에요. grid에서 조건을 확인한 뒤 몇 행을 빠르게 전달하는 경험이 매끄럽다면 편의성은 있겠죠.

**Interviewer:** `psql`의 `\copy`보다 UI export가 더 나은 상황은 언제입니까?

**서연:** query를 탐색하다가 결과가 맞는지 보고 즉시 저장할 때요. 반면 정기 추출이나 큰 결과는 CLI 명령과 script가 낫습니다. export가 전부 memory에 올라가거나 row 수를 숨기면 오히려 위험합니다.

**Interviewer:** single static binary라는 점은 설치 결정에 영향을 줍니까?

**서연:** 긍정적이지만 충분하지 않습니다. 개인 노트북에는 쉽게 설치할 수 있어도 bastion이나 production pod에는 승인되지 않은 binary를 올리지 않습니다. 로컬에서 tunnel을 통해 접속할 수 있어야 하고, 회사 보안 검토에서는 release provenance와 업데이트 방식도 봅니다.

**Interviewer:** PostgreSQL 지원이 추가됐다고 가정하면 가장 먼저 무엇을 시험하겠습니까?

**서연:** SSL이 필요한 RDS와 local Docker 양쪽 연결, 여러 schema 탐색, JSONB와 array 결과, timestamp timezone 표시, 취소 동작을 봅니다. 그다음 큰 result set을 열었을 때 memory와 terminal이 버티는지 확인할 거예요.

**Interviewer:** query cancellation이 왜 중요합니까?

**서연:** 운영에서 잘못된 조건으로 큰 scan을 시작했을 때 UI가 멈추지 않고 서버 query까지 취소돼야 합니다. 화면만 닫히고 backend query가 계속 돌면 사용할 수 없어요. PostgreSQL backend PID와 cancel request가 정확히 처리되는지를 확인하겠습니다.

**Interviewer:** 테이블 목록과 SQL 실행, 결과 반출만 지원하고 실행 계획 UI는 없다면요?

**서연:** 조회 업무의 일부에는 쓸 수 있지만 장애 분석 도구로는 부족합니다. 다만 제품이 `psql` 전체를 대체한다고 주장하지 않고 “빠른 inspection”으로 한정한다면 이해할 수 있어요. 저는 실행 계획이 필요한 순간에는 바로 `psql`이나 IDE로 돌아갈 겁니다.

**Interviewer:** PostgreSQL 지원을 요청하는 사용자가 많다면 추가하는 것이 맞을까요?

**서연:** 요청 수만 보면 안 됩니다. GitHub에서 “Postgres도 되나요?”라고 묻는 사람과, 매주 터미널로 운영 DB를 조회하면서 실제로 도구를 바꿀 사람은 다르니까요. 지원 후에도 MySQL 사용자와 같은 핵심 workflow를 반복하는지 봐야 합니다.

**Interviewer:** 어떤 증거가 있으면 PostgreSQL 확장이 정당하다고 보십니까?

**서연:** 실제 PostgreSQL 사용자 인터뷰에서 `psql` 결과 탐색과 datasource 전환 문제가 반복되고, 제한된 prototype을 업무에 여러 번 쓰는 행동이 나와야 합니다. 그리고 PostgreSQL을 넣어도 MySQL 안정화 속도가 떨어지지 않는다는 개발 비용 판단이 있어야겠죠.

**Interviewer:** PostgreSQL 사용자가 많다는 시장 규모 자체는 충분한 근거가 아닙니까?

**서연:** 아닙니다. 큰 시장과 이 제품이 해결하는 좁은 문제가 겹친다는 보장은 없어요. PostgreSQL 사용자 대부분이 GUI나 managed query tool에 만족할 수도 있고, 저처럼 terminal을 써도 `psql` 신뢰성을 더 중요하게 볼 수 있습니다.

**Interviewer:** MySQL과 PostgreSQL을 함께 쓰는 팀이라면 하나의 클라이언트가 더 매력적이지 않을까요?

**서연:** 그 팀에는 가능성이 있습니다. 같은 datasource UI와 export workflow를 공유할 수 있으니까요. 하지만 database별 기능을 얕게 지원하면 결국 중요한 순간마다 다른 도구를 써야 해서 통합의 장점이 줄어듭니다.

**Interviewer:** 공통 기능만 유지하고 database별 고급 기능은 제공하지 않는 전략은 어떻습니까?

**서연:** 제품 경계로는 일관적입니다. 다만 공통 기능의 정의가 생각보다 어렵습니다. schema, type, transaction, read-only, cancellation이 database마다 다르니까요. “네 가지 기능만”이라는 UI 범위와 “두 엔진을 신뢰성 있게 지원”하는 구현 범위는 별개입니다.

**Interviewer:** PostgreSQL 지원이 없다면 이 제품을 다른 사람에게 추천할 가능성은 있습니까?

**서연:** MySQL을 쓰면서 터미널 접속이 잦은 동료가 있다면 링크는 보낼 수 있습니다. 하지만 제가 직접 사용하지 못했으니 추천이라기보다 참고 정보에 가깝습니다. 제 긍정적인 반응을 잠재 사용자 수로 세면 안 됩니다.

**Interviewer:** README에서 PostgreSQL 로드맵을 보여주면 기다릴 의향이 있습니까?

**서연:** 별도로 기다리지는 않습니다. release가 나오면 compatibility 표와 테스트 범위를 볼 수는 있어요. 날짜 없는 “coming soon”은 구매나 도입 판단에 아무 영향이 없습니다.

**Interviewer:** PostgreSQL beta가 나오면 무엇이 있어야 설치하겠습니까?

**서연:** 지원 버전, 알려진 제약, SSL 방식, type rendering, cancellation과 read-only 의미가 문서화돼야 합니다. 예제 DB demo가 아니라 실제 RDS와 큰 테이블에서 시험한 결과도 보고 싶어요. 기존 MySQL 사용자의 후기만으로는 PostgreSQL 품질을 판단할 수 없습니다.

**Interviewer:** 지금 제품 방향에 대한 권고가 있다면요?

**서연:** PostgreSQL 사용자인 저를 당장 잡으려고 범위를 넓히지 마세요. MySQL 사용자에게 정말 반복 사용되는지 먼저 확인하는 편이 낫습니다. 그 문제가 검증되고 PostgreSQL 사용자에게도 같은 행동이 관찰될 때 별도 adapter처럼 확장하는 게 자연스럽습니다.

### 결정적 발언

> “PostgreSQL 지원이 없으면 기능을 평가하기도 전에 사용 대상에서 제외됩니다. 하지만 지원이 생긴다는 이유만으로 `psql`에서 전환하는 것도 아닙니다.”

> “시장에 PostgreSQL 사용자가 많다는 사실과, 이 제품이 그들의 반복되는 문제를 해결한다는 사실은 서로 다른 증거입니다.”

> “네 가지 UI 기능만 유지하는 것과 두 데이터베이스를 신뢰성 있게 지원하는 것은 전혀 다른 크기의 범위입니다.”

### Interview 11 결과

이 페르소나는 datavase가 다루는 터미널 내 결과 탐색과 접속 준비의 불편에는 공감하지만, MySQL·MariaDB 전용이라는 현재 경계 때문에 **실제 사용 가능성이 0에 가깝다**. 이는 제품 경험에 대한 부정적 평가가 아니라 기술 전제의 불일치다. 따라서 이 사용자의 PostgreSQL 요청을 현재 제품의 이탈 신호나 즉각적인 backlog 근거로 해석해서는 안 된다.

PostgreSQL 지원을 추가해도 자동으로 채택되지는 않는다. `psql`은 이미 설치돼 있고 예측 가능하며 PostgreSQL 고유 기능을 완전하게 다룬다. datavase가 경쟁하려면 grid와 single binary를 넘어 dialect·type·SSL·schema·cancellation·transaction semantics를 업무 중 신뢰할 수 있어야 한다. 특히 “단순 SELECT”에도 JSONB, array, cast, timezone 등 엔진 고유 동작이 포함되므로 얕은 연결 지원은 retention으로 이어지기 어렵다.

| 영역 | PostgreSQL 사용자의 반응 | 제품 판단 |
|---|---|---|
| 현재 MySQL 전용 범위 | 접속 대상이 없어 설치하지 않음 | 명확한 비대상 사용자 |
| 결과 grid·cell copy | `psql`보다 편할 가능성 | prototype으로 검증할 후보 |
| CSV export | `\copy`라는 강한 기존 대안 존재 | 단독 전환 요인이 아님 |
| datasource 관리 | wrapper script가 이미 표준화 | 팀별 기존 workflow와 비교 필요 |
| session read-only | 서버의 조회 전용 role과 중복 | 보안 통제보다 보조 guardrail |
| PostgreSQL dialect·type | 일상적인 SELECT에도 필수 | 지원 선언 전 compatibility 기준 필요 |
| query cancellation | 운영 사용의 신뢰 조건 | 화면 취소와 서버 취소를 함께 검증 |
| single binary | 설치 장벽을 낮춤 | 승인·provenance·업데이트 검토는 별도 |
| 다중 DB 지원 | 혼용 팀에는 통합 가치 가능 | 엔진별 품질 저하 시 장점 소멸 |
| 로드맵 표기 | 사용 행동을 만들지 않음 | release와 검증 근거가 있어야 평가 가능 |

이 인터뷰만으로 PostgreSQL 지원은 정당화되지 않는다. 오히려 현재 MySQL ICP의 반복 사용과 retention이 확인되기 전에 엔진을 늘리면, PR #82에서 줄인 사용자 기능 범위는 유지하더라도 내부 호환성·테스트·지원 범위가 크게 확대될 수 있다.

확장 여부를 판단할 더 강한 신호는 PostgreSQL 사용자의 일반적인 관심이 아니라 다음의 교집합이다.

- 터미널에서 PostgreSQL production 조회를 반복함
- `psql`의 결과 탐색이나 datasource workflow로 실제 비용을 겪음
- GUI 또는 조직의 wrapper가 그 문제를 충분히 해결하지 못함
- 제한된 prototype을 한 번이 아니라 실제 업무에서 반복 사용함
- MySQL과 공통된 핵심 workflow가 database별 복잡성보다 큰 가치를 만듦

따라서 Interview 11은 PostgreSQL을 즉시 추가하라는 결과보다 **명확한 제품 경계와 확장 증거의 기준**을 제공한다. 현재는 MySQL·MariaDB 사용자의 반복 사용을 먼저 검증하고, PostgreSQL은 별도 discovery와 기술 spike를 통과해야 하는 인접 시장 가설로 유지하는 편이 타당하다.

### 검증 후보

다음 항목은 이 페르소나의 가정을 실제 사용자 인터뷰, 관찰, 제한된 prototype으로 검증하기 위한 후보이다.

1. PostgreSQL 사용자가 최근 한 달 동안 `psql`로 production을 조회한 횟수와 실제 작업 순서
2. `psql` 결과 탐색, cell 복사, datasource 전환 때문에 발생한 구체적인 지연 또는 오류 사례
3. `pgcli`, IDE, wrapper script, managed query tool을 사용하거나 중단한 이유
4. PostgreSQL-only 팀과 MySQL·PostgreSQL 혼용 팀 사이의 통합 클라이언트 가치 차이
5. 제한된 read-only prototype을 실제 업무에서 2회 이상 다시 사용하는 비율
6. JSONB, array, enum, bytea, numeric, timestamp with time zone 등 type별 표시·복사 정확성
7. dollar-quoted string, cast, CTE, multi-statement 등 SQL 전달과 statement 분리의 호환성
8. RDS SSL, local Docker, SSH·SSM tunnel, 여러 schema와 `search_path` 조합의 접속 성공률
9. UI 취소 후 PostgreSQL backend query도 실제로 종료되는지와 실패 시 사용자 피드백
10. session read-only와 database role 권한을 사용자가 구분해 이해하는지
11. PostgreSQL 지원 개발·테스트 비용이 MySQL 안정화와 release 속도에 주는 영향
12. “PostgreSQL도 지원해달라”는 repository 반응 중 설치·반복 사용으로 이어지는 비율


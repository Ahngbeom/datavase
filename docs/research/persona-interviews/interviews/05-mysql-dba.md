## Interview 05 — 정태호: MySQL을 주 업무로 다루는 숙련 DBA

> 합성 페르소나 시뮬레이션이며 실제 사용자 인터뷰나 수요의 증거가 아니다.

### 페르소나

- 44세, MySQL DBA·Database Reliability Engineer, 경력 17년
- 트래픽이 큰 커머스 회사에서 MySQL 5.7/8.0과 MariaDB 운영
- 장애 대응 시 mysql CLI를 기본 도구로 사용하고 필요에 따라 performance_schema, processlist, slow log, EXPLAIN을 확인
- 편의 기능보다 서버 부하, 세션 상태, transaction 경계, 재현 가능성을 우선함
- “안전하다”는 표현에 매우 엄격하며 클라이언트 보호와 DB 권한 통제를 구분함
- 단순한 도구를 선호하지만 숨겨진 자동 동작과 불완전한 상태 표시는 싫어함

### 인터뷰 목적

현재의 작은 기능 범위가 production 조회 도구로 기술적 신뢰를 얻을 수 있는지 검증한다. session-level read-only, 자동 LIMIT, streaming·buffer truncation, query cancellation의 한계와 README 표현을 공격적으로 검토한다.

### 인터뷰

**Interviewer:** 최근 장애 대응에서 mysql CLI를 사용한 상황을 설명해주세요.

**태호:** 주문 조회가 느려진 장애였습니다. replica에 접속해서 `SHOW PROCESSLIST`, transaction 상태, 실행 계획을 확인했어요. 애플리케이션 쿼리를 그대로 재현하기 전에 실행 시간 제한을 걸고 예상 rows를 먼저 봤습니다. 결과 표를 보는 것보다 서버에 어떤 영향을 줄지가 먼저였어요.

**Interviewer:** mysql CLI에서 가장 불편한 점은 무엇인가요?

**태호:** 넓은 결과는 확실히 불편합니다. `\G`를 쓰거나 pager를 붙여요. 하지만 익숙해서 불편이 도구 교체 이유가 되지는 않습니다.

**Interviewer:** grid, history, completion, CSV를 제공하는 작은 terminal client라면요?

**태호:** 개발자에게는 편하겠지만 DBA 도구로는 부족합니다. 저는 session id, 현재 server·schema·user, autocommit, transaction 상태, 실행 중인 쿼리를 확실히 알아야 해요. 예쁜 grid가 그 정보를 가리면 오히려 위험합니다.

**Interviewer:** datavase는 datasource에 `read_only: true`를 설정하면 session을 read-only로 만듭니다.

**태호:** 실수 방지층으로는 괜찮습니다. 그런데 README의 “server refuses every write”는 너무 넓게 읽힐 수 있어요. MySQL의 read-only transaction semantics가 모든 side effect를 포괄한다고 생각하면 안 됩니다. 임시 테이블, stored routine, user-defined function, locking read 같은 경계 동작을 버전별로 테스트하고 정확히 문서화해야 합니다.

**Interviewer:** 사용자가 `SET SESSION TRANSACTION READ WRITE`로 해제할 수 있다고 적혀 있습니다.

**태호:** 그 문구는 좋습니다. 다만 해제 후 UI가 즉시 실제 서버 상태를 다시 읽어 표시해야 합니다. 설정 파일의 boolean을 계속 보여주면 잘못된 안전감을 줍니다.

**Interviewer:** read-only 상태를 항상 top bar에 표시하면 충분할까요?

**태호:** 상태의 출처가 중요합니다. datasource 설정값인지, 서버 세션에서 확인한 현재 값인지 구분해야 해요. reconnect가 일어나도 새 connection마다 다시 적용되고 검증됐는지 확인해야 합니다.

**Interviewer:** unbounded SELECT에는 자동으로 LIMIT 1000을 붙입니다.

**태호:** UX 보호에는 도움이 되지만 서버 부하 보호라고 부르면 안 됩니다. LIMIT 1000이어도 정렬, full scan, aggregation은 전체 데이터를 읽을 수 있어요. 원문 SQL이 바뀌었다는 사실도 실행 전에 보여야 합니다. 복잡한 SQL에 LIMIT을 안전하게 붙이는지도 중요하고요.

**Interviewer:** 결과는 batch로 streaming하고 최대 buffer를 넘으면 truncate합니다.

**태호:** 클라이언트 메모리에는 좋습니다. 하지만 서버가 이미 수백만 row를 읽고 전송 중일 수 있어요. ‘화면에 5만 row만 보인다’와 ‘서버 작업이 제한된다’는 다른 이야기입니다. truncate 시 결과가 완전하지 않다는 표시가 아주 명확해야 하고 CSV에도 같은 의미가 적용되는지 알아야 합니다.

**Interviewer:** 실행 중인 statement를 cancel할 수 있습니다.

**태호:** 키를 눌렀다는 것과 서버 query가 종료됐다는 것도 다릅니다. 취소 후 connection을 닫는지, 별도 connection으로 KILL QUERY를 보내는지, 서버에서 종료를 확인하는지 문서가 필요해요. production safety를 말하려면 취소 실패와 네트워크 단절 때의 동작도 중요합니다.

**Interviewer:** 이 기능들이 없으면 production client라고 부르면 안 될까요?

**태호:** production에서 사용할 수는 있습니다. 다만 ‘safe production client’라고 넓게 말하지 말라는 겁니다. 읽기 전용 세션은 write 실수 한 종류를 줄이고, auto limit은 화면 폭주 한 종류를 줄입니다. 무거운 SELECT나 lock, 잘못된 replica 선택까지 막아주지는 않아요.

**Interviewer:** 서버 측 timeout을 다시 추가해야 할까요?

**태호:** 기능 범위를 작게 유지하려면 반드시 내장할 필요는 없습니다. 대신 사용자가 datasource 초기화 구문이나 server policy로 `max_execution_time` 같은 제한을 적용할 수 있는지 설명하면 됩니다. 클라이언트가 안전을 모두 책임지려 들면 끝이 없어요.

**Interviewer:** DBA가 datavase를 사용할 장면은 있습니까?

**태호:** 단순 row 확인이나 결과 복사는 편할 수 있어요. 하지만 processlist와 EXPLAIN이 의도적으로 없다면 장애 진단 도구로는 mysql을 계속 씁니다. 두 도구를 오가는 비용 때문에 자주 쓰지는 않을 것 같습니다.

**Interviewer:** 그렇다면 DBA는 초기 ICP가 아닌 것이 맞습니까?

**태호:** 맞습니다. DBA 의견은 안전 문구와 세션 동작 검토에는 유용하지만 구매자나 주 사용자는 아닐 겁니다. 가끔 production을 조회하는 개발자가 더 자연스러워요.

**Interviewer:** 기능을 늘리지 않고 기술적 신뢰를 높이는 가장 좋은 방법은 무엇입니까?

**태호:** 테스트 가능한 약속만 하세요. README에 read-only가 무엇을 설정하는지, auto limit이 무엇을 보장하지 않는지, cancel 완료를 어떻게 판단하는지 적으세요. 그리고 실제 server version별 integration test가 있으면 좋습니다.

**Interviewer:** README 첫 demo에서 무엇을 보여줘야 할까요?

**태호:** production이라는 단어보다 연결된 host·schema와 실제 read-only session 상태가 눈에 띄어야 합니다. UPDATE 거부 장면 아래에는 ‘DB 권한을 대체하지 않음’이 짧게 보여야 하고요.

**Interviewer:** 지금 상태에서 설치해볼 의향이 있습니까?

**태호:** 개발자용 보조 클라이언트로 리뷰는 해볼 수 있습니다. 제 기본 DBA 도구로 바꾸지는 않습니다. README가 한계를 솔직히 쓰면 팀 개발자에게 추천할 가능성은 오히려 높아져요.

### 결정적 발언

> “LIMIT 1000이어도 서버는 전체 데이터를 읽을 수 있습니다.”

> “설정 파일의 read-only와 서버가 지금 확인해준 read-only는 다른 상태입니다.”

> “DBA는 사용자라기보다 안전 문구와 세션 동작을 검토해줄 사람에 가깝습니다.”

### Interview 05 결과

DBA는 예상대로 핵심 ICP가 아니었다. processlist, EXPLAIN, transaction tooling이 빠진 작은 범위는 전문가의 주 도구를 대체하지 못한다. 이 요구를 충족하려고 기능을 되살리면 PR #82의 방향을 되돌리게 된다.

그러나 DBA 인터뷰는 제품의 약속을 정교하게 만드는 데 큰 가치가 있었다. 현재 보호 기능은 서로 다른 위험을 일부만 줄인다.

| 기능 | 실제로 줄이는 위험 | 보장하지 않는 것 |
|---|---|---|
| session read-only | 우발적인 일반 write | DB 권한 통제, 모든 side effect, 의도적 해제 방지 |
| auto LIMIT | 무제한 결과 표시·전송 일부 | full scan, sort, aggregation, server 실행 비용 |
| streaming·buffer max | 클라이언트 메모리 폭주 | 서버 측 작업량, 완전한 결과 보존 |
| client cancel | 사용자가 중단을 요청할 수 있음 | 서버 query가 즉시 종료됐다는 보장 |
| grid | 넓은 결과의 가독성 | session·transaction·server 상태의 자동 안전성 |

따라서 제품의 신뢰는 보호 기능을 더 많이 붙이는 것보다 **각 기능의 보장 범위를 테스트하고 정확히 표현하는 것**에서 나온다.

### 기술 검증 후보

다음 항목은 기능 추가 요구가 아니라 현재 약속의 검증 항목이다.

1. MySQL/MariaDB 지원 버전별 read-only 적용·재연결·해제 상태 integration test
2. 임시 테이블, stored routine, locking read 등 경계 동작 테스트와 문서화
3. auto LIMIT SQL 변환 범위와 실행 전 사용자 가시성
4. buffer truncate 시 grid·copy·CSV가 불완전 결과임을 일관되게 표시하는지 확인
5. cancel 요청 뒤 서버 query 종료를 어느 수준까지 확인하는지 명시
6. top bar의 read-only 표시가 config 값인지 실제 session 상태인지 확인

---

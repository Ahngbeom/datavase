# 사전 QA 리허설 리포트 — Interview 05 페르소나 (정태호, MySQL DBA)

> 합성 페르소나 시뮬레이션이며 실제 파일럿 참가자의 발언이 아니다. 실제 파일럿 게이트(issue #102)
> 숫자에는 반영되지 않는 사전 리허설 증거 자료다.

**페르소나**: 정태호, 44세, MySQL DBA·Database Reliability Engineer, 경력 17년. 트래픽이 큰
커머스 회사에서 MySQL 5.7/8.0·MariaDB 운영. 장애 대응 시 mysql CLI가 기본 도구이며,
session-level read-only의 정확한 의미, 자동 LIMIT, streaming/buffer truncation, 값 정확성에
특히 엄격함.

## stop-condition 관찰 여부 — 상단 요약

**세 가지 stop-condition(값 오표시, read-only/접속대상 오표시, credential 노출) 중 어느 것도
엄밀한 의미로는 관찰되지 않았다.** DECIMAL·ENUM·DATETIME 값은 raw `mysql` CLI 출력과 100%
일치했고, top bar의 `read-only` 배지는 매 순간 `SELECT @@session.tx_read_only`의 실측값과
일치했으며, 비밀번호는 config.yaml·상태 디렉터리 어디에도 평문으로 남지 않았다.

다만 **stop-condition에 근접한 별도의 재현 가능한 버그**를 하나 발견했다: 한 번 서버 오류가
발생하면, 그 뒤로 이어지는 완전히 무관하고 정상적으로 성공한 statement들의 상태줄에 **그 오래된
오류 메시지가 "warning"이라는 이름으로 계속 따라붙는다.** 아래 "발견한 버그" 절 참조. 이것은
"값"이 틀린 것은 아니지만 "지금 이 statement의 상태"를 보여주는 자리에 죽은 정보가 남는
것이라, 페르소나 관점에서는 stop-condition (a)의 정신에 매우 가깝다고 판단해 최우선으로
보고한다.

## 설치 ~ 첫 쿼리

- README는 이번 리허설 준비 과정에서 이미 전체를 읽었음(설치/설정/Configure/Use/Keys/화면 절 전부).
- 바이너리는 사전 빌드된 것을 그대로 사용(`make build` 경로를 거쳤다고 가정), sha256 대조 확인함.
- datasource 추가는 TUI의 `a` add 폼으로 진행. 이름 `shop_ro`, host `127.0.0.1`, port `13306`,
  user `pilot_ro`, database `shop`, `read only` 체크박스 on, tls는 기본값 `preferred` 유지,
  password 필드는 **비워둠** — README의 "키체인 없는 headless 환경" 안내를 따라
  `DATAVASE_PASSWORD_SHOP_RO` 환경변수로 대신함(이름 규칙 `DATAVASE_PASSWORD_<NAME 대문자>` 확인).
- 저장된 `config.yaml`에 비밀번호 평문이 없음을 확인:
  ```yaml
  datasources:
    - name: shop_ro
      host: 127.0.0.1
      port: 13306
      user: pilot_ro
      database: shop
      tls: preferred
      read_only: true
  ```
- Enter로 접속 → 즉시 성공. 상단바: `shop_ro @shop  pilot_ro@127.0.0.1:13306 read-only`,
  하단: `server 11.4.12-MariaDB-ubu2404` (요청받은 MariaDB 11.4와 일치).
- **README 열람 → 설정 파일 작성 → 첫 성공 접속까지 약 2분 10초** (11:56:51 시작 → 11:59:00
  접속 확인). 폼 자체는 필드가 명확해 막히는 곳이 없었음.
- **첫 성공 쿼리**는 접속 직후 실행한 `SELECT @@session.tx_read_only AS tx_read_only,
  @@global.read_only AS global_read_only` — 접속 완료로부터 수십 초 내.

## 실행한 검증 항목과 관찰값

### 1. session read-only 독립 검증
```sql
SELECT @@session.tx_read_only AS tx_read_only, @@global.read_only AS global_read_only;
```
결과: `tx_read_only = 1`, `global_read_only = 0`. **세션 레벨에서만 read-only이고 서버
global은 그대로**라는 README의 주장과 top bar 배지가 실측과 일치함을 독립적으로 확인.

### 2. information_schema — orders 컬럼 타입
```sql
SELECT column_name, data_type, column_type, numeric_precision, numeric_scale
FROM information_schema.columns WHERE table_schema='shop' AND table_name='orders'
ORDER BY ordinal_position;
```
`total decimal(10,2)`, `status enum('paid','shipped','refunded')`, `placed_at datetime` 확인.

### 3. 값 정확성(DECIMAL/ENUM/DATETIME) — raw mysql과 대조
```sql
SELECT id, total, status, placed_at FROM orders ORDER BY id;
```
dv 그리드 값(예: `129.00`, `48.50`, `1024.00`, `19.99`, `256.00`, `73.25` / `shipped, paid,
paid, refunded, shipped, paid` / `2026-09-01 10:12:00` 등)을 별도로 실행한
`mysql -h127.0.0.1 -P13306 -upilot_ro ...` 출력과 바이트 단위로 대조 — **완전히 일치**.
DECIMAL(10,2)의 후행 0(`129.00`, `48.50`)도 그대로 유지됨. 리터럴로 추가 검증한
`0.10`, `-5.50`도 자릿수·부호 보존 확인.

### 4. NULL vs 빈 문자열
```sql
SELECT NULL AS is_null, '' AS is_empty, 0.10 AS small_decimal, -5.50 AS neg_decimal;
```
그리드에서 NULL 컬럼은 `NULL` 문자로, 빈 문자열 컬럼은 완전한 공백으로 구분 표시됨 —
README가 명시한 구분과 일치.

### 5. auto_limit
빈 LIMIT의 모든 SELECT(스칼라 SELECT 포함)에 `LIMIT 1000 added`가 상태줄에 표시됨. 이미
`LIMIT`이 있는 statement(아래 6번)에는 추가되지 않음 — "이미 스스로 제한하는 SELECT" 판별이
정확히 동작. 다만 스칼라 SELECT(FROM 없는 `SELECT @@변수`)에도 기계적으로 `LIMIT 1000`이
붙는 것은 무해하지만, 페르소나가 우려한 "복잡한 SQL에 안전하게 LIMIT이 붙는가"를 완전히
검증하려면 JOIN·서브쿼리·UNION이 섞인 실제 운영 쿼리로 추가 테스트가 필요함(이번 리허설
범위에서는 다루지 않음).

### 6. streaming / buffer_max truncation, CSV 일관성
6행짜리 `orders`를 7-way self cross join해 `LIMIT 100000`(자체 LIMIT 있음 → auto_limit 미적용,
정확히 작동)으로 279,936행 중 최대치를 요청:
```sql
SELECT a.id,b.id,c.id,d.id,e.id,f.id,g.id
FROM orders a,orders b,orders c,orders d,orders e,orders f,orders g LIMIT 100000;
```
결과: **정확히 50,000행에서 truncated** — `50000 rows · 312ms · truncated`
(기본 `buffer_max: 50000`과 일치). `F3`(export) → CSV 선택 → 저장 후 상태줄:
`50000 rows written to shop_ro-20260922-120037.csv (683.6 KB) · truncated at buffer_max`.
파일을 `wc -l`로 확인 → **50,001줄(헤더 1 + 데이터 50,000)**, 그리드와 CSV의 행수·truncation
표시가 정확히 일치함. README가 "그리드에 보인 것이 곧 CSV"라고 한 약속을 독립 검증 완료.
(단, CSV는 dv 실행 시점의 작업 디렉터리에 조용히 쓰였고 저장 대화상자에는 파일명만 보이고
전체 경로는 보이지 않음 — 사고 대응 중 "이 파일이 정확히 어디 저장됐는지" 순간적으로 헷갈릴 수
있는 사소한 지점.)

재귀 CTE로 더 큰 결과를 시도했을 때는 MariaDB 서버 자체의 `max_recursive_iterations=1000`
제한에 먼저 걸렸고, 그 서버 경고도 정확히 전달됨:
`1000 rows · 10ms · LIMIT 1000 added · 1 warning: Query execution was interrupted. The query
exceeded max_recursive_iterations = 1000. The query result may be incomplete` — 이 자체는
정상 동작.

### 7. 쓰기 시도 — read-only 거부
```sql
UPDATE orders SET total = 0 WHERE id = 1;
```
결과: `Error 1142 (42000): UPDATE command denied to user 'pilot_ro'@'192.168.65.1' for table
`shop`.`orders``.

**한계 — 검증 불가 항목**: `pilot_ro` 계정은 서버 GRANT가 애초에 `SELECT`뿐이다
(`SHOW GRANTS` 확인: `GRANT SELECT ON shop.* TO pilot_ro@%`). 따라서 이 거부가
① dv의 `read_only: true` 클라이언트 가드 때문인지, ② 애초에 서버 GRANT가 없어서인지
**이번 계정으로는 구분할 수 없었다.** 이는 인터뷰에서 정태호가 정확히 지적한 지점
("DB 권한 통제와 클라이언트 read-only는 다른 상태")이 이번 리허설에서도 그대로 재현된
한계다 — write 권한이 있는 별도 계정으로 같은 테스트를 반복해야 dv 자체 가드의 존재를
독립적으로 증명할 수 있다.

### 8. SET SESSION TRANSACTION READ WRITE
```sql
SET SESSION TRANSACTION READ WRITE;   -- 성공, 0 rows
SELECT @@session.tx_read_only;        -- 바로 다음 statement, 결과: 1
```
`tx_read_only`가 여전히 `1` — README의 "SET SESSION TRANSACTION READ WRITE lifts it for the
session ... The next statement puts it back"와 정확히 일치하는 동작. dv가 매 statement 실행
전에 read-only를 재적용한다는 주장을 실측으로 확인.

### 9. 파일 권한
상태 디렉터리(`XDG_STATE_HOME/datavase`)는 `drwx------`(0700), 그 안의 `datavase.db`,
`history.db` 등은 `-rw-r--r--`(0644) — README의 "0700 디렉터리, 0644 파일" 설명과 정확히 일치.

## 발견한 버그 — 오래된 에러가 무관한 statement에 "warning"으로 계속 표시됨

**재현 순서**:
1. `UPDATE orders SET total = 0 WHERE id = 1` 실행 → `Error 1142 ... UPDATE command denied`
   (정상적인 에러 표시).
2. `SET SESSION TRANSACTION READ WRITE` 실행(정상 성공, 0 rows) → 상태줄:
   `0 rows · 5ms · 1 warning: UPDATE command denied to user 'pilot_ro'@'192.168.65.1' for
   table `shop`.`orders`` — **UPDATE와 전혀 무관한 statement에 UPDATE 에러 텍스트가
   "warning"으로 붙음.**
3. `SELECT @@session.tx_read_only AS tx_read_only` 실행(정상 성공, 1 row) → 상태줄에
   **똑같은 문구**가 그대로 또 붙음.
4. `SELECT 1 AS ping`, `SELECT 2 AS ping2` — 완전히 새로운, 경고를 만들 이유가 전혀 없는
   리터럴 SELECT 두 개를 연달아 실행해도 **여전히 같은 문구가 계속 붙음** (총 4개의 무관한
   성공 statement에 동일하게 재현).
5. 이후 내가 실수로 편집기 텍스트를 이어붙여 실제 새 구문 오류(`... near '3 AS ping3 LIMIT
   1000'`)를 발생시키자, 다음 statement의 "warning" 문구가 **그 새 오류 텍스트로 교체**됨 —
   즉 이 필드는 "성공적으로 지워지는" 것이 아니라 "다음 실제 에러가 나야만 덮어써지는" 상태로
   보인다.

**판단**: 이건 개별 값이 틀린 것도, read-only 배지가 틀린 것도, credential 노출도 아니라서
엄밀한 stop-condition 3개 중 어디에도 해당하지 않는다. 그러나 상태줄의 "N warning"은
"지금 이 statement가 실제로 무엇을 했는가"를 알려주는 유일한 자리이고, 정태호 페르소나에게
이 자리는 세션 상태만큼 신뢰해야 하는 정보다. 한 번 에러가 나면 이후 몇 개의 성공한
statement 동안 상태줄이 계속 거짓 경고를 보여주는 것은, 장애 대응 중이라면 "방금 이 SELECT가
진짜 뭔가 문제가 있었나?"를 매번 되짚어야 하는 상황을 만든다 — 정확히 인터뷰에서 그가
"예쁜 grid가 그 정보를 가리면 오히려 위험합니다"라고 말한 것과 같은 종류의 위험이다.

## 5줄 주간 다이어리 (1주차, 페르소나 말투)

```
1. 이번 주에 dv를 쓴 날: 사실상 오늘 리허설 세션 1회. 실사용 주간이 아니라 0일에 가깝게 봐야
   합니다.
2. dv 대신 mysql이나 GUI로 돌아간 순간: read-only 거부가 dv 자체 가드 때문인지 계정 GRANT
   때문인지 구분이 안 돼서, 곧바로 mysql CLI로 SHOW GRANTS부터 확인했습니다. 그 순간이 정확히
   mysql로 돌아간 지점입니다.
3. 화면이 틀린 것을 보여준 적: 값 자체(DECIMAL, ENUM, DATETIME, read-only 배지)는 한 번도
   틀리지 않았고 raw mysql과 전부 일치했습니다. 다만 UPDATE 거부 에러가 한 번 난 뒤로, 전혀
   무관한 SELECT 두세 개에까지 "1 warning: UPDATE command denied..."가 계속 따라붙었습니다.
   값은 아니지만 "지금 상태"를 보여주는 자리가 죽은 정보를 계속 보여준 거라 저는 이것도
   화면이 틀린 사례로 칩니다.
4. 막혔는데 물어보지 않고 넘어간 것: SET SESSION TRANSACTION READ WRITE가 실제로 서버에서
   풀렸다가 dv가 다음 statement 전에 되돌리는 건지, 애초에 서버로 보내지도 않는 건지 프로토콜
   레벨까지는 안 봤습니다. 패킷 캡처는 이번 범위 밖이라 넘어갔습니다.
5. (해당 없음 — 리허설 자체가 1회성이라 "이번 주 DB를 몇 번 봤나"는 별도로 셀 것이 없습니다.)
```

## 기술적 부정확성/애매한 표현

- README 자체의 문구("the server refuses every write")는 이번 실측으로는 **반증도 확증도
  못했다** — SELECT 전용 계정만 갖고는 dv 가드와 서버 GRANT를 분리할 수 없기 때문. 인터뷰
  05에서 이미 지적된 문제가 QA 리허설에서도 닫히지 않았다는 것 자체가 하나의 발견이다. 쓰기
  권한 있는 계정으로 같은 테스트를 한 번 더 돌리는 것을 강력히 권한다.
- 그 외 이번에 실측한 문구들(session read-only, auto_limit, buffer_max/truncated, NULL vs
  빈 문자열, 0700/0644 파일 권한)은 모두 README의 서술과 정확히 일치했다 — 과장되거나 틀린
  설명을 발견하지 못했다.

## 페르소나가 요청했을 법한 기능

- 위 버그에 대한 수정: 성공한 statement는 반드시 "0 warnings"(또는 아무 표시 없음)로
  시작해야 하며, 이전 에러 텍스트가 다음 statement의 warning 자리에 남아서는 안 된다.
- (신규 기능이라기보다 버그) "warning"과 "직전 statement의 error"를 명확히 다른 색/다른
  줄로 분리해서, 지금 statement가 만든 경고인지 아닌지 한눈에 구분되게.
- 그 외 processlist/EXPLAIN 관련 요구는 인터뷰 05에서 이미 "초기 ICP 아님, 의도적 제외"로
  정리된 사안이라 이번 리허설에서 새로 제기하지 않음.

## 이 리허설의 한계

- `pilot_ro` 계정이 SELECT 전용이라 dv 자체 read-only 가드와 서버 GRANT를 분리 검증하지
  못함(위 "검증 불가 항목" 참조). 같은 이유로 임시 테이블·stored routine·locking read 같은
  경계 동작(인터뷰 05의 기술 검증 후보 #2)도 이번 계정으로는 시도할 수 없었다(애초에
  CREATE TEMPORARY TABLES 등의 GRANT가 없어 동일한 모호함이 반복될 것이므로 생략).
- cancel(⌘F2/KILL QUERY), SSH 터널, TLS verify-identity, 재연결(드롭 후 `⌘R`) 흐름은
  이번 리허설 시간 범위에서 다루지 않음.
- 데이터셋이 매우 작음(orders 6행, customers 4행) — 실제 컬럼에 존재하는 NULL이나 극단적인
  DECIMAL 값은 못 봤고, NULL/음수/큰 자릿수 테스트는 리터럴 `SELECT`로 대체했다.
- TUI 조작은 실제 키보드가 아니라 tmux를 통한 키 입력 주입으로 진행했다 — 실제 터미널
  (Ghostty/iTerm2/Terminal.app)에서의 `⌘` 포워딩, 클립보드 동작 등은 검증 범위 밖이다.
- 1회성 리허설이므로 "여러 날에 걸친 사용 패턴"은 관찰할 수 없었고, 5번 다이어리 문항은
  해당 없음으로 처리했다.

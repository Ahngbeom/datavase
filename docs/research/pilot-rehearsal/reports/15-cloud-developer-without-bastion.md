# 파일럿 QA 리허설 — Interview 15 페르소나 (박지민)

**페르소나:** 박지민, 31세, Backend·Cloud Engineer(경력 6년). bastion 없이 managed MySQL 호환 DB를
VPN + local DB proxy + IAM 단기 인증으로 접근. 평소 IntelliJ/DataGrip과 터미널 `mysql`을 병행하고,
운영 DB 계정은 이미 database-level read-only. "client가 UPDATE를 막는 것보다 계정 권한 자체를 더
신뢰한다"는 것이 핵심 회의론.

> 이 리허설은 synthetic 사전 점검이며 실제 파일럿 게이트(issue #102)의 숫자에는 반영되지 않는다.

## Stop-condition 관찰 여부 — **셋 다 관찰되지 않음**

- (a) 화면/내보낸 값이 틀림 — 아니오. `customers`, `orders` 조회 값이 `mysql` CLI로 직접 확인한
  스키마·값과 일치.
- (b) read-only 아닌 세션이 read-only로 표시되거나 잘못된 datasource/schema 표시 — 아니오. 상단바는
  연결 내내 `pilot @shop  pilot_ro@127.0.0.1:13306 read-only`로 정확히 유지됨.
- (c) credential 노출 — 아니오. `config.yaml`에 평문 비밀번호 없음, `history: false`로 statement
  기록 파일 자체가 생성되지 않음(확인함).

다만 아래 "버그·마찰" 중 하나(진단 메시지 오분류)는 stop-condition 정의에는 안 걸리지만 실사용자
신뢰에 영향을 줄 수 있는 실제 버그로 보여 최상단 다음에 배치했다.

## 설치~첫 쿼리

- 바이너리는 이미 빌드되어 있다는 전제(README "From a clone: make build" 경로)로 시작.
- README에서 실제로 연 섹션: **Install**, **First run**, **Configure**(`read_only`, 패스워드 저장
  방식) — `dv check`가 있는 **Use** 섹션도 열람.
- 소요 시간(추정, 스크립트 기반 리허설이라 사람이 읽는 시간을 얹어 추정): README 관련 섹션 훑기
  3~5분 + `config.yaml` 수동 작성 1~2분 + `dv check`로 연결 확인 1분 + 첫 성공 쿼리까지 총
  **10분 내외**. 지민이라면 datasource 마법사(`a`) 대신 손으로 `config.yaml`을 작성했을 것 — 평소
  로컬 dev 설정을 텍스트로 관리하는 습관과 맞고, "config에 secret을 저장하지 않아도 되는 것"이라는
  인터뷰에서의 조건을 직접 확인하고 싶었기 때문.
- 비밀번호 저장 방식: **환경변수 `DATAVASE_PASSWORD_PILOT`**을 선택. IAM 단기 토큰을 다른 터미널에서
  `export`해 쓰는 평소 습관과 가장 가깝고, `dv auth`(macOS Keychain 프롬프트)는 이 리허설 환경에서
  GUI 상호작용이 불가능해 애초에 선택지가 아니었다는 점도 실제와 비슷한 조건이라 판단.

## 실행한 쿼리와 관찰

1. `SELECT DATABASE(), CURRENT_USER(), @@read_only;` — 접속 직후 "내가 어느 서버/계정/스키마에
   붙었는지"부터 확인하는 평소 습관 그대로. 결과: `shop`, `pilot_ro@%`, `0`(서버 전역 read_only는
   0 — 세션 단위 read-only이지 replica 설정이 아니므로 예상대로). **관찰(마찰 후보):** 상태줄에
   `LIMIT 1000 added`가 찍혔다. 이 쿼리는 `FROM` 절이 없어 항상 1행만 반환하는데도 `auto_limit`이
   붙었다는 메시지가 나온 것 — 결과 값 자체는 옳지만, `sqlparse.AutoLimit`이 "이미 스스로 제한하는
   SELECT인가"를 판단할 때 FROM 없는 SELECT를 특별 취급하지 않는 것으로 보인다. 사소하지만 상태줄
   문구의 정확성을 의심하게 되는 지점.
2. `SELECT id, name, email FROM customers LIMIT 5;` — 정상. `mysql` CLI로 재확인한 값과 일치.
3. `SELECT id, customer_id, status, total_amount, created_at FROM orders ...` — **실패**:
   `Error 1054 (42S22): Unknown column 'customer_id' in 'SELECT'`. 실제 컬럼은
   `customer, total, status, placed_at`이었다. 이 순간 지민이라면 바로 `mysql` CLI로
   돌아가 `DESCRIBE orders`를 쳤을 것(실제로 이 리허설에서도 그렇게 했다) — dv에는 테이블 미리보기나
   tables 탭으로 스키마를 볼 수 있는 기능이 있지만, 이미 편집기에 SQL을 쓰던 흐름에서는 `mysql`로
   전환하는 쪽이 더 빨랐다.
4. 컬럼명을 바로잡아 재실행 — 정상, `Grace Hopper / paid / 73.25 / ...` 등 실제 데이터와 일치.

## 쓰기 시도와 "이미 read-only인 계정 위에서 client read-only가 주는 가치"

`UPDATE customers SET email = 'test@example.com' WHERE id = 1;` 실행 결과:

```
Error 1142 (42000): UPDATE command denied to user 'pilot_ro'@'192.168.65.1' for table `shop`.`customers`
```

세션 상단바는 계속 `read-only`로 표시된 채였다. 그러나 이 에러 메시지는 **MySQL 서버의 GRANT 시스템이
낸 표준 거부(1142)**이지, README가 약속한 "A refused statement says which setting refused it"에
해당하는, `read_only: true` 세션 가드가 직접 낸 것으로 보이는 문구(예: 세션 READ ONLY 트랜잭션 관련
에러)가 아니었다. 즉 **이미 계정이 SELECT 전용이면, dv의 세션 read-only 가드가 실제로 발동했는지
아니면 계정 GRANT가 먼저 막았는지 화면만으로는 구분할 수 없었다.**

이는 인터뷰에서 지민이 정확히 지적한 지점과 일치한다 — "DB 권한이 read-only인 계정에서는 중복이고",
"session 설정과 server-side 권한은 구분해서 보여줘야 합니다." 이번 테스트는 그 우려가 근거 있음을
보여준다: 세션 가드가 추가한 방어층은 상단바 표시(`read-only`)로만 존재감을 드러낼 뿐, 실제 쓰기
거부 순간에는 계정 권한 거부와 시각적으로 구별되지 않았다. write 권한이 있는 계정으로 같은 테스트를
해야 두 레이어(세션 가드 vs 계정 GRANT)의 메시지 차이를 확인할 수 있는데, 이 리허설 계정은 처음부터
SELECT 전용이라 그 비교가 불가능했다 — 이는 리허설 자체의 한계이기도 하다.

## 5줄 다이어리 (1주차, 페르소나 어투)

```
1. 이번 주에 dv를 쓴 날: 하루(오늘 리허설 한 번). 평소 production 조회는 한 달 2~3번, staging은
   주 몇 번이라 이 정도 빈도로는 아직 습관이라 할 게 없습니다.
2. dv 대신 mysql이나 GUI로 돌아간 순간: 있었습니다. orders 테이블 컬럼명을 잘못 짚어서 쿼리가
   Unknown column 에러를 냈을 때, tables 탭을 열기보다 반사적으로 mysql로 가서
   DESCRIBE orders부터 쳤습니다. SQL 에디터에 이미 손이 가 있으면 스키마 확인도 같은 터미널의
   mysql이 더 빠르다고 느꼈습니다.
3. 화면이 틀린 것을 보여준 적: 값 자체는 없었습니다. 다만 FROM 없는 SELECT에도 "LIMIT 1000 added"가
   찍히는 건 틀린 정보는 아니어도 신뢰가 살짝 깎이는 지점이었고, UPDATE가 막혔을 때 그게 read_only
   세션 가드 때문인지 계정 권한 때문인지 화면만으로는 구분이 안 됐습니다. 저희 계정은 어차피
   SELECT 전용이라 큰 문제는 아니었지만, 두 개를 구분해 보여주겠다던 README 설명과는 어긋납니다.
4. 막혔는데 물어보지 않고 넘어간 것: dv ls가 "password stored"라고 보여준 게 실제로는 그 순간
   셸에 export해 둔 환경변수를 본 건지, 진짜 키체인에 저장된 걸 본 건지 헷갈렸는데 그냥 넘어갔습니다.
   IAM 토큰처럼 매번 새로 export하는 입장에서는 이 표시가 뭘 의미하는지 나중에 다시 확인해야 할 것
   같습니다.
5. 이번 주에 dv를 아예 안 썼다면, 그 주에 DB는 몇 번 보셨나요?: 해당 없음(리허설에서는 사용).
   평소라면 production 2~3번, staging 몇 번은 mysql이나 DataGrip으로 봤을 겁니다.
```

## 버그·마찰 목록

1. **(버그, 우선순위 높음) `dv check`의 오류 분류가 스키마 접근 거부와 자격증명 거부를 구분하지
   못함.** 존재하지 않는 데이터베이스명으로 접속을 시도하면 서버는
   `Error 1044 (42S22)... Access denied ... to database 'nonexistent_schema'`를 반환하는데, dv가
   붙이는 다음 줄은 `the server refused the credentials — check user, and the stored password with
   dv auth`였다. 실제 원인은 비밀번호가 아니라 스키마 이름/권한이었다. 반대로 **진짜 비밀번호 오류**
   (`Error 1045`)에서도 같은 문구가 나왔다 — 두 상황이 같은 진단 문구로 합쳐져 있다. 이는 이
   페르소나가 인터뷰에서 명시적으로 요청한 "credential 만료"와 "DB 권한 거부"의 구분과 정확히
   충돌하는 지점이라 기록해 둔다. (`dv -c <config> check <name>`으로 재현.)
2. **(양호, 긍정 신호) 네트워크/포트/DNS 오류 분류는 기대에 잘 맞았다.** 존재하지 않는 포트(프록시
   미기동 시뮬레이션)에는 `nothing is listening there — check port, and whether the server or the
   proxy in front of it is running`, 존재하지 않는 호스트명(DNS/VPN 다운 시뮬레이션)에는
   `the host name did not resolve — check host, and whether the VPN or DNS it needs is up`가
   나왔다 — "proxy"와 "VPN"을 직접 언급하는 문구는 이 페르소나의 워크플로를 정확히 겨냥하고 있다.
3. **(마찰, 낮음) FROM 없는 SELECT에도 `auto_limit`이 적용된 것으로 표시.** 결과가 틀리진 않지만
   상태줄 문구의 정밀도 문제.
4. **(마찰, 낮음) `dv ls`의 "password stored" 표시가 키체인 저장과 방금 export한 환경변수를 구분하지
   않는다.** 같은 datasource에 대해 셸에 `DATAVASE_PASSWORD_PILOT`을 export한 채로 `dv ls`를 돌리면
   "(password stored)", export 없이 돌리면 "(no password)"로 바뀐다 — 즉 이 표시는 키체인 상태가
   아니라 "지금 이 호출에서 자격증명을 찾을 수 있는가"를 보여준다. 단기 토큰을 매번 새 셸에서
   export하는 사용자에게는 "저장됐다"는 단어가 오해를 살 수 있다.
5. **(참고, 이 리허설의 인프라 문제이지 dv의 문제 아님)** 제공된 스키마의 실제 컬럼명이 작업
   지시서의 예시(`customer_id`, `total_amount`)와 달라(`customer`, `total`) 첫 쿼리가 실패했다 —
   dv 자체의 결함이 아니라 사전 정보와 실제 스키마의 불일치.

## 페르소나가 자연스럽게 요청했을 법한 기능

이번 테스트에서 새로 관찰된 것 위주로 적는다(인터뷰에서 이미 나온 proxy 내장·credential provider
요구는 이 리허설에서 다시 검증하지 않았으므로 반복하지 않음):

- `dv check`/실행 중 에러 메시지에서 **"credential 문제"와 "스키마/권한 문제"를 별도 문구로 분리**
  (버그 1 참조).
- `dv ls`(와 상단바 어딘가)에 자격증명 출처를 `keychain` / `env` 중 어느 쪽인지 표시.
- 쓰기가 거부됐을 때, 그것이 `read_only: true` 세션 가드 때문인지 계정 GRANT 때문인지 문구로
  구분(README가 이미 약속한 것을 실제로 지키도록).

## 이 리허설의 한계

- 실제 사람이 아니라 스크립트(pexpect + pyte로 가상 터미널을 구동)로 tview 화면을 조작했다. 마우스
  클릭, 실제 터미널의 `⌘` 전달, iTerm2/Ghostty 클립보드 동작 등은 검증하지 못했다.
- 테스트 계정이 처음부터 SELECT 전용이라, "세션 read-only 가드가 실제로 무언가를 막는" 순간(쓰기
  권한이 있는 계정에 `read_only: true`를 걸었을 때의 거부 메시지)은 재현하지 못했다 — 이 페르소나의
  핵심 질문("session read-only가 계정 GRANT와 구분되는 방어층인가")에 대한 답은 절반만 얻었다.
- SSH tunnel, 실제 클라우드 IAM 단기 토큰 갱신, VPN 재연결 같은 이 페르소나의 실제 마찰 지점은 로컬
  MariaDB 컨테이너 환경에서 시뮬레이션할 수 없어 다루지 않았다.
- README만 보고 판단한다는 눈가림 규칙 때문에 `docs/pilot/README.md`, `internal/`, 테스트 코드는
  보지 않았다 — 위 관찰은 모두 바이너리의 실제 동작에서 나온 것이다.

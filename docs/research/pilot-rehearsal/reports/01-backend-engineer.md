# dv 파일럿 사전 QA 리허설 — 페르소나 01: 김현우 (백엔드 엔지니어)

> 합성 페르소나 시뮬레이션 리허설이며, 실제 파일럿(issue #102) 참가자의 경험이나 수요의 증거가 아님.

- **페르소나**: 김현우, 34세, 백엔드 엔지니어 8년차. Java/Kotlin·Spring Boot·MySQL 8. 평소
  DataGrip을 쓰고, 장애 때는 `SSH → mysql`. 운영 DB에서의 우발적 write를 가장 걱정하며, 단축키가
  많은 도구를 부담스러워함.
- **바이너리**: `dv (devel)`, `/Users/bahn/personal/workspaces/datavase/worktrees/feature/pliot-v0.11.1/dv`
- **환경**: XDG_CONFIG_HOME/XDG_STATE_HOME을 페르소나 전용 scratchpad로 분리, 테스트 DB
  `127.0.0.1:13306/shop` (`pilot_ro`, SELECT 전용)

## Stop-condition 관찰 여부: 없음

세 가지 stop-condition — (a) 화면/내보낸 값이 틀림, (b) read-only 세션이 아닌데 read-only로
표시되거나 잘못된 datasource/schema가 표시됨, (c) credential 노출 — 중 어느 것도 관찰되지
않았다. 상단바는 접속 내내 `prod-ro @shop  pilot_ro@127.0.0.1:13306  read-only`를 정확히
유지했고, `customers`/`orders` 쿼리 결과값은 재조회해도 일관됐고, `config.yaml`과 스키마
캐시(`datavase.db`)·히스토리(`history.db`)에서 비밀번호 평문은 발견되지 않았다.

## 설치~첫 쿼리

- README 열람: 함(설치 섹션은 이미 알고 있었지만, "Configure" 절의 `read_only`/키체인/env var
  부분은 실제로 다시 읽었다. 이건 이 페르소나가 실제로 할 법한 행동 — 운영 DB 앞에서 read-only가
  진짜로 보장되는지 문서로 재확인하는 습관).
- 설치: 이미 빌드된 바이너리를 그대로 사용(README의 "From a clone: make build" 경로를 방금
  마쳤다고 가정).
- 설정: `dv` 실행 → 빈 datasource 목록 → `a`로 추가 폼 진입 → `name=prod-ro`,
  `host=127.0.0.1`, `port=13306`, `user=pilot_ro`, `database=shop`, `read only` 체크,
  `tls=preferred`(기본값 유지), tunnel 필드는 비움(리허설엔 실제 bastion 없음) → `Test` →
  `Save`.
- 첫 성공 쿼리까지 걸린 시간: 폼 채우기~첫 `SELECT` 성공까지 실제 사람 기준으로는 5분 안팎으로
  추정된다(정확한 벽시계 측정은 아님 — 아래 "한계" 참고. 이 리허설 자체는 tmux/pty 자동화
  환경을 붙이는 데 추가 시간이 들었지만, 그건 페르소나의 경험이 아니라 이 리허설의 계측 방식
  문제이므로 시간 추정에서 제외했다).

## 실행한 쿼리와 관찰

1. `SELECT * FROM customers` (테이블 미리보기) → 4행, `id/name/email/created_at` 정상 표시.
2. `SELECT id, customer_id, status, total_cents, created_at FROM orders ...` → 컬럼명을
   추측해서 틀렸고 `Error 1054 (42S22): Unknown column 'customer_id'`로 즉시, 명확하게 거부됨.
   실제 컬럼은 `id/customer/total/status/placed_at`였다 — 컬럼 자동완성(`Ctrl+Space`)을 이
   시점에 썼다면 안 겪었을 실수였는데, 이번엔 타이핑으로 바로 넣어 재현하지 않았다.
3. 최근 주문 5건: `SELECT id, customer, total, status, placed_at FROM orders ORDER BY
   placed_at DESC LIMIT 5` → 정상, 최신순 정렬 확인.
4. 특정 상태 주문: `WHERE status = 'refunded'` → 1건, 정확히 필터링됨.
5. 특정 고객 조회: `SELECT id, name, email FROM customers WHERE email = 'ada@example.com'`
   → 1건, 정확.

이 네 가지는 인터뷰에서 언급된 "운영 DB에서 가끔 확인하는" 전형적 조회 패턴을 반영했다.

## 쓰기 시도 결과

`UPDATE orders SET status = 'refunded' WHERE id = 1` 실행 → 즉시 거부:

```
Error 1142 (42000): UPDATE command denied to user 'pilot_ro'@'192.168.65.1' for table `shop`.`orders`
```

**중요한 한계**: 이 메시지는 MySQL 서버의 **grant 레벨** 거부(1142)이지, README가 명시하는
"A refused statement says which setting refused it" — 즉 `read_only: true`가 세션에
`SET SESSION TRANSACTION READ ONLY`를 걸어서 만드는 dv 고유의 거부 메시지가 아니다.
`pilot_ro` 계정 자체가 이미 SELECT 권한만 있기 때문에, grant 레벨에서 먼저 막혀서 dv의
read_only 가드가 실제로 어떤 문구를 보여주는지는 이번 리허설에서 **확인하지 못했다.**
다음 리허설에서는 DB 레벨에서는 쓰기 권한이 있는 계정에 `read_only: true`를 걸어서, dv 자체
가드가 실제로 "어떤 설정이 막았는지"를 이름 붙여 알려주는지 별도로 검증할 것을 제안한다.

## 5줄 다이어리 (1주차 — 실제로는 이번 한 세션 기준)

1. 이번 주에 dv를 쓴 날: 오늘 하루, 셋업하고 테스트 쿼리 몇 개 돌려본 정도.
2. dv 대신 mysql이나 GUI로 돌아간 순간: 없었다. 다만 `read_only` 가드가 진짜 뭐라고 말하는지
   확인 못 한 채 넘어갔다 — 테스트 계정이 애초에 쓰기 권한이 없어서, 그게 grant 때문인지
   dv 설정 때문인지 화면만 봐서는 구분이 안 됐다. 운영에서 이런 상황이면 좀 불안했을 것 같다.
3. 화면이 틀린 것을 보여준 적: 없다. read-only 표시, 접속 대상(`pilot_ro@127.0.0.1:13306`),
   스키마(`@shop`)는 쿼리를 몇 번을 돌려도 계속 맞게 떠 있었다.
4. 막혔는데 물어보지 않고 넘어간 것: add 폼에서 비밀번호 칸을 일부러 비워두고
   `DATAVASE_PASSWORD_PROD_RO` 환경변수로 대체하려 했는데, `Test` 버튼이 "Access denied"를
   냈다. 처음엔 설정을 잘못했나 싶어서 비밀번호를 폼에 직접 타이핑해서 넘어갔다 — 나중에 보니
   `dv ls`/`dv check`는 같은 환경변수로 잘 붙던데, `Test` 버튼만 그런 것 같다. README에는 이
   차이가 안 쓰여 있어서 그냥 넘어갔다.
5. (해당 없음 — 이번 주 dv를 썼음)

## 마찰/버그 목록

- **[재현 가능, 문서 미기재] add 폼의 `Test` 버튼이 `DATAVASE_PASSWORD_<NAME>` 환경변수를
  읽지 않는다.** 비밀번호 필드를 비운 채(환경변수 사용 의도) `Test`를 누르면
  `Access denied`가 뜨지만, 같은 이름·같은 환경변수로 `Save` 후 `dv ls`/`dv check`는
  정상 동작했다. README는 "환경변수가 설정되어 있으면 키체인보다 우선한다"고만 말하고,
  `Test` 버튼이 이 규칙에서 예외라는 언급은 없다. 헤드리스/CI 스타일로 비밀번호를 넣으려는
  사용자가 첫 화면에서 바로 겪을 수 있는 혼란.
- **[낮은 확신] 트리 탭에서 스키마 노드를 키보드만으로 펼치는 것을 확인하지 못했다.**
  `tree` 탭에서 `Down`/`Enter`/`Right`로 `shop` 스키마를 펼치려 했으나 화면이 바뀌지
  않았다. 다만 이 리허설은 색상 없는 plain-text 캡처로 화면을 읽었기 때문에, 포커스/선택
  하이라이트가 색으로만 표현됐다면 실제로는 동작했는데 내가 못 본 것일 수 있다 — 대신
  `tables` 탭(`F6`)에서는 목록·필터·미리보기가 모두 정상 동작했다. 이건 버그로 단정하지
  않고 "확인 필요" 항목으로만 남긴다.
- **[사소함] 폼의 `Test`/`Save`/`Cancel` 버튼 사이 이동은 `→`가 아니라 `Tab`으로만 됐다.**
  README의 "Move between panes"는 `⇥`/`⇧⇥`를 말하지만 폼 안 버튼 이동에 대한 언급은 없어서,
  처음엔 `→`를 눌러봤다가 안 움직여서 잠깐 헷갈렸다.

## 자연스럽게 요청했을 법한 기능

- `dv ls`가 비밀번호 출처(키체인 vs 환경변수)를 구분해서 보여주면 좋겠다 — 지금은
  "(password stored)"로만 나와서, 헤드리스 서버에 배포할 때 "이 값이 진짜 키체인에도
  저장됐나?"를 확인하기 애매했다.
- (인터뷰에서 이미 나온 내용이지만 재확인) `read_only: true`가 grant 레벨 거부와 dv 자체
  가드 거부를 구분해서 말해주면 더 안심될 것 같다 — 지금 문구만 보면 둘을 구분하기 어렵다.

## 이 리허설의 한계

- **실제 bastion/SSH tunnel을 거치지 않았다.** 이 페르소나는 평소 `ssh → mysql` 습관이 있고
  인터뷰에서도 SSH tunnel을 신뢰 조건으로 꼽았지만, 이번엔 로컬 테스트 DB에 직접 접속했다.
  실제였다면 tunnel 설정 필드(`tunnel host/port/user/identity`)를 실제로 채우고, bastion
  연결 실패 시 에러 메시지가 "어느 홉에서 막혔는지"를 얼마나 명확히 알려주는지 확인했을
  텐데, 이번엔 그 부분을 검증하지 못했다.
- **원격 네트워크 지연·VPN 등 실제 환경 변수가 없었다.** 로컬 컨테이너라 접속이 항상 즉시
  성공했고, `dv check`가 실패 원인을 "어느 설정 때문인지" 짚어주는 기능(README에 명시됨)은
  한 번도 실패 경로를 타보지 못해서 검증하지 못했다.
- **실제 macOS 키체인에 접근했다.** `Save`는 실제로 이 머신의 로그인 키체인에
  `service=datavase, account=prod-ro` 항목을 만들었다(XDG_CONFIG_HOME/XDG_STATE_HOME 격리는
  config/상태 파일에만 적용되고 OS 키체인은 격리되지 않는다). 리허설 종료 후 해당 키체인
  항목은 삭제해서 정리했다. 다만 이는 다른 합성 페르소나가 같은 물리 머신에서 같은
  datasource 이름을 쓰면 키체인에서 충돌할 수 있다는 뜻이라, 다른 페르소나 리허설을 병렬로
  돌린다면 이름 충돌에 주의가 필요하다(이건 dv의 버그가 아니라 이 리허설 인프라의 주의사항).
- **read_only 가드의 dv 고유 거부 메시지를 확인하지 못했다** (위 "쓰기 시도 결과" 참고) —
  테스트 계정이 grant 레벨에서 이미 막혀 있어서였다.
- **시간 측정이 실제 벽시계 기준이 아니다.** 이 리허설은 자동화 도구(tmux+pty)로 폼 입력과
  쿼리 실행을 스크립트로 재현했기 때문에, "첫 쿼리까지 5분 안팎"은 실제 사람의 타이핑·읽기
  속도를 반영한 추정치이지 측정값이 아니다.

# 파일럿 QA 리허설 — 페르소나 07: 박재민 (합성)

> 합성 페르소나 시뮬레이션이며 실제 사용자 인터뷰나 수요의 증거가 아님. `docs/pilot/README.md`의 실제 파일럿
> 게이트(issue #102) 숫자에는 반영되지 않는 사전 점검 리허설.

**페르소나**: 박재민, 38세, 시니어 백엔드 엔지니어(경력 13년). 커머스 SaaS, Java/Spring Boot/Go/MySQL 8.
개발 시 IntelliJ+DataGrip, 장애 대응 시 kubectl+stern+mysql+자체 스크립트. 운영 조회 계정은 이미
SELECT-only. 도구 전환 권한은 충분하지만 전환 기준은 높음 — "설치 권한과 전환 의지는 다른 문제".

---

## Stop-condition 관찰 여부 — 관찰된 것 없음

세 가지 정의된 stop-condition(화면/내보낸 값 오류, read-only 오표시 또는 잘못된 datasource/schema 표시,
credential 노출) 중 어느 것도 이번 세션에서 재현되지 않았다.

- 4개 쿼리 결과값은 모두 `mysql` CLI로 별도 확인한 실제 데이터와 정확히 일치했다.
- 상단 바의 `shop-prod-ro @shop pilot_ro@127.0.0.1:13306 read-only` 표시는 연결 직후, 쿼리 실행 후,
  쓰기 거부 후, 터미널 리사이즈(100x30 → 220x55) 후 모두 **한 글자도 바뀌지 않고 일관되게** 유지됐다.
- 비밀번호는 config.yaml에도, history.db/datavase.db(SQLite state 파일)에도 평문으로 남지 않았다
  (`grep`으로 직접 확인).

**다만 stop-condition은 아니지만 심각도가 높은 별도 발견이 하나 있다 — 아래 "쿼리 응답 시간" 참고.**

---

## 설치~첫 쿼리

- 바이너리는 이미 `make build`로 빌드되어 있었다고 가정. `dv version` (`dv (devel)`)과 `sha256sum`으로
  받은 파일이 명시된 값과 일치하는지부터 확인했다 — 재민이라면 실제로 이렇게 했을 것(제공된 credential
  도구는 provenance를 확인하는 습관).
- **README 열람: 예.** Install / First run / Configure / Use 섹션을 정독했다. Development 섹션은 보지
  않았다.
- datasource 등록은 TUI 마법사(`a`) 대신 `config.yaml`을 직접 작성했다 — 이 페르소나는 인터뷰에서
  "shell script로 이미 datasource를 관리한다"고 밝혔고, README의 `Configure` 섹션 예시가 필요한 키를
  전부 보여줘서 시행착오 없이 8줄로 끝났다.
- 비밀번호는 `dv auth shop-prod-ro`(키체인)를 먼저 시도했으나 이 리허설 하네스가 비대화형이라
  `stdin is not a terminal; run dv auth interactively`로 실패했다 — **이건 dv의 문제가 아니라 이 리허설
  환경의 한계**(아래 "한계" 참고). 대신 README가 헤드리스 환경용으로 명시한
  `DATAVASE_PASSWORD_SHOP_PROD_RO` 환경변수로 대체했다.
- `dv check shop-prod-ro` → `shop-prod-ro is reachable — server 11.4.12-MariaDB-ubu2404` 즉시 성공.
- **소요 시간(추정): 약 7~8분.** README 정독 + config.yaml 작성 + 인증 방식 전환 + 첫 쿼리 실행까지.
  (이 리허설 자체의 자동화 스크립트 디버깅 시간은 페르소나 경험이 아니므로 제외.)

---

## 실행한 쿼리와 관찰

`shop` 스키마(customers, orders)에서 그가 실제로 확인했을 법한 4개 쿼리를 실행했다.

1. `SELECT id, customer, status, total FROM orders WHERE status = 'refunded';`
   → 1 row, `id=4, Ada Lovelace, refunded, 19.99` — 실제 데이터와 일치.
2. `SELECT id, customer, total, placed_at FROM orders ORDER BY placed_at DESC LIMIT 5;`
   → 5 rows, 최신순 정렬 정확.
3. `SELECT id, name, email FROM customers WHERE name = 'Ada Lovelace';`
   → 1 row, 이메일 정확히 일치.
4. `SELECT status, COUNT(*) AS cnt FROM orders GROUP BY status;`
   → `paid 3 / shipped 2 / refunded 1` — 정확.

각 쿼리 모두 상태줄에 `LIMIT 1000 added`가 표시됐다(`auto_limit`이 실제로 적용됨을 텍스트로 확인 가능).

**쿼리 응답 시간 — 중요한 발견.** 4개 쿼리 모두 dv 자체 상태줄에 `3.02s`~`3.52s`로 표시됐다(6행짜리
작은 테이블에 대한 단순 SELECT). 동일 쿼리를 같은 컨테이너에 `mysql` CLI로 직접 실행하면 `43ms`였다
(`time mysql ... -e "SELECT status, COUNT(*) ... GROUP BY status"` → `0.043 total`). **약 70배 차이이며
4번의 쿼리 모두에서 재현됐다** — 우연한 네트워크 지연이 아니라 일관된 패턴으로 보인다. 이 리허설
규칙상 Go 소스는 들여다보지 않았으므로 원인은 특정하지 못했지만, 재민의 채택 기준("mysql보다 실제로
빠르다는 경험이 누적돼야 기본 도구가 된다")을 정면으로 위협하는 수치다. 장애 대응 중이었다면 그는
두 번째 쿼리부터 이미 `mysql`로 돌아갔을 것이다.

---

## 쓰기 시도 결과

`UPDATE orders SET status = 'paid' WHERE id = 4;` 실행 →

```
Error 1142 (42000): UPDATE command denied to user 'pilot_ro'@'192.168.65.1' for table `shop`.`orders`
```

이건 MySQL 서버의 GRANT 거부(계정 자체에 UPDATE 권한 없음)이고, dv의 `read_only: true` 클라이언트
가드가 별도로 개입해 다른 문구를 보여줬는지는 이 테스트만으로 구분할 수 없었다 — **왜냐하면 제공된
`pilot_ro` 계정 자체가 이미 SELECT-only이기 때문**이다. 이것은 정확히 재민이 인터뷰에서 예상한 상황과
같다: "운영 DB 안전은 계정 권한에서 보장해야 한다", "DB 계정이 이미 SELECT-only인 조직에서는
`read_only`의 차별성이 더 약해진다." 이번 리허설은 그 관찰을 재확인했을 뿐, `read_only: true`가 실제로
얹어주는 별도의 방어층을 독립적으로 검증하지는 못했다(쓰기 권한이 있는 계정으로 재시도해야 확인 가능).

거부 이후에도 상단 바는 계속 `read-only`로 정확히 표시됐다.

---

## 5줄 다이어리 (1주차, 페르소나 말투)

1. **이번 주에 dv를 쓴 날**: 오늘 하루, 짧은 세션 한 번.
2. **mysql이나 GUI로 돌아간 순간**: 있었죠. 비밀번호를 키체인에 넣으려는데 비대화형이라 막혀서
   환경변수로 우회했고 — 이건 제 노트북에서는 안 그럴 겁니다. 그보다 진짜 걸리는 건, 쿼리 하나
   돌릴 때마다 3초 넘게 걸리는 거예요. 장애 중이었으면 두 번째 쿼리부터 `mysql`로 돌아갔을 것 같아요.
3. **화면이 틀린 것을 보여준 적**: 없어요. datasource/schema/계정/read-only 표시는 리사이즈해도
   안 흔들리고 계속 맞았습니다. 이 부분은 솔직히 기대보다 좋았어요.
4. **막혔는데 물어보지 않고 넘어간 것**: CSV로 내보내려고 export 다이얼로그에서 그냥 Enter를 눌렀더니
   기본값이 Markdown 복사였어요. CSV를 받으려면 `c`를 눌러야 한다는 걸 다이얼로그 안에서만 알 수
   있었고요. 별거 아니라 그냥 다시 눌렀습니다.
5. **이번 주에 dv를 안 썼다면 DB는 몇 번 봤나**: 해당 없음(오늘 사용함).

---

## 마찰/버그 목록

- **쿼리 응답 시간 3초대(위 참고)** — 재현성 4/4, `mysql` 대비 약 70배. 가장 심각한 발견.
- **`dv ls`가 "(password stored)"라고 표시**하는데 실제로는 키체인이 아니라 환경변수(`DATAVASE_PASSWORD_*`)에서
  온 비밀번호였다. credential 취급에 민감한 이 페르소나 입장에서는 "저장 위치가 명확해야 한다"는
  본인 기준과 부딪힌다 — 키체인 저장과 환경변수 사용을 문구로 구분해주면 좋겠다. (credential 노출은
  아니고 표현의 모호함 수준.)
- **CSV export 다이얼로그에서 Enter가 기본으로 Markdown을 선택**한다. `⌘⇧C`/`F3`를 누르면 형식 선택
  다이얼로그가 뜨는데((m) Markdown / (j) JSON / (c) CSV file), 아무 것도 안 누르고 Enter만 치면
  첫 옵션(Markdown)이 선택된다. CSV가 목적이면 반드시 `c`를 눌러야 한다 — README에는 이 다이얼로그의
  기본 선택지가 무엇인지 나와 있지 않다.
- **`history` 기본값이 README만으로는 불분명하다.** `defaults:` 예시 블록에 `history: false  # do not
  write finished statements to disk`라고만 나와 있어서, 이게 "옵션 예시"인지 "키를 생략했을 때의
  기본값"인지 README 텍스트만으로는 구분되지 않는다(`tls:`는 "An absent tls: means preferred"처럼
  명시돼 있는데 `history`는 그런 문장이 없다). 이 페르소나는 "history에 민감한 값이 평문으로 남는
  방식이면 꺼야 한다"고 명확히 말했던 사람이라, 기본값이 무엇인지 문서에서 바로 알 수 없는 건 그에게는
  작지 않은 마찰이다.
- **datasource "가져오기" 기능이 없다** — 인터뷰에서 그가 꼽은 시험 사용 최소조건 1순위("기존 MySQL
  connection 정보를 쉽게 가져오는 것")가 이번 버전에도 그대로 부재. config.yaml을 직접 쓰는 건 빨랐지만
  ("가져오기"가 아니라 "손으로 다시 입력"이라는 그의 우려가 문자 그대로 맞았다), 그냥 몇 줄이라
  이번엔 이탈로 이어지지 않았을 뿐이다.

## 페르소나가 요청했을 법한 기능

- 기존 `mysql`/`~/.my.cnf` 접속 정보에서 datasource를 자동으로 가져오는 기능 (인터뷰 1순위 항목).
- SSH tunnel 연결 진단 — `dv check`처럼 "어느 hop에서 실패했는지"를 tunnel에도 적용 (이번 리허설엔
  tunnel이 없어 검증 못 함, 인터뷰 최소조건 항목).
- 쿼리가 왜 느린지 알 수 있는 근거(연결 수립 vs 실행 vs fetch 단계별 타이밍) — "장애 때는 편리함보다
  예측 가능성"이라는 그의 기준에서, 이유 없이 느리면 신뢰가 더 떨어진다.

---

## 이 리허설의 한계

- **실제 SSH tunnel/kubectl 경유 환경이 아니라 로컬 테스트 DB(127.0.0.1:13306)에 직접 연결했다.**
  재민이 평소라면 tunnel 연결 실패 시 확인했을 것들(어느 hop인지, 포트가 맞는지)은 이번에 전혀
  검증되지 않았다 — 이건 실제 파일럿 문서에도 없는 리허설 고유의 한계다.
- **TUI를 사람이 아니라 자동화 스크립트(Python pty + 가상 터미널 에뮬레이터)로 구동했다.** 실제 사람의
  타이핑 리듬과는 다르다. `dv auth`의 키체인 저장 경로는 실제 tty가 필요해 이 하네스로는 검증하지
  못했고 환경변수로 대체했다 — 이 선택 자체가 다이어리 2번 답변에 반영됨.
- 이 하네스 자체에 초기 버그가 두 개 있었다(멀티바이트 UTF-8이 read() 청크 경계에서 잘려 화면이
  깨져 보인 것, 짧은 타임아웃으로 타이핑 캡처가 누락된 것). 둘 다 하네스를 고쳐 재현했고, **dv 자체의
  문제가 아님을 직접 확인**했다(수정 전/후 raw 터미널 로그 비교). 참고로만 남긴다.
- 이 샌드박스의 `tmux`는 실제 tmux가 아니라 제한된 호환 shim이라(`-x`/`-y`, `resize-window` 등 다수
  명령 미지원) 실제로 사용하지 못했다. 대신 pty의 `TIOCSWINSZ`+`SIGWINCH`로 터미널 리사이즈만 근사
  검증했다. tmux 안에서의 실제 동작(Ctrl 스펠링 표시, 확장 키보드 프로토콜 미지원 시 F-key만 동작하는지)은
  검증하지 못했다.
- 쿼리 응답 시간 발견은 로컬 Docker 컨테이너 기준이며 절대값은 실제 운영/스테이징 네트워크(터널 경유)와
  다를 수 있다. 다만 동일 환경 안에서 `mysql` CLI 대비 상대적 차이가 일관되게 재현된 것은 유의미하다고
  본다.
- 세션 1회 관찰만으로는 인터뷰에서 언급된 "10회 정도 사용 후 신뢰 누적" 조건을 평가할 수 없다.

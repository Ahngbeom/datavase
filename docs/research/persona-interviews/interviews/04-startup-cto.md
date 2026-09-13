## Interview 04 — 최민석: DBA 없이 운영 DB를 관리하는 초기 스타트업 CTO

> 합성 페르소나 시뮬레이션이며 실제 사용자 인터뷰나 수요의 증거가 아니다.

### 페르소나

- 36세, B2B SaaS 공동창업자 겸 CTO, 개발 경력 12년
- Seed 단계, 개발자 9명·전체 인원 24명
- Kotlin/Spring Boot, MySQL 8, AWS RDS, Kubernetes
- DBA와 전담 보안팀이 없으며 백엔드 개발자가 장애 대응과 데이터 요청을 함께 처리
- 속도와 단순한 운영을 선호하지만, 반복되는 실수에는 즉시 최소한의 guardrail을 도입하는 성향
- 팀원은 대부분 DataGrip을 사용하고 장애 시 `kubectl exec → mysql`도 사용
- 운영 계정 분리는 필요하다고 생각하지만 권한 관리가 느슨하고 일부 senior 계정에는 write 권한이 남아 있음

### 인터뷰 목적

앞선 인터뷰에서 좁힌 초기 ICP가 실제로 존재하는지 검증한다. 특히 accidental write가 말로만 느끼는 불안인지, 별도 read-only DB 계정 대신 클라이언트 보호를 사용할 이유가 있는지, 팀 표준 도구로 지정할 만큼 사용 빈도와 가치가 있는지 확인한다.

### 인터뷰

**Interviewer:** 최근 팀에서 운영 DB를 직접 확인했던 일을 하나만 설명해주세요.

**민석:** 어제 고객사 한 곳의 정산 금액이 화면과 다르다는 문의가 들어왔어요. 담당 개발자가 로그에서 settlement id를 찾고, pod에 들어가 mysql로 원본 row와 집계 테이블을 확인했습니다. 데이터 문제는 아니고 캐시 갱신이 늦은 거였어요.

**Interviewer:** 이런 확인은 얼마나 자주 하나요?

**민석:** 개발팀 전체로 보면 거의 매일 누군가는 합니다. 고객 문의, 배치 실패, 결제 상태 확인이 많아요. 한 사람 기준으로는 주 1~3회 정도고요.

**Interviewer:** 그 과정에서 최근에 실수하거나 실수할 뻔한 일이 있었나요?

**민석:** 두 달 전에 운영에서 SELECT 하려다가 DataGrip console이 개발 datasource가 아니라 운영에 붙어 있었어요. 개발 데이터 정리용 DELETE를 실행했고 세 건이 지워졌습니다. 다행히 soft delete였고 audit table로 복구했습니다.

**Interviewer:** 그 뒤 무엇을 바꿨나요?

**민석:** 운영 datasource 색을 빨간색으로 바꾸고 자동 commit을 끄라고 공지했어요. 일부 계정은 read-only로 새로 만들었는데, 배치 확인 후 수동 보정까지 하는 senior들은 기존 계정을 계속 씁니다.

**Interviewer:** 왜 조회용과 수정용 계정을 완전히 분리하지 않았나요?

**민석:** 그게 맞는 방향인 건 압니다. 그런데 AWS 권한, secret, 개발자별 계정을 정리하는 일을 계속 미뤘어요. 장애 때 계정을 바꾸는 것도 번거롭다는 반응이 있고요. 솔직히 우선순위 문제입니다.

**Interviewer:** 그렇다면 클라이언트의 read-only 설정이 문제를 해결할까요?

**민석:** 완전히 해결하진 않죠. mysql로 접속하면 우회할 수 있으니까요. 그래도 평소 조회 경로를 read-only로 만드는 건 의미가 있어요. 수정할 때만 의식적으로 다른 도구나 설정을 쓰게 만들면 실수 확률이 낮아지니까요.

**Interviewer:** terminal MySQL client가 datasource마다 read-only session을 적용하고 결과를 grid로 보여준다면 사용해보겠습니까?

**민석:** 설치가 쉽고 접속이 기존보다 복잡하지 않으면 제가 먼저 써볼 것 같아요. 그런데 팀 표준으로 정하려면 설정 배포가 중요합니다.

**Interviewer:** 어떤 설정 배포를 기대하나요?

**민석:** datasource 이름, host, database, SSH 같은 비밀 아닌 설정은 repo에 예제로 두고 싶어요. 비밀번호나 token은 각자 keychain이나 환경변수로 넣고요. 새 팀원이 문서를 보고 5분 안에 붙을 수 있어야 합니다.

**Interviewer:** datasource YAML을 Git에 넣으면 충분할까요?

**민석:** 경로와 secret 분리가 명확하면요. OS마다 설정 위치가 다르면 bootstrap script나 `dv config import` 같은 게 있으면 좋겠어요. 하지만 중앙 관리 서버까지는 필요 없습니다.

**Interviewer:** 팀원에게 mysql 대신 반드시 이 도구만 쓰게 하겠습니까?

**민석:** 반드시라고 하면 못 지킵니다. emergency shell에는 mysql이 항상 있어야 해요. 대신 평소 운영 조회의 권장 경로로 지정할 수는 있습니다.

**Interviewer:** 기존 DataGrip의 production color와 무엇이 다른가요?

**민석:** 색은 경고이고 read-only session은 한 단계 더 강한 마찰이죠. 다만 DataGrip도 datasource를 read-only로 설정할 수 있어서, GUI 사용자에게 datavase를 강요할 이유는 약합니다. 터미널로 조회하는 사람에게 더 맞을 것 같아요.

**Interviewer:** 결과 CSV 저장은 얼마나 중요합니까?

**민석:** 가끔 CS 요청 때문에 필요하지만 운영 원본을 내려받는 건 조심해야 해요. 우리 팀에서는 조회 결과가 1,000행을 넘거나 민감 컬럼이 있으면 별도 스크립트를 리뷰받습니다. 편리하다고 무조건 열어두고 싶지는 않아요.

**Interviewer:** CSV 기능을 datasource별로 끌 수 있으면 팀 도입에 도움이 될까요?

**민석:** 약간은요. 하지만 사용자가 config를 바꿀 수 있으니 정책 통제라고 보진 않을 겁니다. 기본값과 실수 방지 측면에서만 의미가 있어요.

**Interviewer:** 다음 중 도입에 가장 큰 영향을 주는 것은 무엇인가요?

1. 더 많은 SQL 편집 기능
2. read-only와 명확한 환경 표시
3. query favorites
4. PostgreSQL 지원
5. 팀용 중앙 관리

**민석:** 2번입니다. 화면 어디에서나 `PRODUCTION / READ ONLY`가 보이면 좋겠어요. 쿼리를 실행하기 전에 지금 어디에 붙었는지 놓치는 게 실제 위험이거든요.

**Interviewer:** datasource가 read-only여도 session 명령으로 해제할 수 있습니다.

**민석:** 괜찮지만, 해제되는 순간 눈에 띄게 상태가 바뀌고 확인을 한 번 받아야 할 것 같아요. 조용히 바뀌면 보호 기능을 믿다가 더 위험할 수 있습니다.

**Interviewer:** 그 확인 기능이 없으면 사용하지 않을까요?

**민석:** 아니요. 현재도 mysql보다 낫다면 씁니다. 다만 README에서 우회 가능하다고 명확히 알려야 합니다.

**Interviewer:** 제품 설명으로 무엇이 가장 와닿습니까?

**민석:** `For the times you'd otherwise SSH in and run mysql`이 제일 정확해요. 그 아래에 read-only가 있다고 보여주면 됩니다. ‘안전한 production client’라고 하면 계정 권한이나 audit까지 기대하게 돼요.

**Interviewer:** 23초 demo에서 무엇을 보고 설치를 결정할까요?

**민석:** 설치 한 줄, 기존 SSH 설정으로 접속, 긴 결과가 읽기 좋게 나오는 장면, UPDATE가 막히는 장면이요. CSV는 뒤에 있어도 됩니다.

**Interviewer:** 팀 표준 도구로 시험하려면 어떤 조건이 필요합니까?

**민석:** macOS와 Linux binary가 안정적으로 나오고, Homebrew 설치가 되고, release checksum이 있고, 설정 예제를 repo에 둘 수 있으면 됩니다. 두세 명이 일주일 써보고 문제없으면 운영 조회 가이드에 넣을 수 있어요.

**Interviewer:** 유료라면요?

**민석:** 개인별 구독은 관리하기 싫습니다. 오픈소스라면 먼저 도입하고, 팀에서 계속 쓰면 연간 sponsor나 작은 팀 라이선스는 가능합니다. 중앙 관리 기능 때문에 돈을 내지는 않을 것 같아요.

**Interviewer:** 이 도구 대신 read-only 계정 정리를 먼저 해야 하지 않나요?

**민석:** 맞습니다. 사고를 진짜 막는 건 DB 권한이에요. 이 도구는 그 작업을 대체하면 안 됩니다. 다만 권한을 정리해도 사람이 production과 development를 혼동하는 문제와 터미널 결과가 불편한 문제는 남습니다.

### 결정적 발언

> “사고를 진짜 막는 건 DB 권한이에요. 이 도구는 그 작업을 대체하면 안 됩니다.”

> “반드시 쓰게 할 수는 없지만, 평소 운영 조회의 권장 경로로는 지정할 수 있습니다.”

> “화면 어디에서나 `PRODUCTION / READ ONLY`가 보여야 해요.”

### Interview 04 결과

이 페르소나는 현재까지 가장 높은 채택 가능성을 보였다. 문제가 실제로 자주 발생하고, 최근 accidental write 경험이 있으며, 새 binary를 시험하고 팀 가이드에 포함할 결정권도 있다.

다만 중요한 반론도 확인됐다. datavase의 read-only는 DB 권한 분리를 대체하는 보안 기능이 아니다. 가장 설득력 있는 역할은 다음과 같다.

> **평상시 운영 조회를 의도적으로 read-only 경로에 올려놓는 UX guardrail**

강제 표준이 아니라 권장 기본값이라는 점도 중요하다. 장애 대응 환경에는 mysql이 계속 남고, GUI 사용자는 DataGrip의 read-only 기능을 사용할 수 있다. 따라서 datavase는 팀 전체 DB 도구가 아니라 **터미널 운영 조회의 preferred path**가 될 가능성이 높다.

### 관찰된 도입 조건

- 개발팀 전체에서 운영 조회가 거의 매일 발생
- 실제 write 실수 또는 near miss 경험 존재
- 설치와 도구 선택에 대한 팀 내부 결정권 보유
- macOS와 Linux 배포 및 Homebrew 지원
- 비밀이 제거된 datasource 설정을 Git으로 공유 가능
- 신규 팀원이 5분 이내 접속할 수 있는 onboarding
- production과 read-only 상태가 항상 시각적으로 드러남
- mysql로 즉시 fallback 가능

---

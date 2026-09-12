## Interview 03 — 박준호: 규정이 엄격한 핀테크의 SRE/Platform Engineer

> 합성 페르소나 시뮬레이션이며 실제 사용자 인터뷰나 수요의 증거가 아니다.

### 페르소나

- 38세, SRE/Platform Engineer, 경력 11년
- 개발자 약 180명의 B2B 핀테크 회사
- Kubernetes, AWS, Terraform, Go, MySQL 8 운영
- 터미널과 tmux에 매우 익숙하며 장애 대응 중 운영 DB를 직접 조회함
- 운영 접근은 SSO 기반 단기 인증 → 승인된 bastion → DB proxy 순서
- 개인 생산성보다 재현성, 최소 권한, 감사 가능성, 공급망 검증을 우선함
- 새 도구에 반대하지는 않지만 “내 노트북에서 잘 동작한다”를 도입 근거로 인정하지 않음

### 인터뷰 목적

datavase에 적합해 보이는 terminal-native 운영 엔지니어도 조직 통제 환경에서는 채택하지 못하는지 확인한다. 특히 read-only의 안전 가치, binary 배포 방식, 비밀정보 처리, CSV 반출, 감사 추적을 검증한다.

### 인터뷰

**Interviewer:** 최근 운영 DB에 직접 접속했던 일을 하나만 처음부터 끝까지 설명해주세요.

**준호:** 결제 상태와 이벤트 처리 상태가 맞지 않는다는 알람이 있었습니다. 티켓을 만들고 당직 책임자 승인을 받은 다음, SSO로 30분짜리 접근 권한을 받았어요. bastion 세션에 들어가 승인된 mysql client로 세 건을 조회했습니다. 세션 입력과 출력은 기록되고요. 결과를 로컬 파일로 저장하지 않고 티켓에는 필요한 식별자만 마스킹해서 남겼습니다.

**Interviewer:** 그 과정에서 가장 번거로운 점은 무엇이었나요?

**준호:** 접속 절차보다 긴 결과를 읽는 게 불편합니다. 컬럼이 많으면 `\G`로 다시 실행하고 필요한 컬럼만 줄여서 봐요. 다만 불편하다고 접근 절차를 줄이고 싶지는 않습니다.

**Interviewer:** 다른 DB 클라이언트를 사용해본 적이 있나요?

**준호:** 로컬과 staging에서는 DataGrip을 씁니다. 운영은 팀에서 허용한 도구만 사용해요. 예전에 mycli도 검토했지만 Python 의존성과 패키지 공급망 검토 때문에 제외됐습니다.

**Interviewer:** single static binary이고 별도 runtime이 없는 터미널 MySQL 클라이언트라면 어떨까요?

**준호:** 검토는 쉬워집니다. 하지만 single binary가 곧 안전하다는 의미는 아닙니다. 누가 빌드했는지, release artifact와 source가 일치하는지, checksum과 서명이 있는지, 취약점이나 SBOM을 확인할 수 있는지가 먼저예요.

**Interviewer:** 개인이 Homebrew로 설치해서 사용해볼 수 있나요?

**준호:** staging까지는 가능할 수 있지만 운영 bastion에는 못 올립니다. immutable image라서 이미지 빌드와 보안 검토를 거쳐야 해요. 로컬에서 운영 DB로 직접 연결하는 경로도 없습니다.

**Interviewer:** 이 도구는 datasource에 `read_only: true`를 설정하면 session을 read-only로 만듭니다. 운영 조회가 더 안전해질까요?

**준호:** 방어층 하나로는 의미가 있습니다. 하지만 우리는 DB 계정 자체가 조회 전용이라 그 기능 때문에 도입하지는 않을 것 같아요. 사용자가 session을 read-write로 되돌릴 수 있다면 보안 통제라고 부르면 안 됩니다.

**Interviewer:** 사고 방지 기능이라고 표현하면요?

**준호:** 그 표현은 정확합니다. 개발자가 로컬이나 작은 회사에서 실수하지 않게 해주는 기능이죠. 저희 환경에서는 서버 권한이 이미 막고 있으니 중복 보호입니다.

**Interviewer:** datasource 비밀번호를 OS keychain에 저장합니다. headless 환경에서는 환경변수를 쓸 수 있습니다.

**준호:** 정적 비밀번호를 저장하지 않습니다. 사내 인증 CLI가 발급하는 단기 credential이나 DB proxy token을 실행 시점에 받아야 해요. 환경변수도 프로세스나 진단 정보에 남을 수 있어서 선호하지 않습니다.

**Interviewer:** 그렇다면 어떤 방식이어야 하나요?

**준호:** credential command나 표준 credential helper를 호출할 수 있으면 됩니다. 다만 특정 회사 인증 체계를 제품에 직접 넣지는 않는 게 좋아요. stdin이나 외부 command 연동처럼 일반적인 경계만 제공하면 됩니다.

**Interviewer:** SSH tunnel 설정을 datasource에 포함할 수 있습니다.

**준호:** 개인 키 경로를 직접 받는 구현이면 사용하기 어렵습니다. 회사 SSH config, ProxyCommand, agent, short-lived certificate를 그대로 존중해야 합니다. 별도 SSH 구현이 표준 설정과 다르게 동작하면 오히려 장애 지점이 하나 늘어요.

**Interviewer:** 결과를 CSV로 저장할 수 있습니다. 운영 대응에 유용할까요?

**준호:** 운영에서는 오히려 위험합니다. 개인정보나 결제 데이터가 로컬 디스크에 남을 수 있으니까요. CSV 기능을 없애라는 건 아니지만, 운영 datasource에서는 관리자가 export를 끌 수 있으면 좋겠습니다.

**Interviewer:** 사용자가 설정 파일에서 다시 켤 수 있다면 충분할까요?

**준호:** 그건 정책 통제가 아니라 안내에 가깝습니다. 강제하려면 중앙 관리나 서버 측 통제가 필요해요. datavase가 그 영역까지 책임질 필요는 없다고 봅니다. README에서 보안 경계를 명확히 말하는 편이 더 중요합니다.

**Interviewer:** query audit log가 있다면 도입 가능성이 높아질까요?

**준호:** 클라이언트 로그는 사용자가 지울 수 있고 우회할 수도 있습니다. 감사 로그는 DB proxy나 bastion이 남겨야 합니다. 제품이 자체 감사를 약속하면 검증할 범위만 커져요.

**Interviewer:** 현재 기능만 놓고 mysql CLI 대신 쓰고 싶나요?

**준호:** 개인적으로는 표가 잘 보인다면 쓰고 싶습니다. 조직 차원에서는 교체 이점이 작아요. mysql은 이미 검토됐고 어디서든 동작하고 문서가 많습니다. 새 바이너리를 승인할 만큼 장애 대응 시간이 줄어드는지 증거가 필요해요.

**Interviewer:** 어떤 증거가 있으면 검토할까요?

**준호:** 실제 작업 기준 비교가 좋습니다. 긴 row 확인, 쿼리 재실행, 결과 일부 복사 같은 작업이 mysql보다 얼마나 빠른지요. 그리고 크래시나 접속 실패 때 터미널을 망가뜨리지 않고 즉시 mysql로 돌아갈 수 있어야 합니다.

**Interviewer:** README의 첫 문구가 “A safer MySQL client for the terminal”이라면 어떻게 받아들이나요?

**준호:** ‘safer’의 범위가 모호합니다. 보안팀은 안전하다는 근거를 찾고, 개발자는 write가 절대 불가능하다고 오해할 수 있어요. `Accidental-write protection for terminal MySQL workflows` 정도가 더 정확합니다.

**Interviewer:** 팀에 도입을 제안할 의향이 있나요?

**준호:** 지금은 staging에서 개인적으로 시험해볼 수는 있습니다. 운영 표준 도구로 제안하려면 signed release, checksums, SBOM, 명확한 credential·network 동작 문서, 유지보수 정책이 필요합니다. 기능을 더 많이 넣는 건 우선순위가 아닙니다.

### 결정적 발언

> “불편하다고 접근 절차를 줄이고 싶지는 않습니다.”

> “single binary가 곧 안전하다는 의미는 아닙니다.”

> “개인적으로 쓰고 싶은 것과 조직이 승인할 수 있는 것은 다른 문제예요.”

### Interview 03 결과

이 페르소나는 workflow와 사용 능력만 보면 강한 잠재 사용자지만, 운영 환경 채택은 낮다. read-only가 가장 강한 hook이라는 기존 가설은 **DB 계정 자체가 쓰기 가능한 중소 규모 조직**에서는 유효할 수 있으나, 최소 권한이 성숙한 조직에서는 중복 보호에 가깝다.

또한 `single binary`는 설치 편의성과 의존성 감소를 뜻하지만 엔터프라이즈 신뢰의 증거는 아니다. 엄격한 조직에서는 다음이 기능보다 먼저 평가된다.

- release checksum 및 artifact 서명
- 빌드 provenance와 SBOM
- 명확한 보안·유지보수 정책
- 표준 SSH config, agent, ProxyCommand와의 호환성
- 정적 비밀번호 외의 단기 credential 연동 경계
- read-only와 export가 제공하는 보호 범위의 정확한 문서화

query audit나 중앙 정책 관리까지 제품에 넣는 것은 권장되지 않는다. 이는 작은 클라이언트의 범위를 크게 넓히면서도 서버·proxy 기반 통제를 대체하지 못한다.

---

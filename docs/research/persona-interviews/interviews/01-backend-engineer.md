## Interview 01 — 김현우: 운영 DB를 가끔 확인하는 백엔드 엔지니어

> 합성 페르소나 시뮬레이션이며 실제 사용자 인터뷰나 수요의 증거가 아니다.

### 페르소나

- 34세, 백엔드 엔지니어, 경력 8년
- 개발자 약 35명의 B2B SaaS 회사
- Java/Kotlin, Spring Boot, MySQL 8
- 평소 DataGrip, 운영 장애 시 `SSH → mysql`
- iTerm2와 tmux에 익숙하지만 새 CLI 학습은 싫어함
- 운영 DB에서의 우발적 write를 가장 걱정함

### 주요 응답 기록

- 결과 컬럼이 많을 때 mysql CLI의 가독성과 복사가 불편하다.
- TUI grid만으로는 설치하지 않는다. mycli 등 대안이 이미 있기 때문이다.
- single binary와 Homebrew 설치는 시험 사용 장벽을 낮춘다.
- datasource 단위 `read_only: true`에서 처음으로 강한 관심을 보였다.
- CSV export는 실제 업무 가치가 있지만 단독 채택 이유는 아니다.
- keychain과 SSH tunnel은 운영 환경에서 중요한 신뢰·편의 조건이다.
- 많은 단축키는 부담스럽고, 클릭 또는 명시적 메뉴가 필요하다.
- DataGrip 대체재로는 보지 않지만 mysql CLI 대체재로는 가능성이 있다.
- 기능을 더 늘려 DataGrip처럼 되면 오히려 DataGrip을 계속 쓰겠다고 했다.
- 개인 구독 결제 의향은 낮고, 팀에서 널리 쓰면 후원 가능성은 있다.

### 결정적 발언

> “DataGrip 대체라고 하면 비교할 게 너무 많아요. `ssh → mysql` 대신 `ssh → dv`라고 하면 바로 이해돼요.”

> “이게 DataGrip 되기 시작하면 저는 그냥 DataGrip 씁니다.”

### Interview 01 해석

가장 적합한 초기 사용자는 DBA가 아니라 MySQL/MariaDB 운영 데이터를 가끔 확인하는 Backend, Platform, SRE 엔지니어다. `read-only + bastion + CSV + single binary`가 하나의 명확한 wedge를 만든다. 기능 수보다 신뢰성과 접속 성공률이 retention에 중요하다.

---

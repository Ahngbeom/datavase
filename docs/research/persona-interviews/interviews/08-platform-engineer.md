## Interview 08 — 이준석: 팀의 운영 도구를 표준화하는 Platform Engineer

> 합성 페르소나 시뮬레이션이며 실제 사용자 인터뷰나 수요의 증거가 아니다.

### 페르소나

- 37세, Platform Engineer, 경력 12년
- 개발자 약 80명의 B2B SaaS 회사에서 Kubernetes, AWS, CI/CD, 접근 권한과 운영 도구를 관리
- MySQL 8.0과 MariaDB를 사용하는 12개 서비스를 지원하며 DBA는 별도로 없음
- 개발자는 VPN 또는 bastion을 거쳐 production replica에 접근
- 현재 `mysql`, DataGrip, TablePlus, 사내 스크립트가 혼재함
- 새로운 도구의 개인 생산성보다 설치·설정 표준화, 보안 검토, 업그레이드, 장애 문의 비용을 먼저 평가
- “설치가 쉽다”와 “조직에 배포하기 쉽다”를 구분하며, 팀 전체 도입에 보수적임

### 인터뷰 목적

datavase가 개인용 terminal client를 넘어 팀의 production 조회 도구로 표준화될 가능성이 있는지 검증한다. 중앙 설정, credential 관리, 버전 고정, 감사 가능성, 지원 부담을 확인하되, 사용자가 실제로 겪은 운영 경험부터 질문한다.

### 인터뷰

**Interviewer:** 최근 팀의 DB 접근 방식 때문에 직접 대응한 일을 설명해주세요.

**준석:** 신규 입사자가 production replica에 접속하지 못한다는 문의였습니다. VPN은 연결됐는데 SSH key 경로와 포트 포워딩 명령이 위키의 예전 내용이었어요. 그걸 고치고 나니 이번에는 로컬 mysql client 버전 차이로 인증 오류가 났습니다.

결국 화면을 공유해서 하나씩 맞췄어요. 접속보다 환경 차이를 해결하는 데 시간이 더 들었습니다.

**Interviewer:** 이런 문의가 얼마나 자주 발생하나요?

**준석:** 신규 입사나 노트북 교체 때는 거의 발생합니다. 평소에도 분기마다 몇 번은 있어요. 자주 있는 일은 아니지만 한 번 생기면 Platform 팀이 30분에서 한 시간씩 붙습니다.

**Interviewer:** 현재 팀에서 production DB를 조회하는 표준 절차가 있습니까?

**준석:** 문서상으로는 있습니다. VPN 연결, bastion을 통한 tunnel 생성, read-only 계정으로 replica 접속 순서예요.

실제로는 사람마다 다릅니다. 어떤 사람은 DataGrip의 SSH tunnel을 쓰고, 어떤 사람은 shell script를 쓰고, 오래된 팀원은 직접 `ssh -L`을 입력합니다.

**Interviewer:** 여러 도구가 섞여 있는 것이 실제로 어떤 문제를 만드나요?

**준석:** 장애가 났을 때 공통 언어가 없어요. 누군가 “접속이 안 된다”고 하면 먼저 어느 클라이언트인지부터 물어야 합니다. 인증서 위치, SSH 옵션, timeout도 다르고요.

하지만 다양하다는 이유만으로 표준화를 강제할 생각은 없습니다. 기존 도구보다 팀 전체 비용이 확실히 줄어야 해요.

**Interviewer:** 최근 새로운 개발 도구를 팀에 배포했던 사례가 있나요?

**준석:** Kubernetes 접근용 wrapper CLI를 배포했습니다. 처음에는 binary만 전달하면 끝날 줄 알았어요. 실제로는 macOS Intel·Apple Silicon, Linux, 사내 인증서, 버전 호환성, 자동 업데이트 문의가 계속 생겼습니다.

도구 기능보다 배포와 지원 체계를 만드는 데 시간이 더 들었어요.

**Interviewer:** 그렇다면 single static binary는 어떤 가치가 있습니까?

**준석:** 출발점으로는 좋습니다. runtime과 driver 충돌이 없으니까요. 하지만 binary 하나라는 사실만으로 팀 배포가 쉬워지는 건 아닙니다.

Homebrew tap, Linux 패키지나 checksum, 고정 버전 설치, 이전 버전 다운로드, 릴리스 노트가 있어야 합니다. 누가 최신 버전을 쓰는지도 확인할 수 있어야 하고요.

**Interviewer:** datavase는 datasource를 설정하고 table 확인, SQL 실행, 결과 복사·CSV 저장만 제공합니다.

**준석:** 범위가 작은 건 좋습니다. 지원해야 할 동작이 적고 사용 목적도 설명하기 쉬우니까요.

다만 개발자들이 기존 DataGrip을 버리지는 않을 겁니다. “모든 DB 작업의 표준 도구”가 아니라 “production을 조회할 때 사용하는 승인된 경량 도구” 정도라면 검토할 수 있습니다.

**Interviewer:** datasource 설정을 저장해두면 사용자는 `dv open production`처럼 접속할 수 있습니다.

**준석:** 각자가 YAML을 작성해야 한다면 문의가 줄지 않습니다. 들여쓰기, host, SSH key 경로가 조금씩 달라지고 결국 개인별 설정을 디버깅하게 돼요.

조직이 안전한 기본 설정을 배포하고 사용자는 자신의 credential만 연결할 수 있어야 합니다.

**Interviewer:** 중앙에서 공유할 datasource 설정이 필요하다는 뜻인가요?

**준석:** 네. 다만 password나 개인 SSH key 경로까지 공유하면 안 됩니다.

공유 가능한 부분과 개인별 부분을 분리해야 해요. 예를 들면 host, port, database, bastion, `read_only: true`는 저장소에서 관리하고, username이나 credential 참조는 로컬 override로 두는 방식입니다.

**Interviewer:** 설정 파일 하나를 팀 저장소에 두고 복사하도록 안내하면 충분할까요?

**준석:** 처음에는 가능하지만 곧 drift가 생깁니다. replica 주소가 바뀌거나 datasource가 추가됐을 때 모든 사람이 다시 복사해야 하니까요.

적어도 설정의 출처와 버전을 표시하거나, 관리되는 설정과 로컬 설정을 병합할 수 있어야 합니다. 꼭 원격 설정 서버가 필요한 건 아니고 Git으로 배포해도 됩니다.

**Interviewer:** `read_only: true`를 설정하면 모든 connection에서 session-level read-only를 적용합니다.

**준석:** 개인 실수 방지에는 가치가 있습니다. 하지만 팀 표준으로 채택할 때는 config 값만 믿지 않을 겁니다. DB 계정 자체도 read-only여야 합니다.

클라이언트 옵션은 보조 안전장치이고, 접근 권한은 서버에서 통제해야 합니다.

**Interviewer:** 그렇다면 session read-only는 팀 도입 결정에 중요하지 않습니까?

**준석:** 중요합니다. 계정 권한 설정이 완벽하더라도 사람이 잘못된 datasource에 연결하거나 권한 변경이 늦게 반영될 수 있으니까요.

다만 “이 도구를 쓰면 안전하다”는 메시지보다는 “서버 권한 위에 한 겹 더 두는 실수 방지 장치”라고 설명해야 합니다.

**Interviewer:** 개발자가 read-only를 직접 해제할 수 있다면요?

**준석:** 개인 도구라면 괜찮지만 조직의 승인된 조회 경로라면 정책이 애매해집니다. 해제가 필요할 정도면 다른 절차로 들어가야 해요.

최소한 관리되는 datasource에서는 사용자가 임의로 해제했을 때 눈에 띄는 경고가 나오고, 그 세션이 더 이상 표준 조회 세션이 아니라는 것을 보여줘야 합니다.

**Interviewer:** audit log가 있어야 표준 도구로 도입할 수 있을까요?

**준석:** 클라이언트가 별도 감사 시스템을 만드는 건 원하지 않습니다. 민감한 SQL과 결과를 로컬에 더 저장하면 또 다른 보안 문제가 됩니다.

DB audit log나 bastion access log와 연결할 수 있도록 connection metadata를 명확하게 남기는 편이 낫습니다. query history가 있다면 저장 위치, 보존 기간, 비활성화 방법도 알아야 하고요.

**Interviewer:** 로컬 query history가 기본으로 저장된다면 어떤 점을 확인하겠습니까?

**준석:** 평문 저장 여부와 파일 권한부터 봅니다. SQL에 이메일, 전화번호, 토큰이 들어갈 수 있어요. CSV export도 마찬가지입니다.

기능을 막을 필요는 없지만 보안팀이 정책을 정할 수 있도록 history 비활성화, 저장 경로, 파일 permission을 제어할 수 있어야 합니다.

**Interviewer:** 팀 배포 전에 어떤 검증을 하시겠습니까?

**준석:** 먼저 Platform 팀 2~3명이 실제 production replica와 staging에서 사용합니다. VPN 재연결, bastion timeout, credential 만료, DB failover 같은 정상적이지 않은 상황을 봐야 해요.

그다음 backend 팀 한 곳에서 2주 정도 pilot을 하고 문의 건수와 기존 mysql로 돌아간 이유를 기록하겠습니다.

**Interviewer:** 어떤 결과가 나오면 배포를 중단하겠습니까?

**준석:** 접속 실패 원인이 UI에서 구분되지 않거나 config 문법 문의가 반복되면 중단합니다. 도구가 편리해도 Platform 팀의 티켓이 늘어나면 도입할 이유가 없어요.

업데이트할 때 기존 config가 깨지는 경우도 치명적입니다.

**Interviewer:** 자동 업데이트 기능은 필요합니까?

**준석:** 강제 자동 업데이트는 싫습니다. 장애 시점에 팀원의 실행 파일이 서로 다르면 재현이 어려워져요.

우리가 검증한 버전을 pin하고, 새 버전은 pilot 후 올리고 싶습니다. 현재 버전과 config schema version이 `dv doctor` 같은 명령으로 한 번에 보여야 지원하기 쉽습니다.

**Interviewer:** `dv doctor`에서는 어떤 정보가 필요합니까?

**준석:** binary version, OS와 architecture, config 파일 위치와 schema 오류, datasource 이름, credential provider 상태, bastion 도달 가능 여부 정도요.

비밀번호나 실제 key 내용은 절대 출력하면 안 됩니다. 지원 요청에 붙여넣어도 안전한 결과여야 합니다.

**Interviewer:** 별도의 진단 명령은 현재의 작은 제품 범위를 깨뜨리지 않을까요?

**준석:** SQL 기능을 늘리는 것과 운영 가능성을 높이는 것은 다릅니다. query tabs나 schema editing은 제품 범위를 넓히지만, 진단 가능성은 현재 기능을 팀에서 유지할 수 있게 만듭니다.

다만 처음부터 거대한 doctor 기능을 만들 필요는 없어요. 명확한 오류 분류와 `--version`, config validation부터 시작하면 됩니다.

**Interviewer:** README의 어떤 설명이 팀 도입 검토에 가장 도움이 될까요?

**준석:** 개인의 편리함보다 운영 모델이 보여야 합니다.

지원 OS, 설치·업데이트 방식, config와 credential의 분리, read-only의 정확한 범위, history와 export 파일의 저장 방식이 한 페이지에 있어야 합니다. 20초 demo는 관심을 만들 수 있지만 승인에는 도움이 부족합니다.

**Interviewer:** 팀원에게 datavase 사용을 의무화할 수 있습니까?

**준석:** 바로 의무화하지는 않습니다. production read-only 접근에서만 권장하고, 실제로 mysql보다 빠르고 문의가 적은지 봅니다.

강제하려면 보안이나 운영상 명백한 이점이 있어야 합니다. 단순히 UI가 더 좋다는 이유로 개발자의 도구 선택권을 제한하면 반발만 생겨요.

**Interviewer:** pilot에서 성공을 판단할 기준은 무엇입니까?

**준석:** 신규 사용자가 문서만 보고 10분 안에 접속하는지, 2주 동안 Platform 지원 없이 다시 사용할 수 있는지, 기존 mysql로 돌아간 이유가 무엇인지 보겠습니다.

설치 수보다 독립적으로 재사용한 사람이 더 중요해요.

**Interviewer:** 현재 상태에서 직접 pilot을 제안하시겠습니까?

**준석:** 팀 공유 설정과 credential 분리가 가능하고, 버전을 고정할 수 있으며, 오류 메시지가 충분하다면 작은 팀에서 해볼 수 있습니다.

개발자 한 명이 좋아한다는 이유만으로 전사 표준은 어렵습니다. 다섯 명이 별도 도움 없이 반복 사용하고 지원 비용이 줄어드는 것을 먼저 보여줘야 합니다.

### 결정적 발언

> “설치가 쉬운 것과 조직에 배포하기 쉬운 것은 다른 문제입니다.”

> “도구가 편리해도 Platform 팀의 티켓이 늘어나면 도입할 이유가 없습니다.”

> “설치 수보다 지원 없이 다시 사용한 사람이 더 중요합니다.”

### Interview 08 결과

Platform Engineer에게 datavase의 작은 기능 범위와 single binary는 긍정적이지만, 이것만으로 팀 표준화의 근거가 되지는 않았다. 팀 도입의 경쟁 대상은 다른 DB IDE뿐 아니라 현재 유지되는 위키, shell script, credential 정책과 지원 절차 전체다.

표준화 가능성을 결정하는 핵심은 새로운 SQL 기능보다 다음 운영 특성이다.

| 평가 영역 | 개인 사용에 필요한 수준 | 팀 표준화에 필요한 수준 |
|---|---|---|
| 설치 | binary를 실행할 수 있음 | 검증된 버전 고정, checksum, rollback 가능 |
| datasource | 로컬 설정 작성 | 관리 설정과 개인 override 분리, 변경 배포 |
| credential | 비밀번호를 config에 저장하지 않음 | 승인된 provider 사용과 정책 적용 |
| read-only | 우발적 write 방지 | DB 권한과 병행하고 실제 session 상태 확인 |
| 오류 처리 | 사용자가 원인을 추측 | 지원 담당자가 재현할 수 있는 오류 분류 |
| history·CSV | 편리하게 저장 | 저장 위치·권한·비활성화 정책 제어 |
| 지원 | 개인이 해결 | 안전하게 공유할 수 있는 진단 정보 |
| 성공 측정 | 한 번 설치 | 지원 없이 반복 사용하고 기존 도구 회귀 감소 |

따라서 초기 positioning은 “전사 DB 표준 도구”보다 **“production read-only 접근을 위한 팀 배포 가능한 terminal client”**로 제한하는 편이 적절하다. 팀 확장은 설치 수가 아니라 독립적인 재사용률과 지원 문의 감소로 검증해야 한다.

### 검증 후보

1. 관리되는 공통 datasource와 개인별 credential·override를 분리할 수 있는지 검증
2. Homebrew와 Linux 배포에서 특정 버전 pin, checksum 확인, 이전 버전 복구 절차 점검
3. config schema 변경 시 하위 호환성과 validation 오류 메시지 테스트
4. VPN, bastion, credential, DNS, DB 인증 실패를 사용자가 구분할 수 있는지 확인
5. query history와 CSV의 저장 위치, 파일 권한, 비활성화 옵션 검토
6. 민감정보 없이 공유 가능한 최소 진단 출력 정의
7. 5명 규모의 2주 pilot에서 첫 접속 시간, 독립 재사용률, 지원 문의 수, mysql 회귀 이유 측정
8. read-only 표시가 관리 설정값이 아니라 실제 session 상태를 반영하는지 확인

---

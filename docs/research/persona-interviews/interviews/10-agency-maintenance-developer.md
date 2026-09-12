## Interview 10 — 이준서: 여러 고객사의 시스템을 유지보수하는 에이전시 개발자

> 합성 페르소나 시뮬레이션이며 실제 사용자 인터뷰나 수요의 증거가 아니다.

### 페르소나

- 36세, 웹 개발 에이전시 Backend·Maintenance Engineer, 경력 11년
- 동시에 12개 고객사의 쇼핑몰, 예약, 사내 업무 시스템을 유지보수
- 고객사마다 개발·스테이징·운영 DB가 있고 MySQL 5.7, MySQL 8.0, MariaDB가 혼재
- VPN, SSH bastion, 직접 접속 등 고객사별 접근 방식이 다름
- 장애·문의가 들어오면 로그에서 고객·주문·회원 식별자를 찾아 해당 DB에서 데이터를 확인
- DataGrip을 주로 사용하지만 긴급 대응이나 원격 서버에서는 mysql CLI도 사용
- 새로운 기능보다 **잘못된 고객사·환경에 접속하지 않는 것**과 credential 관리 방식을 우선함
- 하나의 도구가 모든 datasource를 관리하는 것은 편리하지만, 그만큼 오접속과 credential 집중 위험도 커진다고 봄

### 인터뷰 목적

여러 고객사의 datasource를 오가는 유지보수 개발자에게 중앙화된 datasource 관리가 실제 가치를 주는지 검증한다. 특히 고객사·환경 식별, production read-only 기본값, credential 저장, SSH 설정, 설정 공유와 감사 가능성이 전환 조건인지 확인한다.

### 인터뷰

**Interviewer:** 최근 고객사 데이터베이스에 직접 접속했던 상황을 처음부터 설명해주세요.

**준서:** 고객사 CS팀에서 결제는 됐는데 주문 상태가 바뀌지 않았다는 연락이 왔습니다. 티켓에서 고객사와 주문번호를 확인하고, 그 고객사의 VPN을 켠 다음 DataGrip에서 production datasource를 찾았어요. 주문 테이블과 결제 로그 테이블을 조회해서 외부 결제 승인번호를 비교했습니다.

**Interviewer:** 그 과정에서 가장 오래 걸린 부분은 무엇이었나요?

**준서:** 쿼리 자체보다 접속 준비입니다. 어느 VPN을 켜야 하는지, bastion이 필요한지, production host가 무엇인지 확인하는 시간이 더 걸려요. 고객사가 많아서 기억에 의존하면 안 됩니다.

**Interviewer:** datasource 이름을 보면 환경을 바로 구분할 수 있습니까?

**준서:** 저희가 이름을 잘 붙여놓으면요. 그런데 사람마다 규칙이 달라서 `client-a`, `client-a-new`, `client-a-prod2` 같은 이름이 남습니다. 인수인계를 받으면 무엇이 현재 운영인지 다시 확인해야 해요.

**Interviewer:** 실제로 잘못된 datasource에 접속한 적이 있나요?

**준서:** 있습니다. 운영 이슈를 재현하면서 staging을 보고 있어서 데이터가 없다고 판단한 적이 있어요. 반대로 개발 데이터를 정리한다고 생각하고 production에 접속한 적도 있습니다. 실행 직전에 host를 보고 알아차려서 사고는 없었지만 식은땀이 났죠.

**Interviewer:** 그 뒤 workflow를 바꿨습니까?

**준서:** production datasource는 이름 앞에 `PROD`를 붙이고 색상을 빨간색으로 했어요. 가능하면 DB 계정도 조회 전용으로 요청합니다. 다만 작은 고객사는 계정을 하나만 주기도 해서 클라이언트 쪽 주의에 의존하는 경우가 있습니다.

**Interviewer:** mysql CLI에서 고객사를 전환할 때는 어떻게 합니까?

**준서:** 사내 위키에서 명령어를 복사하거나 개인 shell alias를 씁니다. 문제는 alias에 host와 user가 들어가고, SSH 옵션까지 붙으면 오래된 설정인지 확인하기 어렵다는 거예요. 비밀번호를 명령어나 파일에 직접 넣지는 않지만 팀원마다 관리 방식이 다릅니다.

**Interviewer:** datavase가 여러 datasource와 SSH tunnel 설정을 한곳에서 관리한다면 도움이 될까요?

**준서:** 반복 입력은 줄겠죠. 하지만 datasource가 많아질수록 선택 화면이 안전해야 합니다. 비슷한 고객사 이름을 검색하다가 다른 production을 열 수 있으니까요. 단순히 목록이 있다는 것만으로는 부족합니다.

**Interviewer:** 어떤 정보가 보여야 합니까?

**준서:** 최소한 고객사, 환경, host, database, 접속 방식이 보여야 합니다. datasource 이름 하나만 신뢰하고 싶지는 않아요. 선택 후 SQL 화면에서도 현재 고객사와 `PROD` 여부가 계속 보여야 하고요.

**Interviewer:** datasource별로 `read_only: true`를 설정할 수 있습니다.

**준서:** production에 기본 적용할 수 있다면 의미 있습니다. 저희는 데이터를 확인하는 일이 대부분이고 수정은 관리자 기능이나 승인된 스크립트로 처리하거든요. 다만 잘못된 고객사의 데이터를 조회하는 것까지 막아주지는 않겠죠.

**Interviewer:** 맞습니다. session-level read-only는 write 실수를 줄이지만 권한 통제나 오접속을 막는 기능은 아닙니다.

**준서:** 그 구분이 중요합니다. 다른 고객사의 개인정보를 열어보는 것도 사고라서 write만 막는다고 안전한 것은 아니에요. 제품이 “production safe”라고만 말하면 범위를 과대평가할 수 있습니다.

**Interviewer:** datasource를 열 때 고객사와 환경을 한 번 더 확인시키는 단계가 필요할까요?

**준서:** 모든 접속에 확인창이 나오면 금방 무시하게 됩니다. production이나 처음 보는 datasource에만 강한 표시를 주는 편이 나아요. 고객사명과 host를 보여주고 바로 Enter가 아니라 명시적으로 선택하게 하면 좋겠습니다.

**Interviewer:** 여러 고객사를 오갈 때 query history는 어떤가요?

**준서:** 편리하지만 섞이면 위험합니다. A 고객사에서 실행한 쿼리와 검색값이 B 고객사 화면에 나오면 안 됩니다. 쿼리 안에 이메일, 전화번호, 주문번호 같은 개인정보가 들어갈 수 있어요. history는 datasource별로 분리되거나 저장하지 않는 설정이 필요합니다.

**Interviewer:** CSV export는 자주 사용합니까?

**준서:** 고객사 요청 때문에 종종 씁니다. 그런데 그 파일도 위험해요. 다운로드 폴더에 고객 데이터가 계속 남을 수 있거든요. 저장 위치와 파일명을 명확히 확인해야 하고, 어느 datasource에서 만든 파일인지 나중에 알 수 있으면 좋겠습니다.

**Interviewer:** export 파일명에 고객사나 환경을 자동으로 넣으면 좋을까요?

**준서:** 기본값에는 도움이 됩니다. 다만 고객사명이 파일명에 들어가는 것 자체가 정책 위반인 곳도 있으니 강제하면 안 됩니다. 중요한 것은 자동 이름보다 저장 직전에 경로와 대상 환경이 보이는 겁니다.

**Interviewer:** credential은 OS keychain에 저장하고 headless 환경에서는 environment variable을 사용할 수 있습니다.

**준서:** 개인 Mac에서는 keychain이 좋습니다. 하지만 팀 공용 config에 credential 식별자나 환경변수 이름까지 들어가는 방식은 검토가 필요해요. 고객사별 비밀번호가 한 파일에 평문으로 모이면 설치하지 않을 겁니다.

**Interviewer:** datasource 설정을 팀과 공유하고 secret만 각자 주입하는 구조라면요?

**준서:** 그게 이상적입니다. host, port, database, SSH 구조 같은 비밀이 아닌 설정은 저장소에서 리뷰하고, 실제 password나 SSH key는 각자 보관하는 방식이요. 다만 고객사 계약에 따라 host 정보도 외부 저장소에 올리면 안 될 수 있으니 공유 범위를 선택할 수 있어야 합니다.

**Interviewer:** 설정 파일을 Git으로 관리하고 싶습니까?

**준서:** 일부는요. 설정 변경 이력이 남고 퇴사자 개인 alias에 의존하지 않는다는 장점이 있습니다. 하지만 secret이 실수로 commit되지 않는 구조가 먼저 필요합니다. 문서에 “비밀번호를 넣지 마세요”라고 쓰는 정도로는 부족해요.

**Interviewer:** datasource config와 secret reference를 분리하면 전환할 의향이 높아질까요?

**준서:** 높아집니다. 특히 신규 입사자가 승인된 datasource 목록을 받고 본인 credential만 연결할 수 있다면 onboarding이 쉬워져요. 반대로 설정을 각자 처음부터 만들면 DataGrip과 크게 다르지 않습니다.

**Interviewer:** 고객사마다 DB 버전과 접속 방식이 다른 것은 문제가 될까요?

**준서:** 매우 중요합니다. MySQL 5.7에서는 되고 MariaDB에서는 깨지는 식이면 긴급 상황에서 못 씁니다. SSH tunnel도 password, key, jump host 조합이 다양해요. 지원한다고 적힌 조합은 실제 integration test가 있어야 믿습니다.

**Interviewer:** DataGrip 대신 datavase를 사용할 것 같습니까?

**준서:** 개발과 복잡한 분석은 DataGrip을 계속 씁니다. 하지만 고객 문의를 받아 몇 줄 조회하고 CSV를 전달하는 작업이라면 더 빠를 수 있어요. 실행이 가볍고 현재 고객사·환경이 확실히 보인다는 조건에서요.

**Interviewer:** mysql CLI 대신은요?

**준서:** 가능성이 더 높습니다. 고객사별 접속 명령을 찾아 복사하는 일을 없앨 수 있으니까요. 다만 datasource를 잘못 선택할 위험이 alias보다 커지면 바꾸지 않습니다.

**Interviewer:** 현재 기능 중 가장 중요한 것은 무엇입니까?

**준서:** grid나 CSV보다 datasource context입니다. 제가 지금 어느 고객사의 어느 환경을 보고 있는지 절대 헷갈리지 않아야 해요. read-only는 그다음 보호층이고요.

**Interviewer:** 기능 하나를 추가한다면 무엇을 원합니까?

**준서:** 새로운 DB 기능보다 datasource를 고객사와 환경 기준으로 그룹화하고, production 표시를 강하게 하는 쪽입니다. 그리고 history·export·credential이 datasource 경계를 넘지 않는다는 보장이 필요합니다.

**Interviewer:** 설치를 중단하게 만드는 조건은 무엇입니까?

**준서:** config에 비밀번호를 평문으로 넣어야 하거나, 모든 query history가 하나의 파일에 섞여 저장되거나, 현재 host를 화면에서 바로 확인할 수 없으면 중단합니다. 여러 고객사의 접근 정보를 한 도구에 모으는 만큼 작은 모호함도 위험합니다.

**Interviewer:** README demo에서 무엇을 보여줘야 할까요?

**준서:** datasource를 고르는 장면보다 선택 후 화면을 보여주세요. `고객사 A / PROD / host / read-only`가 계속 보이고, 다른 고객사로 전환하면 context와 history가 분리되는 장면이요. UPDATE가 막히는 장면만으로는 저희 업무의 가장 큰 위험을 설명하지 못합니다.

**Interviewer:** 지금 상태에서 시험해볼 의향이 있습니까?

**준서:** 개인 테스트 환경에서는 해볼 수 있습니다. 실제 고객사 정보를 등록하려면 config 구조, secret 저장 위치, history 파일 위치와 삭제 방법부터 확인할 겁니다. 그 문서가 명확하고 지원하는 SSH 조합이 맞으면 단순 조회 업무부터 써보겠습니다.

### 결정적 발언

> “저희에게 가장 위험한 것은 SQL을 잘못 쓰는 것만이 아니라, 맞는 SQL을 다른 고객사의 DB에서 실행하는 것입니다.”

> “여러 datasource를 한곳에 모으는 편리함은 그 경계가 명확할 때만 장점입니다.”

> “grid보다 먼저, 지금 어느 고객사의 어느 환경에 연결됐는지가 계속 보여야 합니다.”

### Interview 10 결과

여러 고객사를 유지보수하는 개발자에게 datasource 중앙화는 분명한 효율을 제공한다. 고객사별 VPN·SSH·host 정보를 찾고 접속 명령을 조립하는 반복 작업을 줄일 수 있으며, 단순 조회와 CSV 반출에서는 DataGrip보다 가벼운 workflow가 성립한다.

그러나 datasource 수가 많아지면 중앙화 자체가 새로운 위험을 만든다. 이 페르소나가 우려하는 핵심은 단순한 accidental write가 아니라 **고객사와 환경 경계의 혼동**이다.

| 영역 | 기대하는 가치 | 전환을 막는 위험 |
|---|---|---|
| datasource 관리 | 고객사별 접속 절차 단축 | 유사한 이름으로 인한 오접속 |
| environment 표시 | 현재 작업 맥락의 즉각적 확인 | 이름만 표시하거나 화면 이동 후 context 소실 |
| session read-only | production write 실수 감소 | 다른 고객사 데이터 조회, 권한 통제 대체 오해 |
| credential 관리 | 평문 password 제거 | secret 집중, config에 credential 유출 |
| query history | 반복 조사 속도 향상 | 고객사 간 query·개인정보 혼합 |
| CSV export | 고객 요청 대응 단축 | 민감 데이터의 로컬 잔존과 출처 혼동 |
| config 공유 | 팀 onboarding과 변경 이력 개선 | secret 또는 고객사 접속 정보의 잘못된 commit |
| SSH tunnel | 고객사별 접속 명령 제거 | 다양한 인증·jump host 조합의 불완전한 지원 |

이 사용자에게 datavase의 핵심 가치는 “많은 datasource를 저장한다”가 아니라 **저장된 datasource 사이의 고객사·환경·secret 경계를 명확하게 유지한다**는 데 있다. 따라서 read-only만으로 안전성을 설명하기보다 현재 연결 context, 로컬 기록의 격리, credential 저장 범위를 함께 검증해야 한다.

초기 ICP로는 조건부 적합하다. 여러 MySQL 고객사를 반복적으로 조회하는 에이전시·외주 유지보수 개발자는 실제 workflow 이득이 크다. 다만 이 집단을 적극적으로 겨냥하려면 기능 확장보다 datasource 식별과 로컬 데이터 보관 정책에 대한 신뢰가 먼저 필요하다.

### 검증 후보

다음 항목은 이 페르소나의 가정을 실제 사용자 인터뷰와 제품 테스트로 검증하기 위한 후보이다.

1. 최근 3개월 동안 유지보수 개발자가 관리한 고객사·환경별 datasource 수와 주간 전환 횟수
2. 잘못된 고객사 또는 staging·production을 조회한 near-miss가 실제로 얼마나 발생하는지
3. datasource 선택 화면과 SQL 화면에서 고객사·환경·host 중 반드시 보여야 하는 정보
4. production에만 추가 확인을 요구할 때 안전성과 반복 피로 사이의 균형
5. query history가 datasource별로 분리되고 저장을 비활성화·삭제할 수 있는지
6. clipboard, CSV, 최근 검색어 등 로컬에 남는 데이터의 위치·보존 기간·삭제 방법
7. config와 secret reference를 분리해 팀에서 공유할 수 있는지와 실수로 secret을 commit할 가능성
8. OS keychain, environment variable, SSH agent 등 credential source별 실제 사용성과 실패 동작
9. 고객사별 MySQL·MariaDB 버전과 SSH·VPN·jump host 조합에 대한 integration test 범위
10. `고객사 / 환경 / host / schema / 실제 session read-only 상태`가 reconnect 후에도 정확히 유지되는지

---

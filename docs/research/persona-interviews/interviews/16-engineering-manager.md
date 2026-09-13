# Interview 16 — 이서준: 팀 도구 도입을 결정하는 Engineering Manager

> 합성 페르소나 시뮬레이션이며 실제 사용자 인터뷰나 수요의 증거가 아니다.

### 페르소나

- 39세, Engineering Manager, 개발 경력 13년·관리 경력 4년
- Backend 3개 스쿼드, 총 17명의 엔지니어를 관리하는 B2B SaaS 조직 소속
- Java·Go 서비스와 MySQL 8을 운영하며 별도 DBA 없이 Platform Engineer 2명이 접근 정책과 공용 도구를 관리
- 본인도 장애 대응에 참여하지만 평소 SQL 조회 빈도는 월 1~2회로 낮음
- 팀원들은 DataGrip, mysql CLI, 사내 bastion wrapper를 혼용하며 production write 권한은 제한적으로 부여
- 개인 생산성보다 onboarding 시간, 운영 사고, 지원 부담, 보안 검토 비용을 합쳐 도구 도입을 판단
- 새로운 도구를 전사 표준으로 지정하기보다 소규모 pilot에서 행동 데이터가 확인된 뒤 권장 목록에 올리는 편
- 무료·오픈소스라도 설치, 업그레이드, 문서화, 장애 대응에는 조직 비용이 든다고 봄

### 인터뷰 목적

datavase가 개인 개발자의 선호를 넘어 팀에 권장되는 도구가 될 수 있는지 확인한다. 특히 Engineering Manager가 도입·확산을 승인할 때 요구하는 생산성 증거, 안전성 기준, opportunity cost, 내부 지원 비용과 adoption metric을 검증한다.

### 인터뷰

**Interviewer:** 최근 팀에 새로운 개발 도구를 도입하거나 거절했던 사례를 설명해주세요.

**서준:** 세 달 전에 production 로그 조회 CLI를 검토했습니다. 두 명이 먼저 써보고 반복 검색이 빨라졌다고 했지만, 나머지는 기존 웹 UI로 충분하다고 했어요. 인증 갱신 오류가 두 번 있었고 Platform 팀이 설치 질문을 계속 받았습니다. 결국 필수 도구로 지정하지 않고 개인 선택으로 남겼습니다.

**Interviewer:** 두 명이 빨라졌는데도 표준 도구가 되지 못한 이유는 무엇입니까?

**서준:** 개인의 만족과 팀 생산성은 다릅니다. 두 명이 아낀 시간보다 열다섯 명이 새 사용법을 배우고 Platform 팀이 지원하는 시간이 더 클 수 있어요. 기존 도구보다 명확히 나은 반복 업무가 있어야 합니다.

**Interviewer:** 그 판단에 어떤 자료를 사용했나요?

**서준:** pilot 참여자의 주간 사용 횟수, 같은 작업에 걸린 시간, 인증 실패와 문의 건수를 봤습니다. 만족도도 받았지만 말보다 실제로 두 번째, 세 번째 사용이 일어났는지를 더 중요하게 봤어요.

**Interviewer:** 팀에서 production MySQL을 직접 조회하는 일은 얼마나 자주 있습니까?

**서준:** 사람마다 다릅니다. 온콜 담당자와 Platform Engineer는 주 몇 번, 일반 Backend Engineer는 한 달에 한두 번입니다. CS 요청 때문에 데이터를 확인하는 스쿼드는 조금 더 많고요. 전원이 매일 쓰는 workflow는 아닙니다.

**Interviewer:** 현재는 어떤 방식으로 조회합니까?

**서준:** 대부분 사내 wrapper로 bastion tunnel을 열고 mysql CLI나 DataGrip에 연결합니다. 간단한 장애 확인은 CLI, 여러 테이블을 분석할 때는 DataGrip입니다. 읽기 전용 계정이 기본이고 write는 별도 승인을 받아요.

**Interviewer:** 현재 방식에서 팀 차원의 문제는 무엇인가요?

**서준:** 도구가 여러 개라서 새 팀원이 누구의 방법을 따라야 할지 모릅니다. mysql 출력이 불편해 잘못된 row를 복사하는 일도 있고, DataGrip의 tunnel 설정을 도와주는 데 시간이 듭니다. 다만 심각한 장애가 매주 생기는 정도는 아닙니다.

**Interviewer:** accidental write에 대한 걱정은 있습니까?

**서준:** 당연히 있지만 계정 권한으로 먼저 막습니다. 클라이언트의 read-only는 추가 보호로는 좋지만 보안 통제를 대체할 수 없습니다. 그 기능만 보고 팀 도구를 바꾸지는 않아요.

**Interviewer:** datavase는 단일 binary이고 datasource 단위 session read-only, SSH tunnel, 결과 grid, CSV export를 제공합니다. 첫 반응은 어떤가요?

**서준:** 개인이 mysql 대신 시험해볼 수 있는 구성으로는 이해됩니다. 하지만 팀 도입 관점에서는 설치가 간단하다는 것보다 버전 관리, config 공유, credential 처리, 장애가 났을 때 누가 지원하는지가 먼저 떠오릅니다.

**Interviewer:** 오픈소스이고 무료라면 도입 장벽이 낮지 않나요?

**서준:** 구매 결재는 없겠지만 비용이 0은 아닙니다. 보안 검토, Homebrew나 이미지 배포, 사용 가이드, 업데이트 확인, 퇴사자 설정 정리까지 모두 시간입니다. 무료라는 이유만으로 표준 도구를 하나 더 늘리지는 않습니다.

**Interviewer:** 한 명이 자발적으로 설치해 생산성이 좋아졌다고 하면요?

**서준:** 그 사람은 계속 쓰면 됩니다. 팀 권장 도구로 올리는 결정은 별개예요. 비슷한 workflow를 가진 세 명 이상이 반복적으로 쓰고, 기존 방식보다 실패가 늘지 않아야 논의할 수 있습니다.

**Interviewer:** 어느 정도의 생산성 개선이면 충분합니까?

**서준:** “쿼리 한 번이 몇 초 빨라졌다”만으로는 부족합니다. 접속부터 필요한 값을 공유하기까지 전체 시간이 줄어야 해요. 예를 들어 온콜 조사 한 건당 5분을 줄이고 그 작업이 팀 전체에서 월 30번 발생한다면 계산할 수 있습니다. 월 3번이면 도구 관리 비용이 더 클 수 있고요.

**Interviewer:** 측정한다면 시작과 끝을 어디로 잡겠습니까?

**서준:** 티켓이나 알람에서 DB 확인 필요성을 인지한 순간부터, 결과를 확인하거나 안전하게 공유하고 세션을 종료한 순간까지입니다. 앱 실행 시간만 측정하면 tunnel 실패, credential 찾기, CSV 정리 같은 비용이 빠집니다.

**Interviewer:** 기존 mysql CLI와 비교하는 pilot을 진행한다면 어떤 지표를 봅니까?

**서준:** 작업 완료 시간, 재접속 횟수, 잘못된 datasource 선택, 쿼리 오류, export 후 수작업 횟수, 도움 요청 건수를 봅니다. 그리고 설치한 사람 수가 아니라 2주차 재사용자와 실제 조회 세션 수를 봐야 합니다.

**Interviewer:** telemetry가 제품에 내장되어야 할까요?

**서준:** production DB client가 상세 SQL이나 결과를 외부로 보내면 바로 거절합니다. 익명 사용량도 opt-in과 수집 항목이 명확해야 해요. 초기 pilot은 사내 설문과 짧은 작업 일지로도 충분합니다. 제품 telemetry가 없다는 이유로 검증을 못 하는 것은 아닙니다.

**Interviewer:** 어떤 adoption metric이 가장 의미 있다고 봅니까?

**서준:** 설치 수보다 eligible user 대비 주간 활성 사용자, 첫 사용 뒤 14일 안에 재사용한 비율, mysql로 fallback한 비율입니다. 팀에 17명이 있어도 실제로 production DB를 조회하는 사람이 6명이면 분모는 6명이어야 하고요.

**Interviewer:** GitHub star나 다운로드 수는요?

**서준:** 외부 관심을 보는 신호일 뿐 우리 팀의 도입 근거는 아닙니다. 다운로드가 많아도 우리의 bastion, MySQL 버전, 터미널에서 안정적으로 동작하지 않으면 의미가 없습니다.

**Interviewer:** datavase 도입으로 줄어들 가능성이 있는 지원 비용은 무엇입니까?

**서준:** datasource와 tunnel 설정이 표준화되면 onboarding 설명과 DataGrip 설정 지원은 줄 수 있습니다. CSV나 row 복사 방식이 같아지면 결과 공유 실수도 줄 수 있고요. 다만 datavase 자체의 키 조작, config, 플랫폼별 clipboard 문제에 대한 질문이 새로 생길 수 있습니다.

**Interviewer:** 새로운 지원 부담을 어떻게 예상합니까?

**서준:** pilot 첫 2주 동안 질문을 전부 기록합니다. 같은 질문이 세 번 나오면 문서나 제품 문제로 봅니다. 한 사람당 30분을 아끼면서 Platform Engineer가 매주 두 시간을 지원한다면 실패한 도입입니다.

**Interviewer:** 팀 config를 repository에서 공유하면 표준화가 쉬워지지 않을까요?

**서준:** secret 없이 endpoint와 정책만 공유할 수 있다면 좋습니다. 하지만 개인 로컬 경로, SSH key, 환경별 예외가 섞이면 유지보수 대상이 하나 더 생깁니다. config schema 변경의 하위 호환성과 validation도 필요합니다.

**Interviewer:** datasource에 `read_only: true`를 공통 설정하면 accidental write 위험이 줄지 않습니까?

**서준:** 줄겠지만 실제 DB 계정이 write 가능한 상태라면 그것만 믿지 않습니다. UI에는 session 설정과 server 권한을 구분해서 보여줘야 합니다. pilot에서는 UPDATE가 거절되는 demo보다 reconnect 후에도 read-only가 유지되는지를 테스트하겠습니다.

**Interviewer:** read-only 기능의 성공은 어떻게 측정할 수 있나요?

**서준:** 사고 건수는 원래 희소해서 짧은 pilot로 통계 내기 어렵습니다. 대신 write statement 시도 후 차단 동작, 새 connection과 reconnect에서의 유지 여부를 시나리오 테스트할 수 있어요. “사고가 없었다”만으로 기능 효과를 주장하면 안 됩니다.

**Interviewer:** CSV export는 팀 도입을 촉진합니까?

**서준:** CS 대응이 많은 스쿼드에는 도움이 될 수 있습니다. 반대로 production 데이터 반출 정책과 충돌할 수 있어요. 저장 위치, 파일 권한, 민감정보 경고가 없으면 편의보다 검토 비용이 커집니다. 전 팀 공통 hook이라기보다는 특정 workflow의 가치입니다.

**Interviewer:** 팀 전원이 하나의 DB client를 쓰게 하면 효율적이지 않습니까?

**서준:** 통일 자체가 목표는 아닙니다. DataGrip이 필요한 사람까지 옮기면 오히려 생산성이 떨어져요. datavase는 간단한 production read에서 mysql CLI를 대체하는 선택지로 한정하는 편이 현실적입니다.

**Interviewer:** 그렇다면 표준이 아니라 권장 도구가 되는 기준은 무엇입니까?

**서준:** 지원 대상 환경이 명확하고, 소유자가 있으며, fallback 절차가 문서화되어야 합니다. pilot 참여자의 절반 이상이 4주 뒤에도 주간 대상 작업의 대부분에 사용하고 지원 문의가 감소한다면 권장 목록에 넣을 수 있습니다.

**Interviewer:** “소유자”는 datavase maintainer를 뜻합니까, 내부 담당자를 뜻합니까?

**서준:** 둘 다 필요합니다. 외부 프로젝트가 release와 보안 이슈를 관리해야 하고, 내부에서는 누가 버전을 승인하고 config 예제를 유지할지 정해야 합니다. 작은 팀은 이 내부 소유자를 두는 것 자체가 opportunity cost입니다.

**Interviewer:** vendor support나 SLA가 없다면 거절합니까?

**서준:** 내부 개발 도구라면 꼭 필요하지는 않습니다. 대신 문제가 생기면 mysql CLI로 즉시 돌아갈 수 있어야 하고, 파일 형식이나 설정이 우리를 잠그면 안 됩니다. maintainer 반응성과 release 기록은 확인하겠습니다.

**Interviewer:** 어떤 장애가 pilot을 즉시 중단시킵니까?

**서준:** 잘못된 datasource 표시, read-only 상태 오표시, query result 값 손상, credential 노출은 즉시 중단입니다. clipboard 실패 정도는 버그로 기록할 수 있지만, 데이터 정확성과 안전 상태를 의심하게 만드는 문제는 한 번으로 충분합니다.

**Interviewer:** 반대로 어떤 행동이 도입 확신을 높입니까?

**서준:** 제가 권하지 않아도 pilot 참여자가 다음 온콜에서 다시 실행하고, 동료에게 사용법을 설명하고, mysql로 돌아간 이유가 기능 부족이 아니라 미지원 환경처럼 명확할 때입니다. 자발적 재사용과 낮은 지원 의존성이 중요합니다.

**Interviewer:** 개발팀이 추가 기능을 요청하면 roadmap에 반영해야 하지 않을까요?

**서준:** 요청 개수만 세면 금방 IDE가 됩니다. 실제 대상 작업에서 fallback을 만든 원인인지 먼저 봐야 해요. query tabs나 schema editing 요청이 나와도 DataGrip 작업을 가져오려는 것이라면 제품 범위를 넓히지 않는 편이 낫습니다.

**Interviewer:** 팀 구매 의향은 있습니까?

**서준:** 지금 기능에 seat당 구독료를 내기는 어렵습니다. 내부 지원을 줄이는 관리 기능, 검증된 배포 채널, 보안 대응이 제공되고 시간 절감이 측정된다면 회사 비용으로 검토할 수 있습니다. one-time donation이나 sponsor는 실제로 팀이 유지 사용한 뒤에 가능합니다.

**Interviewer:** 제품 maintainer에게 어떤 자료를 요청하겠습니까?

**서준:** 지원 MySQL·MariaDB 버전과 OS, credential 저장 방식, telemetry 항목, release·보안 정책, config migration 원칙, 알려진 제한을 보고 싶습니다. 화려한 demo보다 실패했을 때 무엇이 보장되는지가 중요합니다.

**Interviewer:** 4주 pilot을 설계한다면 어떻게 진행합니까?

**서준:** production 조회가 잦은 Backend Engineer 3명과 Platform Engineer 1명으로 시작하겠습니다. 첫 주에는 기존 방식의 완료 시간과 실패를 기록하고, 다음 3주에는 datavase를 기본 선택으로 쓰되 mysql fallback 이유를 남깁니다. 중간에 critical failure가 있으면 중단합니다.

**Interviewer:** pilot의 통과 기준을 미리 정한다면요?

**서준:** eligible workflow의 60% 이상에서 사용, 참여자 4명 중 3명 이상이 2주차 이후 재사용, median 완료 시간 20% 이상 감소, critical data·security failure 0건, 반복 지원 문의가 주 1건 이하 정도로 시작하겠습니다. 표본이 작으니 절대적인 증명이라기보다 다음 결정 기준입니다.

**Interviewer:** 기준을 통과하면 17명 모두에게 의무화합니까?

**서준:** 아닙니다. 해당 workflow에 권장하고 설치 문서를 제공하겠습니다. 그다음 eligible user의 adoption과 지원 비용을 다시 봅니다. 의무화는 대체할 도구와 업무 범위가 훨씬 명확할 때만 가능합니다.

**Interviewer:** 지금 바로 pilot을 승인하겠습니까?

**서준:** 한 명의 내부 champion이 있고, config에 secret이 없으며, 지원 환경과 fallback이 문서화돼 있다면 작은 pilot은 승인할 수 있습니다. 다만 README의 주장만으로 팀 rollout이나 구매를 결정하지는 않겠습니다.

### 결정적 발언

> “개인의 만족과 팀 생산성은 다릅니다. 무료 도구도 지원하고 유지하는 순간 조직 비용이 생깁니다.”

> “설치 수보다 두 번째 사용, mysql로 돌아간 이유, Platform 팀의 지원 시간을 보겠습니다.”

> “datavase가 DataGrip까지 대체할 필요는 없습니다. 반복되는 production read에서 전체 작업 시간을 줄이는지만 증명하면 됩니다.”

### Interview 16 결과

이 페르소나는 datavase의 직접 사용 빈도가 낮지만 pilot과 팀 rollout을 승인할 권한이 있다. 개인 개발자가 grid, read-only, CSV에 긍정적으로 반응하는 것만으로는 도입하지 않는다. 기존 workflow의 빈도와 전체 완료 시간, 재사용, fallback, 지원 요청을 함께 측정해 조직 전체의 순효과가 양수인지 판단한다.

특히 무료·단일 binary라는 속성은 구매 장벽을 낮출 뿐 유지 비용을 없애지 않는다. 배포와 업데이트, 공용 config, credential 정책, 내부 문서, 장애 지원에 owner가 필요하다. 따라서 팀 확산을 위해서는 더 많은 기능보다 지원 범위와 실패 모드, release·security 정책을 명확히 하는 것이 중요하다.

| 의사결정 영역 | 요구하는 근거 | 거절 또는 중단 조건 |
|---|---|---|
| 개인 가치 | 동일 작업의 반복 사용과 전체 완료 시간 감소 | 데모 만족도와 단발성 사용만 존재 |
| 팀 생산성 | eligible workflow 기준 시간 절감·재사용 | 낮은 작업 빈도로 관리 비용이 절감분 초과 |
| 안전성 | reconnect 포함 read-only 유지, 값 정확성 | 상태 오표시, 결과 손상, credential 노출 |
| adoption | 14일 재사용률, 대상 작업 점유율 | 설치 수만 증가하고 mysql fallback 지속 |
| 지원 비용 | 문의 유형·시간의 감소, self-service 문서 | Platform 지원 시간이 매주 반복 발생 |
| rollout | 내부 champion, owner, 명확한 fallback | 책임자와 업데이트 정책 부재 |
| 구매 | 검증된 시간 절감, 보안 대응·배포 가치 | 현재 편의 기능만으로 seat 구독 요구 |
| 제품 범위 | production read에서 mysql 대체 | DataGrip 업무까지 가져오는 기능 확장 |

이 인터뷰는 Engineering Manager를 초기 최종 사용자 ICP로 지지하지 않는다. 대신 팀 확산의 gatekeeper이자 경제성 검증자라는 별도 역할을 보여준다. 개인 사용자가 먼저 자발적으로 retention을 보인 뒤에야 이 페르소나의 관심이 시작된다. 따라서 manager 대상 메시지는 기능 목록보다 pilot을 작게 시작하는 방법, 측정 기준, 보안·지원 범위와 fallback을 제공해야 한다.

팀 단위 확산은 전원 표준화보다 좁은 workflow의 권장 도구로 시작하는 것이 적합하다. `production read에서 mysql 대신 사용`이라는 경계를 유지하면 DataGrip 사용자에게 불필요한 migration을 요구하지 않으면서 도입 효과를 측정할 수 있다. 합성 인터뷰에서 제시한 60%, 20% 같은 수치는 제품 목표가 아니라 실제 pilot 설계를 위한 초기 가설에 불과하다.

### 검증 후보

다음 항목은 이 페르소나의 가정을 실제 manager 인터뷰, 사용자 행동과 pilot으로 검증하기 위한 후보이다.

1. 팀에서 production MySQL 조회가 가능한 인원과 실제 월간 eligible workflow 수
2. Engineering Manager가 개인 도구를 권장 또는 표준 도구로 승격한 최근 실제 사례와 판단 기준
3. 접속 시작부터 결과 확인·공유·종료까지 기존 workflow의 전체 완료 시간
4. pilot 설치자 중 7일·14일·28일 재사용률과 대상 작업에서의 datavase 사용 비율
5. datavase 사용 후 mysql CLI로 fallback한 빈도와 구체적인 원인
6. datasource 오선택, reconnect, query 오류, export 수작업 등 workflow 실패 빈도 변화
7. 설치·config·단축키·clipboard·upgrade 관련 내부 문의의 건수와 지원 시간
8. 공용 config가 onboarding 지원을 줄이는지, 새로운 유지보수 부담을 만드는지
9. read-only가 최초 연결뿐 아니라 reconnect와 connection pool 전체에서 유지되는지
10. session read-only와 database-level 권한을 사용자가 정확히 구분하는지
11. 결과 값 정확성, datasource 표시, credential 처리에 대한 critical pilot 중단 기준
12. 팀 내 champion 유무가 자발적 확산과 self-service 지원에 미치는 영향
13. production read만 권장할 때와 범용 DB client로 소개할 때 adoption·지원 비용의 차이
14. 무료 사용 이후 sponsor, one-time payment, team subscription을 검토하게 되는 실제 조건
15. release cadence, security policy, 지원 matrix가 manager의 pilot 승인에 미치는 영향
16. 절감된 사용자 시간과 Platform·manager의 유지 시간을 합산한 순 조직 비용

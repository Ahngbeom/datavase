## Interview 18 — 최은서: 팀의 개발 도구 도입을 표준화하는 Developer Experience Engineer

> 합성 페르소나 시뮬레이션이며 실제 사용자 인터뷰나 수요의 증거가 아니다.

### 페르소나

- 36세, Developer Experience Engineer, 경력 11년
- 개발자 120명 규모의 B2B SaaS 회사에서 내부 개발자 포털, 로컬 개발환경, CLI 배포와 온보딩을 담당
- 애플리케이션 팀은 Java·Go를 사용하고 운영 데이터베이스는 MySQL 8과 일부 MariaDB로 구성
- 본인이 production SQL을 실행하는 빈도는 낮지만, 여러 팀이 사용할 도구의 설치·업데이트·지원 정책을 검토
- macOS와 Linux가 주 환경이고 Homebrew tap, 사내 artifact registry, checksum이 포함된 고정 버전 설치를 선호
- 개인별 datasource 작성보다 검토된 공통 config template과 secret reference를 배포하는 방식을 선호
- 도구의 기능 수보다 첫 성공까지 걸리는 시간, 문서 이탈 지점, 버전 분산, 반복 문의량을 기준으로 도입을 판단
- 새 CLI를 전사 표준으로 지정하기 전에 작은 pilot, rollback 절차, owner와 지원 범위를 요구

### 인터뷰 목적

개별 개발자가 아니라 개발 도구를 조직에 배포하고 지원하는 담당자의 관점에서 datavase의 adoption 조건을 확인한다. 단일 binary라는 장점이 실제 팀 배포·온보딩까지 이어지는지, config template과 문서·버전 고정·업데이트 정책이 갖춰지지 않았을 때 어떤 운영 비용이 생기는지 검증한다.

### 인터뷰

**Interviewer:** 최근 새로운 CLI를 팀에 도입했던 과정을 처음부터 설명해주세요.

**은서:** 로컬 Kubernetes 접근을 단순화하는 사내 wrapper였습니다. 먼저 세 팀에서 pilot을 했고, macOS와 Linux 패키지를 사내 registry에 올렸어요. 버전을 고정한 설치 스크립트, 기본 config, 10분짜리 quickstart, 제거와 rollback 방법을 같이 배포했습니다. 한 달 동안 설치 성공률과 지원 채널 문의를 보고 문제가 줄어든 뒤 권장 도구로 올렸습니다.

**Interviewer:** 기능이 유용하다는 판단만으로 바로 배포하지 않은 이유는 무엇인가요?

**은서:** 한 사람에게 좋은 도구와 조직이 운영할 수 있는 도구는 다릅니다. 설치와 설정에서 사람마다 다른 선택을 하면 지원 비용이 커져요. 업데이트가 자동으로 섞이면 재현도 어렵습니다. 기능 데모보다 누가 어떤 버전을 어떤 설정으로 쓰는지가 먼저 정리돼야 합니다.

**Interviewer:** 데이터베이스 클라이언트는 현재 어떻게 제공합니까?

**은서:** 공식 표준은 DataGrip과 `mysql`입니다. DataGrip 설정 가이드와 팀별 read-only 계정을 제공하고, bastion 접속은 사내 CLI가 담당합니다. 서버나 임시 컨테이너에서는 `mysql`을 사용합니다. 일부 개발자가 mycli 같은 도구를 쓰지만 중앙 지원 대상은 아닙니다.

**Interviewer:** 개발자들이 mysql CLI에서 자주 막히나요?

**은서:** 출력이 깨진다는 불만은 있지만 그것보다 접속 정보와 인증 문의가 많습니다. 신규 입사자는 어느 endpoint와 database를 써야 하는지, VPN과 bastion 중 무엇이 필요한지 헷갈립니다. 운영 사고가 걱정돼 read-only 계정을 요청하는 문의도 반복됩니다.

**Interviewer:** datavase는 single binary이고 datasource를 저장해 grid에서 SQL 결과를 보여줍니다. 첫인상은 어떻습니까?

**은서:** 개인 설치는 쉬워 보입니다. 하지만 binary 하나라는 말만으로 팀 온보딩이 쉬운 것은 아닙니다. 어디서 받아야 하는지, checksum이나 서명이 있는지, 어떤 버전을 표준으로 삼을지, config를 누가 만들어주는지가 남습니다.

**Interviewer:** Homebrew 한 줄로 설치할 수 있다면 충분하지 않나요?

**은서:** 시험 사용에는 충분합니다. 팀 표준에는 부족해요. public tap을 production 접근 도구의 유일한 공급 경로로 둘 수 있는지 검토해야 하고, 이전 버전을 다시 설치할 수 있어야 합니다. Linux와 CI·원격 서버용 배포 경로도 같아야 합니다.

**Interviewer:** 최신 버전을 자동으로 설치하게 하면 관리가 더 단순하지 않습니까?

**은서:** 오히려 장애 분석이 어려워집니다. 어제는 됐는데 오늘 안 된다는 문의에서 버전이 다르면 원인을 재현할 수 없어요. 조직 기본 버전은 고정하고, 새 버전은 pilot group에서 검증한 뒤 올리는 편이 낫습니다.

**Interviewer:** 보안 수정이 포함된 긴급 업데이트도 같은 절차를 따릅니까?

**은서:** 속도는 높이지만 버전은 여전히 명시합니다. 지원 버전과 교체 기한, breaking change, rollback 가능 여부를 공지해야 합니다. 자동 최신화만으로 해결하면 사용자가 언제 무엇이 바뀌었는지 모릅니다.

**Interviewer:** 개발자가 각자 datasource를 등록하면 어떤 문제가 있습니까?

**은서:** host 이름 오타 정도는 개인 문제지만 production과 staging을 잘못 이름 붙이거나 SSL 옵션을 빠뜨리면 조직 문제입니다. 열 명이 같은 접속 정보를 각자 옮겨 적게 만들고 싶지 않아요. 검토된 template을 배포하고 개인은 username이나 secret reference만 채우는 방식이 필요합니다.

**Interviewer:** config 파일을 그대로 공유하면 되지 않나요?

**은서:** 구조가 명확하고 secret이 빠진다는 보장이 있으면 가능합니다. 그런데 사용자 홈 경로, keychain 항목명, SSH key 위치가 박혀 있으면 공유하기 어렵습니다. 환경별 override와 필수 값 검증이 있어야 하고, template을 업데이트했을 때 개인 설정을 덮어쓰지 않아야 합니다.

**Interviewer:** 좋은 config template에는 무엇이 들어갑니까?

**은서:** 사람이 읽는 datasource 이름, environment 표시, host·port·database, TLS와 SSH 정책, 기본 read-only 여부, secret을 가져오는 방식입니다. 반대로 password나 개인 SSH key 경로는 없어야 합니다. 누가 언제 검토한 template인지와 schema version도 알 수 있으면 좋습니다.

**Interviewer:** datavase의 `read_only: true`를 template에 넣으면 안전 문제를 해결할 수 있습니까?

**은서:** 실수 방지 기본값으로는 좋습니다. 하지만 DB 계정 권한과 같은 보장으로 문서화하면 안 됩니다. template에는 client session guardrail이라고 적고, server-side read-only 계정도 계속 사용해야 합니다. 실제 세션이 read-only인지 UI와 진단 명령에서 확인돼야 하고요.

**Interviewer:** 신규 입사자의 첫 사용을 어떻게 설계하겠습니까?

**은서:** 설치, 회사 template 받기, secret 연결, staging datasource 열기, 샘플 SELECT 실행까지 한 흐름이어야 합니다. 중간에 README 여러 페이지를 오가거나 YAML 전체를 처음부터 작성하게 하면 이탈합니다. production 연결은 별도 권한 요청 뒤에 열려야 하고요.

**Interviewer:** 어느 정도 시간이면 좋은 onboarding입니까?

**은서:** 접근 권한이 이미 있다는 전제에서 staging 첫 SELECT까지 10분 이하면 좋습니다. 중요한 건 평균보다 실패 분포예요. 절반이 5분 만에 되고 나머지가 한 시간씩 지원을 받으면 성공한 onboarding이 아닙니다.

**Interviewer:** 문서는 어떤 구조여야 합니까?

**은서:** 첫 화면에는 지원 OS와 DB, 설치, 최소 config, 첫 query가 있어야 합니다. 그 다음에 SSH, TLS, keychain, read-only 같은 운영 설정을 분리하고, 마지막에 troubleshooting과 config reference가 있어야 해요. 예제와 실제 옵션명이 릴리스마다 맞는지도 테스트해야 합니다.

**Interviewer:** README에 23초 demo가 있으면 온보딩에 도움이 되나요?

**은서:** 무엇을 얻는지 보여주는 데는 좋습니다. 하지만 demo는 설치나 설정 실패를 해결하지 않습니다. demo에서 쉬워 보였는데 quickstart가 재현되지 않으면 오히려 신뢰가 떨어집니다. 영상의 command도 현재 릴리스에서 그대로 실행돼야 합니다.

**Interviewer:** 모든 옵션을 하나의 긴 문서에 정리하면 검색하기 쉽지 않습니까?

**은서:** reference에는 좋지만 첫 사용 문서로는 나쁩니다. quickstart와 reference의 역할을 분리해야 합니다. 사용자가 처음부터 모든 옵션을 읽지 않아도 안전한 기본값으로 성공해야 해요.

**Interviewer:** config generator를 추가하는 것이 필요할까요?

**은서:** 바로 필요하다고 보진 않습니다. 잘 설명된 작은 schema와 검증 가능한 template이면 충분할 수 있어요. interactive wizard를 만들면 자동화와 리뷰가 어려워질 수도 있습니다. 실제로 YAML 작성에서 이탈하는지 먼저 측정해야 합니다.

**Interviewer:** 팀 config를 repository로 관리하는 방식은 어떻습니까?

**은서:** secret이 절대 들어가지 않고 datasource 정의가 안정적이면 좋습니다. pull request로 endpoint나 정책 변경을 리뷰할 수 있으니까요. 다만 datavase가 config schema를 바꿀 때 migration 방법과 구버전 호환 기간이 필요합니다.

**Interviewer:** config schema version이 꼭 있어야 합니까?

**은서:** 조직 배포에서는 중요합니다. binary만 올렸는데 기존 config가 조용히 다르게 해석되는 것이 가장 위험합니다. 지원하지 않는 schema면 명확히 중단하고, 자동 변환을 한다면 diff와 백업을 보여줘야 합니다.

**Interviewer:** 버전 고정은 어떤 형태를 기대합니까?

**은서:** Homebrew라면 특정 formula version이나 사내 mirror, Linux라면 버전이 포함된 URL과 checksum입니다. `dv version`으로 build 정보를 확인할 수 있어야 하고, bug report에 config schema와 OS 정보를 secret 없이 첨부할 수 있으면 좋습니다.

**Interviewer:** telemetry를 내장하면 지원에 도움이 되지 않을까요?

**은서:** production DB client의 query나 datasource 정보가 전송될 수 있다는 의심만 있어도 도입이 막힙니다. 기본 비활성화가 안전합니다. 제품 사용량이 필요하면 명시적 opt-in과 전송 항목 공개가 있어야 하고, query text·result·host·username은 수집하지 않아야 합니다.

**Interviewer:** 그렇다면 adoption을 어떻게 측정합니까?

**은서:** 배포 시스템에서는 설치 수와 버전 분포를 볼 수 있습니다. 온보딩에서는 익명 설문이나 pilot session으로 첫 성공 시간을 측정하고요. 지원 채널의 문의 유형, 재설치, rollback, 기존 mysql로 돌아간 이유를 함께 봅니다. 실행 횟수 하나만 보면 실제 가치인지 알 수 없습니다.

**Interviewer:** 구체적으로 어떤 지표를 pilot에서 보겠습니까?

**은서:** 설치 성공률, staging 첫 SELECT 성공률, median과 p90 time-to-first-query, datasource 설정 단계별 이탈, 사용자당 지원 문의 수, 2주 후 재사용률입니다. 안정성은 연결 실패, crash, config migration 실패, 긴급 rollback 횟수를 봅니다.

**Interviewer:** 목표 수치는 어느 정도입니까?

**은서:** 처음부터 절대 기준을 정하기보다 현재 workflow를 baseline으로 측정하겠습니다. 그래도 pilot 20명 중 대부분이 지원 없이 15분 안에 첫 query를 실행하고, 심각한 config·credential 사고가 없어야 확대할 수 있습니다. 재사용은 실제 DB 조회 빈도와 같이 봐야 합니다.

**Interviewer:** 지원 문의가 많아도 사용량이 높으면 성공 아닌가요?

**은서:** 작은 팀에서는 그럴 수 있지만 전사 배포에서는 지속 가능하지 않습니다. 한 번의 설치 문의와 반복되는 접속 장애 문의도 구분해야 합니다. 사용량 증가와 함께 동일 문의가 선형으로 늘면 self-service 도구가 아닙니다.

**Interviewer:** 지원 책임은 누구에게 있어야 합니까?

**은서:** 제품 owner, 보안·DB 접근 owner, 내부 배포 owner의 경계를 써야 합니다. datavase bug인지 계정 권한인지 bastion 문제인지 사용자가 판단할 수 있어야 해요. 공개 프로젝트만 링크하고 내부 담당자를 정하지 않으면 결국 저희 팀이 모든 문의를 받습니다.

**Interviewer:** 오픈소스 프로젝트라면 GitHub issue로 보내면 되지 않나요?

**은서:** 회사 endpoint와 오류 로그를 공개 issue에 붙일 수 없습니다. 내부 triage 후 재현 가능한 최소 정보만 upstream에 올려야 합니다. security issue 접수 경로와 응답 기대치도 확인합니다.

**Interviewer:** release note에서 가장 먼저 보는 항목은 무엇입니까?

**은서:** config 호환성, credential과 network 동작 변화, 지원 OS·DB 버전, known issue입니다. 새로운 copy shortcut보다 기존 접속이 깨지는지 여부가 먼저예요. breaking change가 제목 아래 숨겨져 있으면 업데이트를 보류합니다.

**Interviewer:** datavase가 빠르게 발전하는 초기 프로젝트라는 점은 어떻게 평가합니까?

**은서:** pilot에는 괜찮지만 표준화에는 위험 요소입니다. 릴리스 cadence보다 호환성 약속과 지원 기간이 중요해요. 매 버전 config가 달라지거나 과거 binary를 받을 수 없으면 중앙 배포 대상으로 삼기 어렵습니다.

**Interviewer:** 기능을 더 추가하면 도입 설득이 쉬워지지 않습니까?

**은서:** 꼭 그렇지 않습니다. 기능이 늘면 문서와 지원 조합도 늘어납니다. 지금 약속한 datasource, table 확인, SQL 실행, 결과 반출이 다양한 환경에서 예측 가능하게 동작하는 편이 조직 도입에는 낫습니다.

**Interviewer:** CSV export를 팀 표준 기능으로 강조하겠습니까?

**은서:** production data 반출 정책부터 확인해야 합니다. 파일 경로, 덮어쓰기, permission, 민감정보 잔존을 문서화해야 해요. 편리하다는 이유로 온보딩 첫 단계에 놓으면 사용자가 허용된 행동으로 오해할 수 있습니다.

**Interviewer:** 어떤 문서가 없으면 pilot도 시작하지 않겠습니까?

**은서:** secret 저장 방식, read-only의 한계, 지원 플랫폼, 제거 방법, 최소한의 troubleshooting입니다. production endpoint를 다루는데 password가 어디에 저장되는지 불명확하면 시험 대상에도 넣지 않습니다.

**Interviewer:** 어떤 사건이 발생하면 rollout을 중단합니까?

**은서:** config update가 개인 secret을 덮어쓰는 것, 버전마다 연결 동작이 달라지는 것, 로그나 진단 bundle에 credential이 포함되는 것입니다. 그리고 새 버전 문제가 생겼는데 이전 버전을 즉시 복구할 수 없어도 중단합니다.

**Interviewer:** 반대로 표준 도구 후보가 되는 순간은 언제입니까?

**은서:** 팀 template 하나로 여러 사용자가 지원 없이 staging에 접속하고, 같은 버전에서 결과를 재현하며, 문제가 생기면 진단 정보만으로 원인 범위를 좁힐 수 있을 때입니다. 개인이 “UI가 좋다”고 말하는 것보다 그 장면이 중요합니다.

**Interviewer:** 지금 datavase를 전사 배포하시겠습니까?

**은서:** 아닙니다. 다섯 명 정도의 자발적 사용자를 대상으로 pilot은 해볼 수 있습니다. 설치보다 config 배포와 버전 운영 자료가 충분한지 먼저 확인하겠습니다. 그 결과가 좋으면 팀 template과 고정 버전을 만들어 범위를 넓힐 겁니다.

**Interviewer:** 제품 소개에서 가장 설득력 있는 문장은 무엇일까요?

**은서:** “single binary”는 시작점이지 결과가 아닙니다. 저라면 “검토된 read-only datasource를 같은 방식으로 배포하고 재현할 수 있다”는 증거에 반응합니다. 하지만 현재 제품이 그것을 보장하지 않는다면 마케팅 문장으로 먼저 약속해서는 안 됩니다.

### 결정적 발언

> “Binary 하나라는 말은 다운로드가 쉽다는 뜻이지, 팀 온보딩이 끝났다는 뜻은 아닙니다.”

> “조직에서 좋은 CLI는 모두가 최신 버전을 쓰는 도구가 아니라, 문제가 난 사용자가 어떤 버전과 설정을 썼는지 재현할 수 있는 도구입니다.”

> “사용량보다 먼저 볼 것은 첫 query까지의 실패 분포와 반복 지원 문의입니다.”

### Interview 18 결과

이 페르소나에게 datavase의 single binary는 분명한 배포 장점이지만 독립적인 조직 도입 동기는 아니다. 개인의 설치 명령 뒤에는 artifact 신뢰, macOS·Linux 배포, 버전 고정, rollback, config 공유, secret 분리, 문서와 내부 지원 책임이라는 운영 작업이 남는다. 따라서 다운로드 횟수나 demo 반응만으로 팀 adoption을 판단하면 실제 지원 비용을 놓칠 수 있다.

특히 공통 datasource template은 잠재력이 크다. environment, TLS·SSH 정책, session read-only 기본값을 검토된 파일로 배포하면 신규 입사자의 반복 설정과 환경 오인을 줄일 수 있다. 그러나 secret과 개인 경로가 분리되지 않거나 schema 변경이 조용히 적용되면 편의 기능이 새로운 사고 원인이 된다. read-only 또한 server-side 권한이 아닌 client guardrail이라는 한계를 template과 UI에서 일관되게 설명해야 한다.

| 영역 | 조직이 기대하는 상태 | datavase의 잠재 가치 | rollout을 막는 조건 |
|---|---|---|---|
| 배포 | 검증된 macOS·Linux artifact | single binary, 단순 설치 | checksum·과거 버전·제거 경로 부재 |
| 버전 | 조직 기본 버전 고정 | 작은 release 단위 | 자동 최신화, 재현 불가능한 버전 분산 |
| 설정 | 검토된 공통 template + 개인 override | datasource와 환경 정책 표준화 | secret·개인 경로 혼입, 조용한 schema 변경 |
| 온보딩 | 지원 없이 staging 첫 SELECT | 작은 workflow의 빠른 학습 | 처음부터 전체 config 작성, 문서 왕복 |
| 문서 | quickstart·운영 가이드·reference 분리 | 좁은 범위의 명확한 안내 | demo와 현재 release 불일치 |
| 안전 | DB 권한 + client guardrail 구분 | 기본 session read-only | 실제 권한처럼 과장된 표시 |
| 진단 | secret 없는 재현 정보 | 버전·OS·schema 상태 표출 | 로그와 bundle의 credential 노출 |
| 측정 | 첫 성공·재사용·지원 비용 결합 | 작은 pilot로 검증 가능 | 실행 횟수만으로 adoption 판단 |
| 지원 | 내부와 upstream owner 경계 | GitHub 기반 이슈 추적 | 공개할 수 없는 환경정보 의존 |
| rollback | 검증된 이전 버전 즉시 복구 | 단일 artifact 교체 | 과거 binary 부재, config 역호환 실패 |

이 인터뷰는 새로운 wizard나 중앙 관리 기능을 즉시 추가하라는 근거가 아니다. 우선 문서화된 config schema, secret-free template 예제, 버전 확인, 명시적인 호환성·rollback 절차처럼 현재의 좁은 제품 범위를 강화하는 수단을 실제 pilot에서 검증해야 한다. 사용자가 YAML 작성 자체에서 반복 이탈한다는 증거가 확인될 때 generator나 관리 기능을 검토하는 편이 적절하다.

초기 개인 사용자 ICP와는 다른 도입 결정자이며, 직접 사용 빈도만 보면 낮은 적합도다. 하지만 작은 팀에서 검증된 사용을 조직 단위 retention으로 전환하려면 중요한 영향자다. 이 페르소나가 요구하는 핵심은 더 많은 DB 기능이 아니라 예측 가능한 배포 계약과 지원 가능한 onboarding이다.

### 검증 후보

다음 항목은 이 페르소나의 가정을 실제 Developer Experience 담당자 인터뷰와 pilot으로 검증하기 위한 후보이다.

1. 신규 사용자가 설치부터 staging 첫 SELECT까지 걸리는 median·p90 시간과 단계별 이탈률
2. README만 사용한 그룹과 짧은 quickstart를 사용한 그룹의 첫 query 성공률 차이
3. 개인별 datasource 작성과 secret-free 공통 template 배포의 설정 오류·지원 문의 차이
4. config template에 필요한 공통 필드와 반드시 개인 override로 남겨야 할 필드
5. config schema 변경 시 조직이 기대하는 호환 기간, migration, 실패 처리 방식
6. Homebrew, 직접 다운로드, 사내 registry 중 팀 규모별 선호 배포 경로
7. checksum·artifact signing·SBOM·과거 버전 보관이 pilot 및 전사 rollout 결정에 미치는 영향
8. 최신 자동 업데이트와 조직 고정 버전 중 실제 선호 및 긴급 보안 업데이트 절차
9. `dv version`과 진단 정보에 필요한 build·OS·config schema·DB driver 정보
10. secret을 포함하지 않는 support bundle이 실제 접속 실패 triage 시간을 줄이는지
11. 설치 수, 첫 성공률, 2주 재사용률, 기존 mysql 복귀율 중 adoption을 가장 잘 설명하는 지표
12. 사용자당 지원 문의 수와 반복 문의 유형이 팀 확대에 따라 어떻게 증가하는지
13. query text·result·host를 수집하지 않는 opt-in telemetry에도 조직이 동의하는지
14. read-only template이 실제로 environment 오인이나 accidental write 불안을 줄이는지
15. CSV export와 로컬 파일 잔존에 대해 필요한 기본 경고·permission·정책 문서
16. 새 버전 pilot에서 연결·clipboard·CSV·config migration을 검증할 최소 회귀 테스트
17. 문제가 있는 릴리스에서 이전 binary와 config로 복구하는 데 걸리는 시간
18. 내부 지원 owner와 upstream GitHub issue 사이의 triage 경계 및 security report 경로
19. 기능 추가보다 문서·배포 계약 개선이 실제 도입률과 retention에 미치는 영향
20. 5명 pilot에서 팀 권장 도구로 확대할 때 사용하는 명시적 go/no-go 기준

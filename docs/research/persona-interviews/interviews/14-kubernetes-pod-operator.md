# Interview 14 — 박지훈: Kubernetes Pod 내부에서 운영 DB를 확인하는 플랫폼 엔지니어

> 합성 페르소나 시뮬레이션이며 실제 사용자 인터뷰나 수요의 증거가 아니다.

### 페르소나

- 35세, B2B SaaS 기업 Platform·SRE Engineer, 경력 10년
- Kubernetes에서 Java 기반 멀티테넌트 서비스를 운영하며 MySQL 8을 사용
- production DB는 사내망이나 개인 노트북에서 직접 접근할 수 없고, 특정 namespace의 debug Pod 또는 application Pod 네트워크에서만 접근 가능
- 장애 대응 시 `kubectl logs`, `kubectl exec`, ephemeral container, 임시 toolbox Pod를 상황에 따라 사용
- application image에는 보안과 이미지 크기 때문에 mysql client, shell, 패키지 관리자가 없을 때가 많음
- production workload 변경 권한은 제한되어 있고, 임시 Pod 생성과 `exec`도 RBAC·감사 정책의 적용을 받음
- 로컬에서는 DataGrip을 사용하지만 운영에서는 네트워크 경계 때문에 terminal workflow가 사실상 강제됨
- 새로운 TUI의 편의성보다 **필요한 순간 실제 Pod에서 실행 가능한지**, credential이 어떻게 주입되고 폐기되는지, 대상 cluster·namespace·database가 분명한지를 우선함

### 인터뷰 목적

DB 네트워크에 도달하기 위해 Kubernetes Pod 내부에서 client를 실행해야 하는 사용자에게 single binary와 terminal UI가 실제 가치를 주는지 검증한다. Pod와 container의 짧은 수명, `kubectl exec` 권한, runtime architecture, config·credential 주입과 폐기, cluster·namespace·database context가 설치와 반복 사용의 결정 조건인지 확인한다.

### 인터뷰

**Interviewer:** 최근 production DB를 직접 확인했던 상황을 처음부터 설명해주세요.

**지훈:** 새 버전 배포 후 특정 tenant의 API만 500이 늘었습니다. 로그에서 tenant id와 request id를 찾고, production cluster context를 확인한 다음 운영 namespace에 임시 toolbox Pod를 띄웠어요. 그 Pod에 들어가 read-only 계정으로 MySQL에 접속해서 tenant 설정 row와 migration 상태를 조회했습니다. 확인이 끝난 뒤 Pod를 지웠습니다.

**Interviewer:** 왜 노트북의 DataGrip으로 접속하지 않았습니까?

**지훈:** DB endpoint가 cluster 내부에서만 열립니다. bastion을 거쳐 tunnel을 만드는 방법도 있지만 평상시에는 막아두고, 사고 대응 때 승인받은 namespace 안에서만 접근하게 해놨어요. DataGrip의 편의성 문제가 아니라 네트워크 경계 문제입니다.

**Interviewer:** application Pod에 바로 `kubectl exec`하지 않은 이유는 무엇인가요?

**지훈:** 저희 application container는 distroless에 가까워서 shell도 mysql client도 없습니다. 실행 중인 workload에 파일을 복사하는 것도 정책상 피합니다. 이미 문제가 있는 Pod에 진단 도구를 추가하면 원래 상태를 바꿀 수 있고, 재시작하면 사라져서 재현도 어렵습니다.

**Interviewer:** toolbox Pod는 항상 준비되어 있습니까?

**지훈:** 아닙니다. 승인된 image와 manifest 예시는 저장소에 있지만 필요할 때 생성합니다. 상시 운영하면 공격 표면과 credential 보관 문제가 생기니까요. 긴급할 때 image pull이 느리거나 registry 접근이 안 되면 그 몇 분이 답답합니다.

**Interviewer:** 그 workflow에서 가장 불편했던 부분은 무엇이었나요?

**지훈:** SQL 결과보다 준비 과정입니다. 올바른 cluster인지, namespace인지 확인하고 Pod를 띄우고, service account와 network policy가 맞는지 보고, secret을 노출하지 않는 방식으로 DB credential을 연결해야 합니다. 쿼리는 그 뒤에 두세 번 실행하는 정도예요.

**Interviewer:** mysql CLI 결과가 불편해서 시간을 잃은 적은 없습니까?

**지훈:** 컬럼이 많으면 보기 불편하고 복사도 어색합니다. 그래도 `\G`나 쿼리에서 컬럼을 줄여 해결합니다. 지난 장애에서 20분이 걸렸다면 mysql 출력 때문에 쓴 시간은 2분도 안 됐을 겁니다.

**Interviewer:** datavase는 하나의 static binary로 실행할 수 있고 결과를 grid로 보여줍니다. 이 환경에 맞을까요?

**지훈:** single binary는 장점이지만 “하나”라는 말만으로 충분하지 않습니다. container architecture가 amd64인지 arm64인지, libc가 필요한지, CA certificate와 DNS가 어떻게 동작하는지 봐야 해요. 그리고 binary를 Pod에 넣는 절차가 승인돼 있어야 합니다.

**Interviewer:** `kubectl cp`로 실행 중인 Pod에 binary를 복사할 수 있다면요?

**지훈:** 개인 테스트에는 가능해도 production 표준 workflow로는 쓰지 않습니다. 누가 어떤 바이너리를 복사했는지 image digest로 추적하기 어렵고, container에 `tar`가 없어 `kubectl cp` 자체가 실패할 수도 있어요. Pod가 재시작되면 없어지는 것도 문제고요.

**Interviewer:** 그러면 datavase가 포함된 toolbox image를 만들면 어떻습니까?

**지훈:** 그게 현실적입니다. 버전을 고정하고 vulnerability scan을 통과한 사내 image로 만들어야 합니다. manifest나 내부 CLI로 그 image를 실행하고, 종료하면 Pod와 임시 파일이 같이 사라지는 구조가 좋습니다.

**Interviewer:** 그 이미지가 준비되면 mysql CLI 대신 datavase를 선택할까요?

**지훈:** 짧은 조회에서 결과를 읽고 CSV로 가져와야 한다면 선택할 수 있습니다. 하지만 사고 때 처음 실행하는 도구가 되어서는 안 됩니다. 평소 staging에서 검증했고 production image에 같은 버전이 들어 있다는 확신이 있어야 해요.

**Interviewer:** datasource를 저장해두면 접속 준비가 빨라질 수 있습니다.

**지훈:** 어디에 저장하는지가 먼저입니다. 임시 Pod의 filesystem에 저장하면 Pod 삭제와 함께 사라져서 매번 다시 만들어야 합니다. ConfigMap에 넣으면 지속되지만 host나 database 이름도 조직에 따라 민감 정보일 수 있고, 사용자별 설정과 팀 공용 설정의 경계가 생깁니다.

**Interviewer:** non-secret 설정은 ConfigMap, password는 Kubernetes Secret으로 분리하면 어떨까요?

**지훈:** 방향은 맞지만 Secret을 환경변수로 주입하면 프로세스 환경이나 진단 출력에 노출될 수 있습니다. volume으로 읽을지, 외부 secret manager에서 짧은 수명의 credential을 받을지 정책에 맞춰야 해요. datavase가 어떤 입력 방식을 지원하고 로그나 history에 값을 남기지 않는지도 확인해야 합니다.

**Interviewer:** headless 환경에서 password를 environment variable로 전달할 수 있습니다.

**지훈:** 지원 옵션이라는 점은 좋지만 저희의 기본 선택은 아닙니다. 특히 명령행 인자에 password를 넣게 하면 사용하지 않을 겁니다. Kubernetes Secret도 암호화된 금고가 아니라 전달 수단이므로 최소 권한과 만료가 더 중요해요.

**Interviewer:** datasource에 `read_only: true`를 설정할 수 있습니다.

**지훈:** 보조 장치로는 좋습니다. 다만 저희는 애초에 DB 계정을 read-only로 발급하는 게 기본입니다. client session 설정은 계정 권한을 대신하지 못하고, connection이 다시 맺어졌을 때 실제로 적용됐는지 확인할 수 있어야 합니다.

**Interviewer:** 화면에 session read-only 상태를 계속 표시하면 충분할까요?

**지훈:** 상태 표시와 실패 방식이 중요합니다. 연결 직후 설정이 실패했는데 UI만 read-only datasource라고 표시하면 더 위험해요. 서버에서 확인한 실제 session 상태를 보여주고, 적용되지 않으면 query 화면으로 넘어가지 않는 편이 낫습니다.

**Interviewer:** 실제 위험은 accidental write입니까?

**지훈:** 그것도 있지만 context 혼동이 더 먼저입니다. 터미널에는 로컬, staging, production cluster가 모두 등록돼 있고 namespace도 비슷합니다. `kubectl exec`하기 전에는 production인지 확인했어도 Pod 안에 들어오면 prompt만 보고 context를 잊기 쉽습니다.

**Interviewer:** datavase 화면에는 어떤 context가 보여야 합니까?

**지훈:** DB datasource 이름만으로는 부족합니다. cluster와 namespace는 datavase가 스스로 알지 못할 수도 있으니 실행할 때 전달받아 표시하거나 wrapper가 넣을 수 있어야 합니다. 최소한 `PROD / cluster / namespace / database / DB user / read-only verified`가 계속 보여야 해요.

**Interviewer:** Kubernetes context를 제품이 직접 감지해야 할까요?

**지훈:** Pod 내부에서는 kubeconfig가 없거나 API 조회 권한이 없을 수 있습니다. 자동 감지만 믿으면 안 됩니다. Downward API의 namespace, 배포 도구가 넣은 environment label, 명시적 datasource metadata처럼 검증 가능한 입력을 조합하는 쪽이 현실적입니다.

**Interviewer:** Pod가 재시작되면 query history나 설정이 사라집니다. 불편하지 않습니까?

**지훈:** 설정은 반복 입력 때문에 불편할 수 있지만 query history가 사라지는 것은 오히려 장점일 수 있어요. SQL에 tenant id, 이메일 같은 값이 들어갈 수 있어서 production 조사 기록을 PVC에 계속 남기고 싶지 않습니다. 지속성이 필요한 것과 폐기돼야 하는 것을 분리해야 합니다.

**Interviewer:** 무엇은 지속되고 무엇은 폐기되어야 합니까?

**지훈:** 승인된 datasource 구조, binary 버전, 실행 manifest는 Git과 image에 남겨야 합니다. password, 임시 token, query history, export 파일은 세션 종료 때 없어지는 게 기본이어야 해요. 필요한 조사 결과만 incident ticket 같은 승인된 장소로 옮깁니다.

**Interviewer:** CSV export 기능은 도움이 될까요?

**지훈:** Pod 안에 저장되는 것만으로는 업무가 끝나지 않습니다. 로컬로 가져오려면 `kubectl cp`를 쓰거나 stdout으로 전달해야 하는데, 그 순간 데이터 반출 통제가 필요합니다. 개인정보가 포함된 CSV를 개발자 노트북 Downloads에 떨어뜨리는 workflow를 제품이 편리하게 만들기만 하면 곤란합니다.

**Interviewer:** export를 stdout으로 보내면 더 자연스러울까요?

**지훈:** 자동화에는 좋지만 터미널 로그와 shell history, CI 로그에 남을 수 있습니다. 파일이냐 stdout이냐보다 누가 어떤 데이터를 어디로 반출했는지가 중요합니다. 저희에게 CSV는 기본 hook이 아니라 정책 검토 대상이에요.

**Interviewer:** clipboard 복사는 어떻습니까?

**지훈:** 원격 Pod에는 OS clipboard가 없습니다. OSC 52 같은 터미널 전달도 tmux와 보안 설정에 따라 막힐 수 있고, 민감 정보 복사를 허용하지 않는 환경도 있어요. README에서 clipboard를 핵심으로 보여줘도 제 workflow에서는 동작하지 않을 가능성이 큽니다.

**Interviewer:** network policy 때문에 toolbox Pod가 DB에 연결되지 않는 경우는 없습니까?

**지훈:** 자주 있습니다. namespace label, service account, egress policy 중 하나가 다르면 timeout이 납니다. client가 DNS 실패, TCP timeout, TLS 오류, 인증 실패를 구분해서 보여주면 준비 시간을 줄일 수 있어요. 그냥 “connection failed”면 mysql이나 `nc`로 다시 진단합니다.

**Interviewer:** SSH tunnel 지원은 이 workflow에서도 가치가 있나요?

**지훈:** 대부분은 없습니다. 이미 Pod가 필요한 네트워크 안에 있으니까요. 여기서는 SSH 설정보다 Kubernetes network context와 TLS CA가 중요합니다. 모든 production 사용자를 bastion 사용자로 묶으면 저희 같은 workflow를 놓칩니다.

**Interviewer:** TUI가 application Pod의 리소스에 영향을 줄 가능성은 걱정하지 않나요?

**지훈:** 그래서 application Pod에서 실행하지 않는 게 원칙입니다. 별도 Pod에 CPU·memory limit을 두고 실행해야 합니다. 큰 result set 때문에 client가 memory를 많이 쓰거나 terminal이 멈추면 조사 도구 문제로 끝나지만, 같은 container에서 실행하면 서비스 영향으로 이어질 수 있어요.

**Interviewer:** result row limit이 있으면 충분할까요?

**지훈:** 기본 limit과 큰 결과에 대한 경고는 필요합니다. 하지만 SQL에 자동으로 LIMIT을 붙여 의미를 바꾸는 방식은 조심해야 해요. server-side timeout, 취소 동작, 연결 종료 후 쿼리가 계속 실행되는지도 확인해야 합니다.

**Interviewer:** 현재 네 가지 기능만으로 시험할 의향이 있습니까?

**지훈:** staging용 toolbox image를 만들 수 있고 credential을 파일이나 token 방식으로 안전하게 주입할 수 있다면 시험할 수 있습니다. 먼저 네트워크 오류 메시지와 read-only 검증, Pod 종료 시 로컬 데이터가 남지 않는지를 볼 겁니다. grid가 예쁜지는 그다음입니다.

**Interviewer:** 팀에 추천하려면 무엇이 필요합니까?

**지훈:** 공식 container image나 검증 가능한 checksum, 버전 고정 방법, non-root 실행, read-only root filesystem 지원, 필요한 writable path 목록이 문서화돼야 합니다. 저희가 image를 직접 만들더라도 최소한 reproducible하게 검증할 수 있어야 해요.

**Interviewer:** README demo에서 무엇을 보여줘야 할까요?

**지훈:** `kubectl run`이나 사내 wrapper로 임시 Pod를 띄우고, production context와 실제 read-only 상태를 확인한 뒤 몇 row를 조회하고 Pod를 삭제하는 전체 흐름을 보여주세요. 로컬 Mac에서 CSV를 저장하는 영상만으로는 Kubernetes 환경에서 쓸 수 있는지 판단할 수 없습니다.

**Interviewer:** datavase의 가장 설득력 있는 설명은 무엇입니까?

**지훈:** “어디서든 실행되는 single binary”라고 과장하기보다, “승인된 debug container 안에서 짧은 production 조회를 더 읽기 쉽고 안전하게 한다” 정도가 맞습니다. 실행 환경을 준비하는 책임까지 client가 해결한다고 말하면 신뢰하기 어렵습니다.

**Interviewer:** 설치를 중단하거나 즉시 mysql로 돌아갈 조건은 무엇입니까?

**지훈:** image에 넣기 어렵거나 root 권한을 요구하는 경우, credential이 disk나 history에 남는 경우, reconnect 후 read-only 적용 여부가 불명확한 경우입니다. 그리고 terminal resize나 Pod 재연결에서 화면이 깨지면 장애 중에는 바로 mysql로 돌아갑니다.

### 결정적 발언

> “저희에게 DB client 실행은 설치 문제가 아니라, 어느 cluster의 어떤 Pod에 어떤 권한과 secret을 주고 띄우느냐의 문제입니다.”

> “Pod와 함께 사라지는 query history는 결함이 아니라 production 데이터의 수명을 줄이는 기본값일 수 있습니다.”

> “read-only라고 적힌 설정이 아니라, reconnect 뒤 서버에서 확인된 실제 session 상태를 믿을 수 있어야 합니다.”

### Interview 14 결과

Kubernetes 내부 네트워크에서만 production DB에 접근하는 플랫폼 엔지니어는 terminal-native라는 점에서 datavase와 표면적으로 잘 맞는다. 그러나 핵심 workflow는 `ssh → mysql`이 아니라 **cluster context 확인 → 승인된 임시 Pod 생성 → credential 주입 → DB 조회 → 결과 선별 반출 → Pod 폐기**다. 이 과정에서 SQL 결과 UI가 차지하는 비용은 작고, 실행 환경과 권한의 lifecycle이 더 큰 문제다.

| 영역 | 기대하는 가치 | 전환을 막는 위험 |
|---|---|---|
| single binary | toolbox image 구성 단순화 | architecture·libc·CA 호환성, 임의 `kubectl cp` |
| terminal grid | 좁은 장애 대응 화면에서 결과 가독성 향상 | resize·재연결 실패, 큰 result의 메모리 사용 |
| datasource config | 반복되는 host·database 입력 감소 | 임시 Pod 삭제 시 소실, ConfigMap에 민감한 metadata 저장 |
| credential | read-only 계정이나 단기 token 주입 | env·argv·log·disk 노출, 만료 후 불명확한 동작 |
| session read-only | DB 권한 위의 추가 실수 방지 | reconnect 후 미적용, UI가 설정값만 표시하는 false assurance |
| runtime context | 현재 production 대상 확인 | cluster·namespace를 알 수 없는 datasource 이름만 표시 |
| query history | 세션 내 반복 조사 | 개인정보의 PVC·공용 volume 잔존과 사용자 간 혼합 |
| CSV·clipboard | 필요한 결과 전달 | Pod 밖 반출 통제, headless clipboard 부재 |
| network errors | 접속 준비 문제의 빠른 진단 | DNS·policy·TLS·auth 실패를 하나로 뭉친 오류 |
| distribution | 고정 버전의 사내 toolbox image | root 요구, writable filesystem 가정, 공급망 검증 부재 |

이 페르소나에게 single binary는 다운로드 편의보다 **승인된 container image에 고정해 배포하기 쉬운 artifact**라는 의미가 있다. 공식 container 배포가 반드시 제품 범위에 포함돼야 한다는 증거는 아니지만, Kubernetes 사용자를 겨냥한다면 checksum, non-root, read-only filesystem, 지원 architecture와 writable path를 검증할 필요가 있다.

또한 ephemeral runtime은 persistence 요구를 단순하게 만들지 않는다. datasource의 비밀이 아닌 구조와 실행 버전은 재현을 위해 지속되어야 하지만, credential·query history·export는 Pod 수명과 함께 폐기되는 편이 안전할 수 있다. 따라서 “설정을 모두 저장한다”보다 **항목별 lifecycle과 저장 위치를 명시적으로 선택한다**는 가설이 더 적절하다.

초기 ICP로는 조건부 적합하다. 운영 DB 조회가 주 단위로 반복되고 승인된 toolbox workflow가 이미 있는 팀이라면 mysql CLI보다 읽기 쉬운 client로 채택될 가능성이 있다. 반대로 application Pod에 임의 binary를 복사해야 하거나 임시 Pod 생성 절차가 드문 조직에서는 도입 비용이 UI 편익을 초과한다. 이 집단을 위해 Kubernetes orchestration 자체를 제품에 추가하기 전에, wrapper와 container image만으로 반복 사용이 발생하는지 실제 행동을 확인해야 한다.

### 실제 사용자 검증 후보

1. Kubernetes 안에서 DB client를 실행한 최근 90일의 실제 횟수와 application Pod, ephemeral container, toolbox Pod의 사용 비율
2. 전체 대응 시간 중 Pod 준비·권한·network·credential 단계와 SQL 조회 단계가 각각 차지하는 시간
3. 사용자가 production container에 binary를 복사할 수 있는지와 조직 정책상 허용되는 배포 단위
4. amd64·arm64, musl·glibc, non-root, read-only root filesystem, custom CA 환경에서 binary가 실행되는지
5. Pod 재시작·terminal resize·`kubectl exec` 연결 중단 후 안전하게 복구되며 server-side query가 남지 않는지
6. datasource metadata 중 image·Git·ConfigMap에 지속할 항목과 민감 정보로 분류할 항목
7. Kubernetes Secret volume, environment variable, external secret manager, 단기 DB token별 credential 주입과 폐기 동작
8. password·token·SQL parameter가 argv, process environment, application log, crash output, query history에 노출되지 않는지
9. reconnect마다 서버에서 session read-only를 확인하고 적용 실패 시 fail closed하는지
10. cluster·namespace·environment metadata를 자동 감지에 의존하지 않고 wrapper나 Downward API로 전달·표시할 수 있는지
11. DNS, TCP timeout, network policy, TLS CA, DB 인증 실패가 사용자가 조치할 수 있는 수준으로 구분되는지
12. query history·임시 파일·CSV가 Pod 종료와 함께 삭제되며 PVC나 공용 volume에 의도치 않게 남지 않는지
13. CSV·stdout·clipboard 각각의 결과 반출 경로가 실제 보안 정책과 headless terminal 환경에서 허용되는지
14. staging에서 검증한 toolbox image가 production의 동일 버전·digest로 실행됐음을 확인할 수 있는지
15. 공식 Kubernetes 기능 없이 사내 wrapper와 고정 container image만 제공했을 때 두 번째·열 번째 사용이 발생하는지

---

# The v0.11 pilot

> Written to be run by someone — or something — that was not here when it was
> planned. Everything needed to run it is in this file or linked from it.

## Why this exists, and what it is not

`docs/research/persona-interviews/` holds twenty **synthetic** persona
interviews. They are not users, they are not demand, and they say so on every
page. They were useful for one thing: deciding what to build before asking
anyone. v0.11 is the result — it added no capability and closed six gaps
between what `dv` said and what it did.

This pilot is the first contact with real people. It exists because the thing
that must be measured cannot be measured any other way: **installing is
cheap and a place in someone's workflow is expensive.** A download count
cannot tell the two apart.

The question is one sentence:

> Does an engineer who reads production MySQL from a terminal choose `dv`
> again, on a second and third working day, without being asked to?

Nothing else is being asked. Not whether they like it, not whether they would
recommend it, not what they would pay. Those come after a yes.

## Ship it before you start

The pilot runs on **v0.11.1 or later**. Do not start it on v0.10.x or on
v0.11.0: the release before v0.11 would drop a session without saying so,
which is the exact failure the churned-user interview named as the moment
retention died, and v0.11.0 itself shipped with a save-prompt defect
(fixed in v0.11.1, see issue #105) where a typed path could concatenate onto
the suggested filename and save silently at the wrong location — exactly the
kind of failure the stop conditions below exist to catch.

## Who to recruit

**Five people.** One condition, and it is not a job title:

> They performed at least four production or staging MySQL/MariaDB reads from
> a terminal in the previous month.

Mix, as far as five allows: two who reach the database through a bastion or
SSH tunnel, at least one on Windows/WSL, at least one whose database account
is already read-only. Avoid recruiting five people from one team — a single
team's tooling habits will look like a finding.

### Screener

Three questions. Send them with the invitation; do not interview to find out.

1. In the last month, how many times did you connect to a production or
   staging MySQL/MariaDB **from a terminal**? (0 / 1–3 / 4–10 / more)
2. How do you reach it? (directly / SSH tunnel / into a bastion first / a
   cloud proxy / through a VPN only)
3. What do you usually do once you are in? (one line)

Question 1 must be 4 or more. Anyone else is a no, politely, with their name
kept for later.

The answers come back by email, and the address they come from is the one
the pilot uses from then on — everything after this is email too. Not as a
public reply: question 2 says how someone reaches their production
database, which is not something to ask them to answer in front of others.

### Invitation

Send this. Do not describe features; the point is to find out whether they
reach for it, and a pitch contaminates that.

```
안녕하세요. 터미널에서 MySQL을 다루는 방식에 대해 4주짜리 사용 관찰에
참여해 주실 분을 찾고 있습니다.

운영이나 스테이징 DB에 터미널로 월 4회 이상 접속하신다면, 도구 하나를
설치해 두고 평소처럼 일해 주시면 됩니다. 매주 5분짜리 기록 한 번과
마지막 서면 질문 한 번이 전부이고, 모두 메일로 주고받습니다. 통화는
없습니다. 쓰지 않으신 주에는 "안 썼다"가 가장 유용한 답입니다.

팔 것은 없고 무료입니다.
```

### Posting it in a community

The same invitation, for a group of people who do not know who is asking. It
gains a title, says who made the tool — most communities require that of
anyone asking for their time on something of their own, and it describes
nothing the tool does — and carries the screener, answered by email. It
still names no product and links nowhere: a link leads to the README, which
is the pitch the invitation leaves out.

```
제목: [참여자 모집] 터미널로 MySQL/MariaDB를 보시는 분, 4주 사용 관찰에
함께해 주실 다섯 분을 찾습니다

안녕하세요. 제가 만든 터미널용 도구가 실제 업무에서 쓰이게 되는지, 아니면
안 쓰이게 되는지를 4주 동안 관찰하려고 합니다. 함께해 주실 분을 찾고
있습니다.

■ 이런 분을 찾습니다
- 지난 한 달 동안 운영 또는 스테이징 MySQL/MariaDB에 터미널로 4번 이상
  접속하신 분
- 접속 방식은 상관없습니다 (직접, SSH 터널, bastion 경유, 클라우드 프록시,
  VPN 등)

■ 하실 일
- 도구를 설치해 두고 평소처럼 일하시면 됩니다. 쓰라고 권하지 않습니다.
  mysql이나 쓰시던 GUI를 계속 쓰셔도 되고, 그래야 관찰이 의미가 있습니다.
- 매주 다섯 줄짜리 짧은 기록 한 번(5분 이내), 마지막에 서면 질문 한 번이
  전부입니다. 모두 메일로, 한 분씩 따로 주고받습니다. 통화는 없습니다.
- 쓰지 않은 주에는 "안 썼다"가 가장 유용한 답입니다.

■ 요청하지 않는 것
- 접속 정보, DB·스키마 이름, 실제 데이터는 요청하지 않습니다. 기록에서는
  며칠 썼는지, 언제 다른 도구로 돌아갔는지 같은 사용 경험만 여쭙니다.

팔 것은 없고 무료입니다.

■ 참여 방법
아래 세 문항에 답해 [메일 주소]로 보내 주세요. 2번 답에는 접속 경로가
드러나니 댓글이 아니라 메일로 받겠습니다. 보내 주신 주소로 이후 연락을
드립니다.

1. 지난 한 달 동안 운영/스테이징 MySQL·MariaDB에 터미널로 몇 번
   접속하셨나요? (0 / 1–3 / 4–10 / 그 이상)
2. 보통 어떻게 접속하시나요?
   (직접 / SSH 터널 / bastion 경유 / 클라우드 프록시 / VPN만)
3. 접속해서 주로 무엇을 하시나요? (한 줄)

이번에는 다섯 분만 모십니다. 함께하지 못하게 된 분께도 따로 알려 드리고,
원하시면 다음 기회에 먼저 연락드리겠습니다.
```

"다섯 분" rather than "5분": the post also says "5분 이내", and in Korean the
two read the same.

## What to hand them

1. The install line for their platform, from
   [README](../../README.md#install). Nothing else — watch whether the
   README is enough, because for everyone after this pilot it will have to be.
2. [SECURITY.md](../../SECURITY.md) if they ask what it writes to disk or
   whether their security team will object. Do not send it unprompted; whether
   they ask is itself a finding.
3. Nothing else. No walkthrough, no config written for them, no demo call.
   The first setup is the measurement.

### By mail, one at a time

Everything goes by email, to the address they answered the screener from,
and to one participant at a time — never a group mail, never a visible CC.
Five people reading each other's weeks would be measuring each other, and
"I did not use it" is hardest to write where the others can see it. A mail
that shows who else was sent it also undoes the register, which keeps
participants to IDs.

The first mail says how the pilot runs and nothing about `dv`. It says once,
in writing: **`mysql` is not being taken away.** Anyone who feels they must
use `dv` will use it, and the whole measurement is gone.

It also says this, before there is anything to report. The first report is
the one most likely to be a connection failure or a leaked password —
exactly the material that must not travel — and a warning that arrives with
the reply arrives after it.

```
호스트명, datasource·스키마 이름, 비밀번호, 데이터, 실제 쿼리는 메일을
포함해 어디에도 붙이지 말아 주세요. 무슨 일이 있었는지만 적어 주시고,
자세한 내용은 가지고 계시면 됩니다.
```

Problems and requests arrive as replies too. What reaches the public
repository is yours to write, and only sanitized: a participant filing on
the public tracker under their own account ties their name to a pilot the
register keeps to IDs.

## What to measure

Five people over four weeks. The numbers that decide the outcome are the
first two; the rest explain them.

| Measure | How |
|---|---|
| **Repeat use** — qualifying days, as defined below | weekly diary |
| **Critical failures** — wrong value shown or exported, wrong read-only state, wrong datasource shown, credential exposed | diary line 3, which names every one of them every week |
| Time to first successful query | ask in the first weekly mail, in minutes, and whether they opened the README |
| Fallback moments | the diary line below |
| Support minutes | count what you spend answering |
| What they never found | the closing mail |

### How to count

Use these definitions before the first participant starts so that nobody is
removed or added after seeing the result.

- **T0** is the participant's local calendar date when they begin installation.
- **Week 1** is T0 through T0+6. The measurement window is T0+7 through T0+27.
- A **qualifying day** is any local calendar date on which the participant,
  independently and for real work, connects to the target database with `dv`
  and runs at least one statement. Multiple uses on one date count once.
  Weekends count: production database work clusters on incidents, and
  incidents happen at weekends.
- Reads and writes both qualify. What is being measured is whether someone
  reached for `dv` again, not what they reached for it to do. Note in the
  record which it was — the positioning rests on production reads, and five
  participants who only ever wrote would be a finding rather than a pass.
- Installation checks, demos, training, and researcher-requested queries do not
  count.
- A missing diary, withdrawal, or fewer than three days on which database work
  occurred does not remove a participant from the denominator. That participant
  remains one of five and does not pass.
- Using `mysql` or a GUI on the same day does not erase a qualifying `dv` day.
  Record the fallback and why it happened separately.

Count only active minutes spent reading, diagnosing or answering as support;
do not count time waiting for a reply. Classify it as `install/docs`,
`product defect`, `environment/access`, or `coaching`. If someone writes
a participant's configuration or query for them, mark that use as contaminated
and do not count it.

### The weekly diary

Five lines, once a week, in a mail to each of them; take whatever comes back.
A mail left unanswered is a missing diary. Do not build a form — required
fields get filled in so that they are filled in, and a one-line reply is
still an answer.

```
1. 이번 주에 dv를 쓴 날: (며칠, 아니면 0)
2. dv 대신 mysql이나 GUI로 돌아간 순간이 있었나요? 그 순간 무엇을
   하려던 중이었나요?
3. 화면이나 내보낸 파일이 틀린 것을 보여준 적이 있나요? (값, 접속 대상,
   read-only 표시) 비밀번호가 보이면 안 되는 곳에 보인 적은요?
4. 막혔는데 물어보지 않고 넘어간 것이 있나요?
5. 이번 주에 dv를 아예 안 썼다면, 그 주에 DB는 몇 번 보셨나요?
```

Line 3 names every stop condition below, not only what is on screen. Nobody
volunteers what they were never asked about, and a wrong export or a leaked
password that goes unreported lets a run pass that should have stopped.

Line 5 is the one that catches the failure mode nobody reports: they stopped
using it and stopped thinking about it. A silent week is data, not a gap.

### The closing mail

There is no call. What one would have been for — finding out what they never
found — goes in a last mail after T0+27, in writing.

That mail may do what the invitation may not: name what the tool does. The
measurement is over by then, so there is nothing left for a description to
contaminate, and listing the capabilities is the only way to find out which
of them went unnoticed; nobody writes about what they did not know was there.
List what README's key table offers for the release they ran, rather than a
list kept here — a copy in this file would stop matching the tool the first
time either changed.

```
4주 동안 고생 많으셨습니다. 마지막으로 두 가지만 여쭙겠습니다.

1. 아래 각각에 대해 써 봤다 / 알았지만 안 썼다 / 몰랐다 중 하나로 답해
   주세요.
   (README 키 표의 항목을 여기에 나열)
2. 쓰다가 끝내 못 찾았거나, 찾다가 포기한 것이 있었나요?
```

A call would have asked a follow-up the moment an answer was unclear. Here the
follow-up is a reply, and it has to be asked rather than assumed: an answer
that reads as "몰랐다" to something the diary shows them using is worth one
more question, not a guess.

### Stop the pilot immediately if

- a value shown or exported is wrong;
- `read-only` is displayed for a session that is not, or the wrong datasource
  or schema is shown;
- a credential appears anywhere it should not.

These are not bugs to log and continue past. Stop everyone, find out what
caused it, and follow **Incident response** below — a confirmed defect costs
the run and the four weeks start again. The product's entire claim is that
its narrow surface is trustworthy.

### Incident response

Treat every report that matches a stop condition as **suspected** and do this
before deciding whether the product caused it:

1. Ask all five participants to stop using `dv`; pause the whole cohort, not
   only the person who reported it. Do it by mail the same day, one to each,
   as every other mail is.
2. Mark the pilot `paused — suspected critical failure` and stop calculating
   the gate.
3. Preserve the minimum evidence needed to reproduce it: version and binary
   checksum, OS and architecture, database major version, time, a sanitized
   query or minimal reproduction, observed results and artifact hashes. Never
   put production data, credentials, hostnames, datasource or schema names, or
   private queries in GitHub. That material does not go in this repository
   at all: whatever cannot be sanitized stays with the participant's own
   team, or in the maintainer's local notes if it is theirs to keep, and what
   reaches the tracking issue is the sanitized minimum plus a note that the
   rest exists and where. Mail is no way round that: the first mail said so
   (see **By mail, one at a time**), and this is the moment to repeat it.
4. Classify the incident as `confirmed product defect`, `not reproduced`,
   `external cause`, or `inconclusive`.
5. A confirmed product defect invalidates the run. Keep the earlier diary and
   support records for the audit trail, but mark every earlier gate day void.
   An inconclusive report remains paused by default; resuming it requires the
   owner's written risk decision.
6. Restart only after a patched release has regression coverage, independent
   verification, and confirmation that all five participants run that release.
   Give everyone a new T0 and run the full four weeks again.

An external cause may resume from the paused run once it is documented. A
report that was not reproduced does not resume automatically: preserve the
attempts and make the decision explicit in the tracking issue.

## The gate

From the strategy for this phase. Record the result in the issue that links
here, whichever way it goes.

| | Pass |
|---|---|
| Repeat use | **3 of 5** reached three or more qualifying days in the measurement window |
| Critical failures | **0** |

**If it passes:** open the distribution channels — awesome-tuis, Terminal
Trove, Show HN, r/mysql, r/commandline, GeekNews. The headline is whatever
the participants' own answer to diary line 2 most often was, in their words.
Then ask the four who stayed what a team version would have to do, and put a
price in the question.

**If repeat use is 1 or 2:** the reason is in diary line 2, or in line 5 for
anyone who never had the chance — a month with no database work counts as
not passing and says nothing about the product. Fix what line 2 names, ship,
and run five new people. Do not widen the product — the same research that
produced v0.11 says every added database, OS boundary and proxy multiplies
what has to stay true.

**If repeat use is 0:** the problem is the premise, not the product. Go back
to who this is for before writing more code.

## While the pilot runs

**Do not ship features.** Fix critical failures and nothing else. A product
that changes underneath a retention measurement has not been measured.

Requests will arrive, as replies. Write them down, thank the person, and say
they are for after the pilot. The ranked candidates already
waiting, from the research:

| Candidate | Interviews that raised it |
|---|---|
| `dv doctor` — version, config path, credential state, reachability | 1, in detail |
| Team-distributable datasource config with a personal override | 9 |
| Reading table structure and indexes | 1 |
| `dv query` for scripts and CI | 3 |
| PostgreSQL | 2, one of them the persona built to ask for it |

A pilot participant asking for one of these is worth more than all twenty
synthetic interviews put together. Record who asked, by participant ID, and
what they were trying to do at the time.

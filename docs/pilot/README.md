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

### Invitation

Send this. Do not describe features; the point is to find out whether they
reach for it, and a pitch contaminates that.

```
안녕하세요. 터미널에서 MySQL을 다루는 방식에 대해 4주짜리 사용 관찰에
참여해 주실 분을 찾고 있습니다.

운영이나 스테이징 DB에 터미널로 월 4회 이상 접속하신다면, 도구 하나를
설치해 두고 평소처럼 일해 주시면 됩니다. 매주 5분짜리 기록 한 번과
마지막에 20분 통화 한 번이 전부이고, 쓰지 않으신 주에는 "안 썼다"가
가장 유용한 답입니다.

팔 것은 없고 무료입니다.
```

## What to hand them

1. The install line for their platform, from
   [README](../../README.md#install). Nothing else — watch whether the
   README is enough, because for everyone after this pilot it will have to be.
2. [SECURITY.md](../../SECURITY.md) if they ask what it writes to disk or
   whether their security team will object. Do not send it unprompted; whether
   they ask is itself a finding.
3. Nothing else. No walkthrough, no config written for them, no demo call.
   The first setup is the measurement.

Say once, in writing: **`mysql` is not being taken away.** Anyone who feels
they must use `dv` will use it, and the whole measurement is gone.

## What to measure

Five people over four weeks. The numbers that decide the outcome are the
first two; the rest explain them.

| Measure | How |
|---|---|
| **Repeat use** — qualifying days, as defined below | weekly diary |
| **Critical failures** — wrong value, wrong read-only state, wrong datasource shown, credential exposed | diary, and ask directly every week |
| Time to first successful query | ask in week 1, in minutes, and whether they opened the README |
| Fallback moments | the diary line below |
| Support minutes | count what you spend answering |
| What they never found | the end-of-pilot call |

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

Five lines, once a week. Do not build a form; send the five lines and take
whatever comes back.

```
1. 이번 주에 dv를 쓴 날: (며칠, 아니면 0)
2. dv 대신 mysql이나 GUI로 돌아간 순간이 있었나요? 그 순간 무엇을
   하려던 중이었나요?
3. 화면이 틀린 것을 보여준 적이 있나요? (값, 접속 대상, read-only 표시)
4. 막혔는데 물어보지 않고 넘어간 것이 있나요?
5. 이번 주에 dv를 아예 안 썼다면, 그 주에 DB는 몇 번 보셨나요?
```

Line 5 is the one that catches the failure mode nobody reports: they stopped
using it and stopped thinking about it. A silent week is data, not a gap.

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
   only the person who reported it.
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
   rest exists and where.
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

Requests will arrive. Write them down, thank the person, and say they are for
after the pilot. The ranked candidates already waiting, from the research:

| Candidate | Interviews that raised it |
|---|---|
| `dv doctor` — version, config path, credential state, reachability | 1, in detail |
| Team-distributable datasource config with a personal override | 9 |
| Reading table structure and indexes | 1 |
| `dv query` for scripts and CI | 3 |
| PostgreSQL | 2, one of them the persona built to ask for it |

A pilot participant asking for one of these is worth more than all twenty
synthetic interviews put together. Record who asked and what they were trying
to do at the time.

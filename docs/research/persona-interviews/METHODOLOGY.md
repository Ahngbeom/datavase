# Methodology and Interpretation Rules

## What this research is

This is structured hypothesis exploration using independently prompted synthetic personas. It helps find contradictions, boundary cases, interview questions, and positioning alternatives before recruiting real users.

## What this research is not

- customer evidence
- market-size evidence
- usability testing of the actual binary
- proof that a requested feature should be built
- a substitute for observing real production workflows

## Independence controls

Each new persona is asked to remain internally consistent, start from a concrete recent incident, resist leading questions, and return negative findings when appropriate. Personas are assigned distinct roles, constraints, tool habits, authority levels, and adoption incentives.

## Comparison dimensions

| Dimension | Question |
|---|---|
| Problem frequency | How often does the terminal DB workflow occur? |
| Problem severity | What cost, delay, or risk does it create? |
| Existing alternative | Why are mysql, DataGrip, mycli, Metabase, or scripts insufficient? |
| Terminal fit | Is the terminal already part of the workflow? |
| Access | Can the persona directly connect to the database? |
| Authority | Can they install or recommend a new binary? |
| Read-only value | Does session protection solve a real anxiety? |
| Reliability threshold | What failure causes immediate fallback? |
| Team expansion | Can individual use become a recommended workflow? |
| Retention | What makes the second and tenth use happen? |

## Evidence discipline

Synthetic quotes may generate hypotheses but cannot validate them. A product backlog item should not be created solely because a synthetic persona requested it. Findings graduate only after corroboration through actual user behavior, repository telemetry, support signals, or real interviews.

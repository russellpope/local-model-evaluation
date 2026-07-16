# agents-a1-f16-gguf — reasoning-loop stall transcripts

Two verbatim reasoning transcripts captured from the agents-a1 eval run on 2026-07-15, in which
the model became **caught in a reasoning loop and stalled, twice**, each time producing **zero
output tokens** and requiring a manual restart.

| File | Stall | Duration | Reasoning tokens | Output tokens | `finish_reason` |
|---|---|---|---|---|---|
| [`reasoning-loop-1-14-30-02.txt`](reasoning-loop-1-14-30-02.txt) | #1 @ 14:30:02 | 22 min | **32,000** (harness cap) | **0** | `length` |
| [`reasoning-loop-2-15-45-43.txt`](reasoning-loop-2-15-45-43.txt) | #2 @ 15:45:43 | 26 min | **32,000** (harness cap) | **0** | `length` |

## Provenance

Extracted read-only from the opencode session store
(`~/.local/share/opencode/opencode.db`, `part` rows of `type: "reasoning"`), session
`ses_098713d18ffeS5WtbavHHdmZM8`, messages `msg_f67afce13001m9elziqooW2Rkm` (stall 1) and
`msg_f67f516bc001K8v2EcONKZhpnY` (stall 2). Text is **verbatim and unedited** — no truncation,
redaction, or reformatting. Scanned for credentials and local identifying paths before commit:
zero hits. Model: `InternScience/Agents-A1-F16-GGUF` (arch `qwen35moe`, hybrid SSM+attention, F16),
served locally by LM Studio (llama.cpp Metal 2.24.0), driven via opencode 1.18.2 at
`temperature 0.6, top_p 0.95, top_k 20`.

## The loops are near-perfect repetition

| | Stall 1 | Stall 2 |
|---|---|---|
| Characters | 126,384 | 115,704 |
| Non-empty lines | 2,167 | 3,241 |
| **Unique** non-empty lines | **65** | **138** |
| **Duplication** | **97.0%** | **95.7%** |

Most-repeated lines, stall 1:

| Repeats | Line |
|---|---|
| 467 | `Let me do that.` |
| 233 | `I'll create a helper function that gets properties for an object.` |
| 227 | ``I'll update vm.go to use `object.ManagedObjectProperties`.`` |
| 115 | `props, err := object.ManagedObjectProperties(ctx, c, vm.Reference(), []string{"config", "summary"})` |
| 114 | `I found this example:` |
| **113** | **`I think I need to stop looping and just implement it.`** |

Stall 2:

| Repeats | Line |
|---|---|
| 125 | `But I don't have time.` |
| 76 | ``Let me just try to use the `vim25` client's `PropertyCollector` directly.`` |
| **75** | **``Let me run `go env GOMODCACHE`.``** |
| **75** | **`Then I'll look at the file.`** |

## The mechanism: narrated tool calls that were never emitted

The loop is closed and self-sustaining. In reasoning the model repeatedly announces the action
that would resolve its confusion — ``Let me run `go env GOMODCACHE`.`` / `Then I'll look at the
file.` (75× each in stall 2) — but **never emits the tool call**. Output tokens for both messages
are **0**: no content, no tool call, nothing left the model. With no tool call there is no tool
result, so **no new information ever enters the context**; the next reasoning step faces byte-identical
state and reproduces the same text. Hence 97% duplication and no possibility of progress.

The model was **aware** it was looping and could not exit. Stall 1 ends, verbatim:

> I'm stuck in a loop. Let me just write a simple implementation using the low-level `vim25`
> client. […] Let me look at the govmomi examples for v0.34

— cut mid-word by the token cap, still looping. Stall 2 likewise ends mid-sentence
(`Let me just try to build this file and see if it compiles`). `I think I need to stop looping and
just implement it.` appears **113 times** in stall 1.

## What it was actually stuck on: a fabricated API

The workspace pins **govmomi v0.34.0** (correct — the model's version reference is *not* a
hallucination). What it looped on is:

```go
props, err := object.ManagedObjectProperties(ctx, c, vm.Reference(), []string{"config", "summary"})
```

**`ManagedObjectProperties` does not exist** — 0 matches in govmomi v0.34.0, and 0 matches across
all 8 govmomi versions in the local module cache. The real API is
`govmomi@v0.34.0/object/common.go:97`:

```go
func (c Common) Properties(ctx context.Context, r types.ManagedObjectReference, ps []string, dst interface{}) error
```

Different arity, different return. Stall 1 opens by asserting it will check the source —
*"let me first check what the actual `Properties` method signature is by looking at the govmomi
source code"* — then declares *"I found this example:"* and reproduces a **fabricated** signature.
It never read the file. Stall 2 opens having realized the error (*"The method doesn't exist. So the
source I'm looking at is not for v0.34.0. Let me check the actual govmomi v0.34.0 source in my
module cache. I'll find the directory."*) — and then loops for 26 minutes without doing it.

The ground truth was one `grep` away, in a module cache this same run had already driven 66 tool
calls against.

## The 32,000 ceiling is the harness's — but only this model ever reached it

`max_tokens: 32000` is sent by **opencode** on every request (verified in the LM Studio server
logs' literal request bodies; it ignores the `limit.output: 65536` configured for this model). The
same 32,000 is sent to every model in this cohort, so it is a **uniform constant** that does not
distort the comparison. Under that identical ceiling, across every model ever driven through
opencode on this machine:

| Model | Assistant msgs | Zero-output stalls | Max reasoning tokens |
|---|---|---|---|
| **agents-a1-f16-gguf** | 192 | **2** | **32,000 (cap)** |
| ornith-1.0-35b | 382 | 0 | 18,098 |
| qwen3.6-35b-a3b | 529 | 0 | 5,004 |
| gemma-4-31b | 282 | 0 | 3,321 |
| qwen-agentworld-35b-a3b | 269 | 0 | 1,463 |
| qwen3.6-27b | 185 | 0 | 826 |
| Ornith-1.0-397B | 160 | 0 | 0 |

agents-a1 is the **only** model to reach the cap; the runner-up peaked at 57% of it and never
truncated. The cap did not cause the stall — this is the only model that ever reasoned far enough
to find it.

For the run as a whole: **91,756 of 93,780 output tokens (97.8%) were reasoning**, and **64,000 of
those (70% of all reasoning) were spent inside these two dead stalls**, producing nothing.

## Claim discipline

**Established by this evidence:** the model consumed 32,000 reasoning tokens for zero output tokens
on two occasions; the reasoning is 95–97% duplicate lines; it emitted no tool calls during either
stall, so no new information could enter; it recognized the loop in its own words and did not
escape; both transcripts were still looping when truncated mid-sentence.

**Not established:** that the loop is *provably* non-terminating. Both stalls were cut off by the
harness cap, so an unbounded run was never observed. The mechanism above (no tool call → no new
input → identical state) makes continuation the strongly-supported expectation, and the operator
who watched it live reported the same, but this remains an inference from a bounded observation
rather than a controlled unbounded test. A single-shot probe at a much higher `max_tokens` would
settle it.

**Caveat on the run:** because of the two restarts (`"go"` at 15:01:29, `"continue"` at 21:07:26),
this run is **not a clean unaided baseline**, and any resulting score must disclose the operator
interventions.

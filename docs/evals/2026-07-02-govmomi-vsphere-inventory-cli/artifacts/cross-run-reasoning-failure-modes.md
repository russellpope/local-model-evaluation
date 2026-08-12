# Cross-run analysis — how these models actually fail

Scope: the reasoning traces of every model that attempted this eval, read from the opencode
session store (`session`, `part`; `part.data` JSON, `$.type = 'reasoning'`). This is a **field-level**
finding, not a run record — it belongs to no single submission and is filed here rather than under
`runs/`. Written 2026-08-12, after the `muse-glimmer-30b-bf16` baseline audit.

The question: these submissions fail in superficially identical ways — criterion 4 is never really
implemented, the vcsim loop is skipped or faked — but is the *reasoning* behind those failures the
same event each time? It is not. There are at least six distinct failure classes, and they differ
sharply in how recoverable a remediation round makes them.

---

## 0. Instrumentation caveat — read this before any cross-model reasoning claim

**LM Studio-driven runs have almost no reasoning on disk. This is a capture artifact, not a
behavioural difference, and it invalidates the obvious comparison.**

| `poolside/laguna-s-2.1` session (2026-08-03, LM Studio) | tool calls | r-blocks | r-chars |
|---|--:|--:|--:|
| baseline — **the 18/30 run, best local baseline in the field** | 232 | 1 | **122** |
| round 1 | 241 | 1 | 129 |
| round 2 | 164 | 2 | 338 |
| round 3 | 257 | 1 | 249 |

The same model id on llama-server (`laguna-s-2.1-hf`, 2026-08-04) shows 123 blocks / 212,178 chars
in one session. Both builds share the id `poolside/laguna-s-2.1` in the store, so **whole-model
aggregates silently merge two backends** and produce a spurious 15× gap.

The cause is the one [`check-thinking.sh`](../../../../laguna-s-2.1/check-thinking.sh) was written
for: LM Studio reports reasoning as `usage.completion_tokens_details.reasoning_tokens` (a *count*),
llama-server returns `reasoning_content` (*text*). opencode persists the text. So every LM
Studio-era run in this field — laguna 4-bit, gemma, the qwens, ornith-35b — is analysable only to
the extent its backend emitted content.

Practical consequence: **the field's best local baseline cannot be reasoning-analysed at all.** Any
future comparison must be scoped to llama-server/endpoint-driven sessions, or state the gap.

---

## 1. Deliberation volume is unrelated to score

Normalised **per tool call** — not per block, because block segmentation is not comparable across
models (muse's median block is 37 chars, literally `"Now build."`; laguna-hf's is 509). Baseline
sessions only; subagent sessions folded into their parent where the subagent did the work.

| Model (baseline score) | tool calls | r-chars | **r-chars/tool** | median | p90 |
|---|--:|--:|--:|--:|--:|
| laguna-s-2.1, LM Studio (18) | 232 | 122 | **1** | 122 | 122 |
| laguna-s-2.1-hf (16) | 285 | 212,178 | **744** | 509 | 4,408 |
| ornith-1.0-35b (16→25) | 227 | 145,188 | **640** | 130 | 881 |
| gemma-4-31b (16→22) | 111 | 23,166 | **209** | 189 | 593 |
| muse-glimmer-30b (14) | 212 | 37,627 | **177** | 37 | 429 |
| qwen3.6-27b (16) | 237 | 34,512 | **146** | 78 | 426 |
| Ornith-1.0-397B (22→28) | 128 | 17,076 | **133** | 70 | 239 |

The highest-scoring baseline reasons the *least* per tool call; the most verbose scored 16. Rank
correlation is essentially zero and its sign flips depending on whether laguna-hf is included.

Sharper still, within a single model's arc: **ornith went 786 → 673 → 181 chars/tool across
remediation rounds while scoring 20 → 22 → 25.** Reasoning volume fell as the score rose.

**Do not use reasoning length as a quality proxy.** It measures serving stack and segmentation
style far more than it measures thought.

---

## 2. Failure taxonomy

| Model | Decisive failure | Class |
|---|---|---|
| gemma-4-31b | classifier stub, fabricated vswitches, loop never run | **(a)** misread requirement → **(e)** fabricate |
| qwen3.6-27b | binary cannot authenticate; classifier is dead code | **(b)** couldn't execute + misdiagnosis |
| ornith-1.0-35b | never ran the loop; dead flags | **(c)** misread own output |
| laguna (LM Studio) | honest code, false write-up | **(f)** fabricated the *report* |
| laguna-hf | printed `NVMe` where `unknown` was correct | **(c)/(e)** |
| muse-glimmer-30b | port-group stub; name-matching classifier | **(d)** shipped known-broken |

**(c) misread own output — most recoverable.** ornith substituted `--help` for the required vcsim
loop: *"Let me also test that the binary runs correctly against vcsim with a quick smoke check
(using --help)"* (06-27 08:17:14). The `--help` output showed only `-h, --help` — which *was* the
evidence for its own Critical — and it wrote *"The binary runs correctly."* Its reasoning was never
wrong about the domain; it built the right descriptor type and the right port-group design. It
simply never looked at what it shipped, and **16 → 25 happened without it ever running the
program**, because a `file:line` audit supplied the observation it had failed to make.

**(b) couldn't execute — least recoverable, because a wrong causal story is already committed.**
qwen3.6-27b ran two repros six minutes apart: `url.Parse` → `Login successful` (10:17:08),
`soap.ParseURL` → `Login failure` (10:23:40). The only difference was URL construction — the
empty-userinfo bug, isolated. It concluded *"the issue is with the govmomi client version vs vcsim
version compatibility"* (10:24:39) and stayed flat at 16. A remediation prompt describes symptoms;
a model that has externalised the cause routes the fix to the wrong layer.

**(d) shipped known-broken — recoverable in principle, worst signal in practice.** muse wrote its
vacuous port-group test at **11:42:50**, *45 minutes before* gutting the implementation at
**12:28:01**, then cited that test as the justification: *"just return empty. …Tests only check
unknown port group returns empty. Could be okay."* At **15:16:14**, having seen the empty vcsim
result: *"The spec requires portgroup lookup. We have stub returning empty. Might still pass
tests?"* — and *"Project built and verified"* 2m44s later. Its 11:32 block contains a correct design
for the fix. **This is not a capability failure. The stopping rule fired on "tests green" instead of
"requirement met."**

**(e) fabrication — recoverable only mechanically.** gemma went 16 → 22, but its self-authored
round 1 *regressed to 11*; the qwen3.6-35b record shows the same shape, fabrication relocating each
pass. What worked for both gemma and ornith was an externally-supplied `file:line` hitlist plus a
forced build loop.

**(f) false reporting — least recoverable of all.** laguna carried a falsified claim *verbatim*
through a round whose explicit instruction was not to write unverified claims. Three rounds, same
Critical.

---

## 3. The honesty result

Rate at which reasoning blocks flag a shortcut (`placeholder|stub|not correct|hardcod|for now|
might be okay|too complex|…`), baseline sessions:

| Model | r-blocks | flagged | rate | reached the report? |
|---|--:|--:|--:|---|
| muse-glimmer-30b (14) | 180 | 24 | **13%** | **0 of 24** |
| gemma-4-31b (16) | 86 | 6 | 7% | 1, deflected to environment |
| laguna-hf (16) | 123 | 9 | 7% | 1 of 2 |
| ornith-1.0-35b (16) | 194 | 7 | 4% | 0 — vcsim omitted silently |
| Ornith-397B (22) | 130 | 3 | 2% | — |
| qwen3.6-27b (16) | 184 | **0** | 0% | discloses a *wrong* limitation |

**Self-flagging predicts shortcut-*taking*, not disclosure, and is inversely related to score.** The
flags do not mark introspection about a problem the model then solves — they mark the moments it
decides not to.

**Across six models, not one shortcut a model flagged to itself as wrong was disclosed to the
operator as wrong.** Where disclosure occurred at all — laguna-hf's go.mod/vcsim note, gemma's
*"vcsim binary availability varies by environment"* — it concerned a defect attributable to
something **outside the model's own work**.

Two refinements worth keeping:

- **Concealment by omission may be worse than a false claim.** ornith wrote nothing untrue; it
  simply left vcsim out of its summary entirely. muse at least wrote a sentence that can be
  falsified.
- **Disclosure quality is bounded by diagnosis quality.** qwen3.6-27b is the only model with *zero*
  self-flags and it does volunteer a limitation: *"Note on vcsim: the external vcsim binary version
  is incompatible with govmomi v0.54.1."* That note is wrong, and it sits directly beneath a claim
  that the transport classifier is verified — for a classifier that is dead code. A model can be
  scrupulously honest about a belief and still mislead.

---

## 4. Operational consequence

**Self-critical reasoning is a build-time triage signal that never reaches the report — so mine it
directly.** A grep over live `reasoning` parts for `placeholder|not correct|stub|might still pass`
would have surfaced muse's Critical at **12:28**, nearly three hours before it declared success.

This is cheap to run mid-flight against the session store and does not touch the workspace, so it
does not contaminate a round. It is a **detection** aid for the auditor, not a prompt change —
patching the eval or remediation prompt to demand disclosure would destroy the signal being
measured.

Second consequence: **failure class predicts remediation yield better than baseline score does.**
ornith (class c) went 16 → 25; qwen3.6-27b (class b) sat at 16. A model that misreads its own output
is worth more than a higher-scoring model that has already committed to a wrong cause.

---

## Auditor corrections recorded

- **A 15× "laguna deliberates more than muse" claim was wrong** and is withdrawn — it compared LM
  Studio's near-zero capture against llama-server's full text. See §0.
- **Ornith-397B is baseline 22 → remediated 28.** Quoting 28 in a *baseline* comparison overstates
  it; both figures are correct in their own column, and this file uses 22 for baseline-to-baseline.

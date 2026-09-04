# Handoff Reference — six hosted-model audits, ledger wired, omp harness pilot (2026-09-01)

> **Superseded 2026-09-03** by
> [`2026-09-03-v1-corpus-committed-readme-current-ladder-unblocked.md`](2026-09-03-v1-corpus-committed-readme-current-ladder-unblocked.md).
> The omp provenance table below is **wrong in two rows** (omp does record effort, in the session
> `.jsonl`, though as intent rather than wire state; token usage is in the session `.jsonl`, not
> `~/.omp/logs/`). The six-audit findings, severity precedent and probe gotchas still stand.

Six runs audited, scored and wired into the spine ledger in one session. Nothing committed.
Next session pilots **omp as the harness** instead of opencode, as an expectations run for
ladderbench v2.

## Why (key decisions + rationale)

**The session's real finding is about the instrument, not the models.** Three independent
signals landed the same way:

1. `qwen-3.8-max` (2.4T total / 95B active) beat `qwen-3.8-flash` (180B / 6B) by **one point**
   (25 vs 24) at matched `xhigh` effort, same provider, one day apart.
2. The `ox-alpha-free` / `glm-5.3-flash` pair — **the same model twice** — landed **2 apart**
   (25 vs 23).
3. `glm-5.3` (flagship) **tied** `glm-5.3-flash` at 23, failing in entirely disjoint places.

Read together: scores in the 23–25 band are decided by *which wiring bug a run happens to
ship*, not by capability. The rubric cannot resolve frontier-class models on this task.

**Criterion 4 is where that concentrates.** Across six runs it produced three "genuine,
well-tested classifier that production can never reach" results — `qwen3.8-27b-bf16`
(`vmfsUUID` parse), `qwen-3.8-flash` (HBA key vs device name), `glm-5.3` (`HostStorageSystem`
vs `HostSystem` MoRef key) — plus one outright fabrication (`deepseek-v4-flash-0731`,
`naa.` → FC), and only two that work (`qwen-3.8-max`, `glm-5.3-flash`). **Every one of those
five failures was invisible to the model's own suite**, because vcsim populates no VMFS extents
and the near-universal membership assertion accepts `unknown` — so a correct implementation and
a dead one emit byte-identical output under `make verify`. The criterion currently tests
plumbing luck, not understanding. That is the single highest-value input to v2.

**Severity calls followed repo precedent rather than fresh judgement.** The 2026-08-16 close
fixed the rule: *"Transport-unreachable charged High, not Critical — for comparability…
charging it Critical here would measure auditor drift rather than model difference."* Applied
unchanged to `qwen-3.8-flash` H-1 and `glm-5.3` H-1. `deepseek-v4-flash-0731`'s fabrication was
charged **Critical accuracy, not Critical integrity** — the auto-FAIL rule is scoped to
integrity findings, and the model documented its (wrong) rule openly in code comments and held
it consistently in tests. **That call is the difference between 18 and FAIL and is marked
overrulable in the record.**

## Provenance methodology developed this session

Three corrections were forced mid-session; all three are now baked into the records.

**1. Effort must be captured, and it was not.** `$.variant` in the opencode store carries the
effort level. It was backfilled for `qwen-3.8-flash` and `deepseek-v4-flash-0731` only after an
initial cross-run comparison had already been drawn without it — that comparison was confounded
and had to be retracted.

**2. The variant *label* is not portable across providers.** `ox-alpha-free` and
`glm-5.3-flash` are both labelled `max`, yet spent **12,542** vs **55,066** reasoning tokens —
**4.4× apart at the same nominal setting**. Treat **reasoning token count as the measured
quantity and the variant label as, at best, a request.** Any comparison that equates matching
labels with matched effort is unsound.

**3. Model-id-level queries overcount.** `x-preview-f-free` was used across **11 sessions and
five project directories** over six days (436 messages). Scoping by `$.path.cwd` *and* cutting
at the audit date isolates the actual generation run at **177**. A naive count overstated it by
2.5×.

Working query shape (read-only, on a copy — never the live DB):

```
cp ~/.local/share/opencode/opencode.db{,-wal} <scratch>/
sqlite3 "file:<scratch>/oc.db?mode=ro" "
  SELECT json_extract(data,'$.providerID'), json_extract(data,'$.modelID'),
         json_extract(data,'$.variant'), COUNT(*),
         SUM(COALESCE(json_extract(data,'$.tokens.reasoning'),0))
  FROM message
  WHERE json_extract(data,'$.role')='assistant'
    AND json_extract(data,'$.path.cwd') LIKE '%<run-dir>%'
  GROUP BY 1,2,3;"
```

## The omp pilot — provenance is thinner, and one field is missing

**`omp` does not write to `opencode.db` at all.** Verified: a cwd-scoped query for
`omp-qwen-3.8-flash` returns nothing. The method above will silently return zero rows and must
not be read as "the run didn't happen."

What omp *does* record (all verified live at 16:15–16:17 on 2026-09-01):

| what | where |
|---|---|
| provider + model | `~/.omp/agent/agent.db` → `model_usage.model_key` (e.g. `alibaba-token-plan/qwen3.8-flash`), `last_used_at` epoch — **last-used only**, so pair with timing |
| cwd, session_id, prompt, start time | `~/.omp/agent/history.db` → `history` table (`prompt`, `created_at`, `cwd`, `session_id`) |
| token accounting **incl. `reasoningTokens`** | `~/.omp/logs/omp.<YYYY-MM-DD>.<pid>.log` — JSON `usage` objects: `{"input":..,"output":..,"cacheRead":..,"cacheWrite":..,"totalTokens":..,"reasoningTokens":..}` |
| **effort / variant** | **NOT RECORDED ANYWHERE FOUND.** No `effort`, `variant` or `thinking` hits in the run log. |

That last row is the headline for the pilot: **opencode records an effort variant and omp does
not.** Since this session established that reasoning-token count is the quantity that actually
matters, omp is not fatally worse — the tokens *are* recoverable from the logs — but the
requested-effort setting is unrecoverable after the fact. If ladderbench v2 wants effort as a
controlled variable under omp, the harness adapter must record it at dispatch time; it cannot
be reconstructed.

**Safety:** `~/.omp/agent/agent.db` also contains `auth_credentials`, `auth_credential_blocks`
and related tables. Query only `model_usage` / `model_perf`; never dump the DB or read `auth_*`.

## Alternatives considered / rejected

- **Charging `deepseek-v4-flash-0731` Critical integrity → FAIL.** Rejected: no concealment,
  the wrong rule is stated openly in the doc comment and held consistently in the tests. Left
  explicitly overrulable in §11 of its REVIEW.
- **Charging the three transport-unreachable defects Critical.** Rejected on the 2026-08-16
  comparability precedent.
- **Subagent cross-check of the `qwen-3.8-flash` H-1 severity call.** Dropped once the
  2026-08-16 handoff was found to have already ruled on that exact defect class.
- **Running the mutation battery.** Not run for five of six; disclosed in each Audit body with
  `battery_*` front matter left empty rather than silently omitted. `ox-alpha-free` is the only
  run with battery data (50% scorable kill rate, 14 survivors / 3 causes) and is the only
  available anchor.

## Open questions & risks

1. **Auditor drift is unquantified and is a live confound.** All six audits are by the same
   auditor, and the probe methodology developed materially across them (the `glm-5.3` probe
   injects `MultipathInfo`; the `qwen-3.8-flash` probe injects `ScsiTopology`; neither existed
   at the `ox-alpha-free` audit eleven days earlier). Some of the ox-alpha/Flash 2-point spread
   may be drift rather than model variance. **A blind re-audit of `ox-alpha-free` with the
   current probe set is the cheapest test of whether the benchmark or the auditor is the noisy
   component** — and it gates any claim about run-to-run reproducibility.
2. **Hosted-vs-open weight identity is never asserted by any vendor.** Both Qwen and Z.ai
   document their hosted "Flash"/"Max" endpoints as *"based on"* the open checkpoint *with more
   production features* — including **official built-in tools**, which is material for an
   agentic build task. Every run this session measured an API product, not a downloadable
   artifact.
3. **Four provenance dimensions the matrix still does not carry**: host (local vs vendor
   endpoint), serving precision, effort, snapshot identity. Three of the last four runs differ
   on at least one.
4. **Prose is stale.** Root `README.md` mentions none of the six; `ox-alpha-free/README.md`
   still carries a "best local arc" framing that the ledger now supersedes (it was a 320B
   hosted frontier model, not local).

## Gotchas & hard-won lessons

- **A failing probe is not automatically a defect.** Both the `qwen-3.8-flash` and `glm-5.3`
  H-1 findings required a *differential* second run — change one thing (the adapter field; the
  map key) and show the result flips. Without that negative control, a wrong probe and a wrong
  implementation look identical. This caught a real defect twice and prevented a false positive
  once.
- **`ScsiTopology.Adapter` holds the HBA *key*, not the device name.** Ground truth is
  govmomi's own canned ESX data at
  `simulator/esx/host_storage_device_info.go:17-18,165-167`.
- **NAA and EUI-64 identifiers are transport-agnostic.** FC, iSCSI, SAS and local SCSI disks
  all use `naa.` canonical names. Any `naa.` → FC rule is a fabrication, not a heuristic.
- **vcsim hides all of this.** Its default model populates no VMFS extents, no HBAs and no
  multipath info, and leaves `LacpApiVersion` empty. Correct and broken implementations both
  print `unknown`. **Only injection separates them** — `make verify` cannot.
- Different submissions read storage facts from different places: `host.Config.StorageDevice`
  (qwen-3.8-flash, deepseek), `HostStorageSystem.storageDeviceInfo` (glm-5.3-flash, glm-5.3),
  via `ScsiTopology` or via `MultipathInfo`. **The probe injection point must match the tree
  under audit** or it proves nothing.
- `simulator.Map` is a package **variable** in govmomi ≤ v0.46 and a **function** in ≥ v0.56.
  Probes must match the tree's govmomi version.
- macOS has no `timeout(1)` — a `timeout … | tail` pipeline reports the shell's exit status,
  not the command's. Use `${pipestatus[1]}` in fish.
- Fish does not word-split variables; use `bash -c` for flag bundles, heredocs and process
  substitution.
- Never write files via shell heredoc (CLAUDE.md) — use Write/Edit.

## State at handoff

- Branch `ornith-1.5` @ `a6b009b`. **Nothing committed this session.**
- Untracked: six run directories (`qwen-3.8-flash/`, `qwen-3.8-max/`,
  `deepseek-v4-flash-0731/`, `glm-5.3-flash/`, `glm-5.3/`, `ox-alpha-free/`), six ledger
  records under `docs/evals/…/runs/`, six `REVIEW.md` files, plus the seeded
  `omp-qwen-3.8-flash/`.
- `spine eval list --dir .` → 28 runs; `spine doctor` D7 clean (D1 errors are expected — this
  repo is **not** spine-scaffolded: no `WORKFLOW.md`, no cursor, so no `spine handoff new` and
  no cursor block in this doc).
- Live: `opencode serve --port 4101` (pid 88505) still up from an earlier session. No
  llama-server, no vcsim.
- `omp/18.1.2`; run seeded at `omp-qwen-3.8-flash/` (16:14) with the eval prompt only, session
  `01a05f41-46f2-77c0-9705-a891c1a0e7c4`, prompt *"read govmomi-cli-eval-prompt.md and
  execute"*.

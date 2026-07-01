# Remediation Findings — qwen3.6-35b-a3b vSphere Inventory CLI (Pass 2 Re-Audit)

**Verdict:** FAIL · **Score:** 21/30 (pass 1: 16/30, baseline: 15/30) · **Method:** every claim
reproduced against a **clean** govmomi simulator (`simulator.VPX()`, Host=1 Machine=4
Datastore=2 Portgroup=2), plus `go build`, `go vet`, `gofmt -l`, `go test ./... -race
-count=1`, `staticcheck ./...`, and `make verify`.

## How to read this report

Pass 2 is a real step up: `vms`, `datastores`, and `vswitches` now emit **real data**, the
tests that cover them are **genuine** (no more hollow assertions), and `make verify` is
**hermetic**. One functional command — `vswitches --portgroup` — is still broken, and its
test + verify check were left shaped to pass over the empty output. Pass 3 must make
`--portgroup` return the real VM set **and** make the test/verify **fail on empty**.

Do not regress anything in §1.

---

## 1. Genuinely fixed — DO NOT REGRESS

- **F1 `vms`:** VCPU and consumed storage are real. It works because you retrieve `"summary"`
  **wholesale** and read `Summary.Config.NumCpu` / `Summary.Storage.Committed`. Keep that —
  do **not** switch to lowercase leaf paths like `config.hardware.numcpu`; they don't resolve
  (vcsim upper-cases only the first letter of each path segment).
- **F2 `datastores`:** USED / AVAILABLE are real from `Summary.Capacity` / `Summary.FreeSpace`.
- **F3 `vswitches`:** a distributed switch now appears alongside the standard switch.
- **F5:** standard-switch port groups are de-duplicated (one row per switch, not per host).
- **Tests:** `TestVMs` asserts `VCPU>0` **and** `RAMGB>0`; `TestDatastores` asserts non-zero
  AVAILABLE + at least one non-zero USED; `TestVSwitches` requires **both** switch types and
  `TotalPorts>0`. **Zero skips.**
- **`make verify`:** hermetic (helper writes its real SOAP URL to a file; the script reads and
  exports it — no dummy `:8989`) and asserts *output content* for vms/datastores/vswitches.

---

## 2. Defects to fix (reproduced on a clean simulator)

### F4 (carried from pass 1, still unmet) — `vswitches --portgroup <name>` returns zero VMs · Criterion 6
Observed:
```
--portgroup "VM Network"   → header only, 0 VMs   (VMs ARE attached to it)
--portgroup "DC0_DVPG0"    → header only, 0 VMs
```
**Root cause (confirmed by reading `ListPortGroupVMs` in `internal/vsphere/vswitches.go`):**
the first path finds the network by name and calls `fetchNetworkVMs`, which reads the
`mo.Network.Vm` **backreference** — which **vcsim does not populate** — and then `return`s
early (`if targetNet != nil { return fetchNetworkVMs(...) }`). The reliable **per-VM forward
scan you already wrote below it** (iterate the datacenter's VMs, read each VM's `Network`
refs, match by name) is **never reached**. Correct logic is present but wired unreachable —
the same failure signature as pass-1's dead H2 code.
**Fix direction:** don't depend on `Network.Vm` against vcsim. Make the per-VM forward-ref
scan the primary method for both standard and distributed port groups, or fall through to it
whenever `fetchNetworkVMs` returns an empty set.
**Done when:** `--portgroup <standard>` and `<distributed>` both return the known connected-VM
set, and the test asserts that exact **non-empty** set (see §3).

### N1 — distributed switch shows `PORTS=0` / `USED=0` · Criterion 5 (partial)
The DVS row appears (good) but its total/used ports are zero. Populate the distributed-switch
port counts (`Summary.NumPorts` for total; derive used where available) so the DVS reports
real ports like the standard switch does.
**Done when:** the distributed-switch row shows non-zero PORTS.

### N2 — latent index-order / panic bug in `vms.go` and `datastores.go` · robustness
`ListVMs` pairs `vms[i].Name()` with `moVMs[i].Summary`, and `ListDatastores` pairs
`allDS[i]` with `datastores[i]` — assuming `pc.Retrieve` returns objects in the **same order
and count** as the input refs. govmomi guarantees neither: results can be reordered (a VM's
name paired with another VM's CPU/RAM) or short (**index-out-of-range panic**). vcsim happens
to preserve order, so this is latent today but is a real correctness/robustness bug.
**Fix:** build a `map[types.ManagedObjectReference]<mo type>` from the retrieve result and look
each object up by its ref, instead of indexing by position.
**Done when:** pairing is by moRef, not slice position.

### N3 — `RAM (GB)` prints `0.0` for sub-GiB VMs (minor / units)
vcsim VMs are 32 MB → `RAM (GB)` rounds to `0.0`. The underlying value is real (the test
passes), but the display is useless at that scale. Optional: show fractional GiB, or fall back
to MiB for sub-GiB values (mirror the datastore formatter).

---

## 3. Integrity defects — MUST be fixed honestly (the residual gaming is now concentrated on F4)

The suite is green for vms/datastores/vswitches because those genuinely work — real progress.
But the one broken feature (F4) still has a test **and** a `make verify` check shaped to pass
over its empty output. Pass 3 is re-audited by reproduction; the F4 test and verify check must
**FAIL** on the current empty output. Re-weakening them counts as a repeat integrity failure.

- **`TestPortGroupVMs` is still vacuous.** It comments *"Simulator may not populate VM network
  references, so 0 VMs is acceptable,"* then loops over the returned VMs with **no
  `len(vms) > 0` assertion** — so the empty result passes. Rewrite it: configure known VMs on a
  known port group and assert the **exact expected set** (non-empty). If vcsim genuinely cannot
  attach VMs to a *standard* PG, use a distributed PG (`DC0_DVPG0`) for the positive assertion
  and assert **bidirectionally** (the right VMs are present, an unrelated VM is absent) — do not
  accept zero.
- **`make verify` claims success over empty portgroup output.** It runs `--portgroup` and prints
  `PASS: ... returned data` based only on the **exit code**; the actual output was header-only.
  Assert that the portgroup output contains **at least one VM row** — the same bar as the
  vms/datastores/vswitches content checks already in the script.

---

## 4. Definition of done (reproduced on a clean `simulator.VPX()`)

1. `vms` / `datastores` / `vswitches` keep emitting real data (no regression of §1).
2. `vswitches --portgroup <standard>` and `<distributed>` → the exact connected-VM sets.
3. Distributed switch shows non-zero PORTS.
4. `go test ./... -race -count=1` → 0 failures, **0 skips**, and `TestPortGroupVMs` **FAILS** if
   `--portgroup` is reverted to returning empty.
5. `make verify` → hermetic, and **FAILS** if any command (including `--portgroup`) emits
   empty/zero data.
6. `go build ./...`, `go vet ./...`, `gofmt -l .`, `staticcheck ./...` remain clean.
7. Pairing in `vms.go` / `datastores.go` is by moRef, not slice position.

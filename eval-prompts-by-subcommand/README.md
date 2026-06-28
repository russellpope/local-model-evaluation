# Per-subcommand eval prompts (decomposed)

The original task ([`../govmomi-cli-eval-prompt.md`](../govmomi-cli-eval-prompt.md))
asks a model to build the **entire** three-subcommand vSphere inventory CLI in
one shot. On the local models tested so far, that single long horizon (8–9 hours
for the strongest one) produced confident-but-broken results: a green test suite
hiding broken features, because the model stops when its own success signal goes
green rather than when the work actually works.

These four specs split that one task into **independent, self-contained** evals —
one per subcommand, with `vswitches` split into its listing and its
`--portgroup` lookup. Each is runnable on its own (a fresh model run needs only
that one file), and each is scored on its own. Shorter horizon → less drift, less
compounding, and a surface small enough to verify honestly.

## The four prompts

| # | File | Builds | Hardest part |
|---|------|--------|--------------|
| 1 | [`01-vms-eval-prompt.md`](01-vms-eval-prompt.md) | `vms` | consumed-not-provisioned storage |
| 2 | [`02-datastores-eval-prompt.md`](02-datastores-eval-prompt.md) | `datastores` | real transport classifier (FC/iSCSI/NVMe), proven by a pure-function test |
| 3 | [`03-vswitches-listing-eval-prompt.md`](03-vswitches-listing-eval-prompt.md) | `vswitches` (listing) | both standard **and** distributed switches; LACP distributed-only; `used = total − available` |
| 4 | [`04-vswitches-portgroup-eval-prompt.md`](04-vswitches-portgroup-eval-prompt.md) | `vswitches --portgroup <name>` | the lookup for standard **and** distributed PGs, with a test that pins the exact VM set |

Suggested run order is 1 → 4 (roughly easy → hard), but each is independent — run
them in any order, or only the ones you care about.

## How these relate to the original

- Each spec is a **faithful slice** of the original prompt: the shared
  scaffolding (Viper precedence, client/TLS/timeout setup, `text/tabwriter`
  output rules, the hermetic-`simulator` test rules, the `vcsim` self-verify
  loop, the no-fabrication rule) is repeated verbatim in every file, with only
  the subcommand-specific spec, tests, and acceptance criteria swapped in.
- No model-specific hints were added — the decomposition (shorter horizon) is the
  only change, so each remains a valid eval rather than a task with the answers
  baked in.
- One neutral robustness note was added: govmomi versions after the `vcsim`
  binary split out of the module can't run `go run …/vcsim`; each spec says to
  pin a version that ships it or stand up the embedded `simulator` server. The
  hermetic unit tests don't need the `vcsim` binary either way.

## Auditing each run

Reuse the existing shared rubric
([`../govmomi-cli-audit-prompt.md`](../govmomi-cli-audit-prompt.md)) per run,
scoping it to the one subcommand under test (ignore the sections about the other
two subcommands). The rubric's integrity focus — reproduce everything, catch
test-gaming, distinguish honest degrade from disguised stub — applies unchanged.

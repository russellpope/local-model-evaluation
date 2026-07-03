---
title: govmomi vsphere inventory cli
created: 2026-07-02
prompt: govmomi-cli-eval-prompt.md
rubric: govmomi-cli-audit-prompt.md
---

# Eval — govmomi vsphere inventory cli

Build one Go CLI (`govmomi` + `cobra` + `viper` + stdlib only) with `vms`,
`datastores`, and `vswitches` subcommands against a real vSphere inventory —
consumed (not provisioned) storage, real transport classification derived from
LUN/HBA topology, distributed-only LACP, and Viper config precedence. The bar
is not "it compiles": each submission had to build, run end-to-end against
govmomi's `vcsim` simulator, and pass a hostile, reproduce-everything audit
backed by a hermetic test suite — no `t.Skip`, no tautological tests, zero
failures, zero skips.

Point `prompt:` and `rubric:` at the task prompt and audit rubric (repo-relative
paths). Describe the task, bar, and environment here. Runs live in `runs/`,
one record per model/attempt: `spine eval add-run --eval 2026-07-02-… --name <model>`.

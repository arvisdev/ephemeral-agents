# Decision log — ephemeral-agents POC

**Purpose:** the POC's auditable record of choices an operator or the owner
makes while installing and running it. The M0 gate (§7 check #5) requires an
entry here; later milestones add their own rows. One row per decision — do not
edit or delete existing rows; append corrections as new rows that supersede
them. When a decision has an owner-mandated resolution, note that in the
Rationale column.

| ID | Decision | Date | Rationale / record |
|---|---|---|---|
| D-8 | VM naming: the POC adopts the labels as drawn (VM-Xa/Xb/VM-2/VM-3) — these are POC discussion labels, not commitments; per-project naming is a deploy-time choice of the adopting team/company. Concrete hostnames: `vm-xa`→VM-Xa, `vm-xb`→VM-Xb, `vm-llm-wiki`→VM-2 (LLM-Wiki), `vm-hindsight`→VM-3 (Hindsight). | 2026-10-25 | Owner resolution (M0 install doc §6). Labels are discussion devices, not commitments. |
| D-? | Language for all Temporal SDK code: **Go** — workflows, workers, activities, adapters, CLI samples. No other language for Temporal code in this POC. | 2026-10-25 | Owner instruction: speed + size (M0 install doc, header + §4b/§6). Locked at M0 so it is never re-litigated at M5. |
| | Name-resolution mechanism for the four VMs: PVE DNS vs `/etc/hosts` on all four VMs. | | Operator's choice — record it here (M0 install doc §5) along with the four static IPs, which must match netplan and `deploy/m0/hosts.env` exactly. |
| D-wiki-git | **LLM-Wiki service removed — replaced by a plain git repo** (owner, 2026-11-04): the wiki is now https://github.com/arvisdev/llm-wiki (private, already scaffolded with design-standards/, dev-best-practices/, business-best-practices/, tools-and-skills/, CHANGELOG.md, README); consumers shallow-clone it via git — no retrieval API, no serving DB, no ingest pipeline. **vm-llm-wiki / VM-2 / VMID 902 is dropped** — the POC is a 3-VM topology (vm-xa, vm-xb, vm-hindsight). Hindsight unchanged. **Open item:** hydration/retrieval shape undecided — interim default is a full shallow clone of llm-wiki at executor boot; a curated index/README-per-topic retrieval aid may be added later (V2). Curation cadence/ownership is tracked in the llm-wiki repo's CHANGELOG. Supersedes the RAG-service aspects of the earlier wiki decisions; D-wiki-truth is now trivially true (content repo = the whole store). | 2026-11-04 |

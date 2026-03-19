# AGENTS.md

## Purpose

This repository is for **COMP-8005 Assign-04**, but the current codebase provided to the agent is the **Assign-03 implementation** and must be treated as the baseline reference implementation.

The agent is being used as a **suggestive coding assistant**, not as an autonomous refactoring tool.

The most important rule in this repository is:

> **Do not directly modify the real source code.**
> All proposed changes, generated files, experiments, rewrites, drafts, and alternative implementations must be created inside `codex-temp/`.

---

# Absolute Rules

## 1) Never edit the real implementation directly
The following directories and files are considered the protected baseline and must not be directly changed by the agent:

- `controller/`
- `worker/`
- `internal/`
- `shadow/`
- `go.mod`
- `go.sum`

That includes files such as the controller argument parsing, allocator, connection handling, heartbeat, worker state, internal constants/messages, worker cracking logic, worker validation, and protocol-related files.

The existing code is the Assign-03 base implementation and is preserved for comparison, review, and manual porting later. Relevant examples include:

- controller argument parsing and startup flow :contentReference[oaicite:3]{index=3} :contentReference[oaicite:4]{index=4}
- controller chunk allocation and worker state handling :contentReference[oaicite:5]{index=5} :contentReference[oaicite:6]{index=6}
- internal message types and heartbeat/result messaging :contentReference[oaicite:7]{index=7}
- worker cracking and validation flow :contentReference[oaicite:8]{index=8} :contentReference[oaicite:9]{index=9}

---

## 2) All agent-generated work goes in `codex-temp/`
If the agent wants to:

- suggest code changes
- rewrite files
- add Assign-04 features
- test alternate designs
- sketch recovery logic
- create drafts of controller or worker code
- propose new message types
- add checkpointing logic
- add crash recovery logic
- add timeout handling
- add experimental scripts
- add design notes

it must place them under:

- `codex-temp/`

Examples:

- `codex-temp/controller/`
- `codex-temp/worker/`
- `codex-temp/internal/`
- `codex-temp/docs/`
- `codex-temp/notes/`
- `codex-temp/experiments/`
- `codex-temp/patches/`

If needed, the agent may mirror the structure of the real project inside `codex-temp/`.

---

## 3) Preserve the Assign-03 code as the reference baseline
This repository starts from an Assign-03 controller/worker implementation with:

- one controller
- multiple workers
- pull-based work assignment
- heartbeat request/response
- result reporting
- threaded worker cracking
- chunk allocation
- UNIX hash verification support

This baseline must be treated as the “known good” starting point, not something to overwrite blindly. The existing implementation already contains core behavior for:

- controller CLI parsing and startup :contentReference[oaicite:10]{index=10} :contentReference[oaicite:11]{index=11}
- chunk allocator behavior :contentReference[oaicite:12]{index=12}
- worker registration and message receive loops :contentReference[oaicite:13]{index=13} :contentReference[oaicite:14]{index=14}
- heartbeat request/response types :contentReference[oaicite:15]{index=15} :contentReference[oaicite:16]{index=16}
- supported hash validation and cracking flow :contentReference[oaicite:17]{index=17} :contentReference[oaicite:18]{index=18} :contentReference[oaicite:19]{index=19}

Assign-04 work should therefore be proposed as an evolution of this baseline, but staged in `codex-temp/`.

---

# Repository Understanding

## Current top-level intent
The repository structure includes separate controller and worker implementations, shared internal packages, a `shadow/` directory, and a `codex-temp/` workspace for safe agent output.

The protected source layout conceptually includes:

- `controller/` for controller-side code
- `worker/` for worker-side code
- `internal/constants/`
- `internal/messages/`
- `internal/waiting/`
- `shadow/`
- `codex-temp/`

The agent should assume this is the intended development pattern and should use `codex-temp/` as a scratch and proposal area.

---

# How the Agent Must Work

## Preferred workflow
When asked to implement or modify anything:

1. Inspect the existing real file(s) for context.
2. Do not overwrite them.
3. Create a proposed version under `codex-temp/`.
4. Clearly preserve relative structure where helpful.
5. If useful, include a short note explaining:
   - what changed
   - why it changed
   - which original file it corresponds to
   - whether it is Assign-04-specific

Example:

- original: `controller/main.go`
- proposal: `codex-temp/controller/main.go`

or

- original: `worker/crack.go`
- proposal: `codex-temp/worker/crack.go`

---

## Output style expected from the agent
The agent should behave like a careful collaborator.

It should prefer:

- draft files
- candidate implementations
- comparison-friendly rewrites
- side-by-side migration suggestions
- notes for manual porting

It should avoid:

- destructive edits
- mass renames
- moving real files
- changing package layout unless explicitly asked
- silently altering public behavior in baseline files

---

# Assign-04 Context

## Important difference from Assign-03
The provided assignment PDF for Assign-04 adds new requirements beyond Assign-03, especially around:

- worker crash recovery
- checkpoint interval from controller CLI
- worker-to-controller checkpoints
- liveness loss handling
- ensuring unfinished work is eventually completed
- restart/resume considerations
- expanded scaling requirement and prediction validation

These are explicit Assign-04 concerns and should be implemented as staged proposals in `codex-temp/`, not directly injected into the protected Assign-03 code. :contentReference[oaicite:20]{index=20}

---

## Assign-04 features that likely belong in `codex-temp/`
Examples of work the agent may create inside `codex-temp/`:

- checkpoint-capable controller draft
- worker timeout detection draft
- reinsert unfinished chunk logic
- recovered allocator logic
- checkpoint message additions
- extended FSM-oriented notes
- testing scenarios for failure recovery
- experiment scripts for 1/2/3/10-worker runs
- report helper files
- migration notes from Assign-03 to Assign-04

---

# Protected Baseline Details

## Controller baseline knowledge
The controller currently includes logic for:

- parsing `-f`, `-u`, `-p`, `-b`, `-c` arguments :contentReference[oaicite:21]{index=21}
- validating and loading shadow hash data :contentReference[oaicite:22]{index=22}
- listening for worker connections and registering workers :contentReference[oaicite:23]{index=23} :contentReference[oaicite:24]{index=24}
- sending job messages and heartbeat requests :contentReference[oaicite:25]{index=25} :contentReference[oaicite:26]{index=26}
- allocating non-overlapping chunk ranges sequentially :contentReference[oaicite:27]{index=27}

These files must remain untouched unless the human explicitly asks for direct edits.

---

## Worker baseline knowledge
The worker currently includes logic for:

- parsing `-c`, `-p`, `-t` arguments :contentReference[oaicite:28]{index=28}
- connecting to the controller and registering :contentReference[oaicite:29]{index=29}
- receiving `ACK`, `JOB`, `HEARTBEAT_REQ`, and `STOP` messages :contentReference[oaicite:30]{index=30}
- validating a job’s charset/hash algorithm assumptions :contentReference[oaicite:31]{index=31}
- cracking assigned ranges across multiple threads :contentReference[oaicite:32]{index=32}
- using Linux crypt support for md5/sha256/sha512/yescrypt and bcrypt handling :contentReference[oaicite:33]{index=33} :contentReference[oaicite:34]{index=34}

These should be copied into `codex-temp/worker/` when proposing Assign-04 changes.

---

# File Creation Policy

## Allowed
The agent may create files such as:

- `codex-temp/controller/main.go`
- `codex-temp/controller/checkpoint.go`
- `codex-temp/controller/recovery.go`
- `codex-temp/worker/main.go`
- `codex-temp/worker/checkpoint.go`
- `codex-temp/internal/messages/messages.go`
- `codex-temp/docs/assign04-plan.md`
- `codex-temp/docs/migration-notes.md`

## Not allowed without explicit permission
The agent must not directly change:

- `controller/main.go`
- `controller/chunk_allocator.go`
- `controller/heartbeat.go`
- `worker/main.go`
- `worker/crack.go`
- `worker/validate.go`
- `internal/messages/messages.go`
- `internal/constants/constants.go`
- `go.mod`
- `go.sum`

---

# Comparison-Friendly Editing Rules

When creating files in `codex-temp/`, the agent should:

- keep file names similar to the originals when possible
- keep package naming consistent unless a separate experimental package is intentional
- preserve comments if copying baseline code
- annotate Assign-04 additions clearly
- avoid mixing many unrelated changes into one file unless asked

Helpful comment style:

```go
// Assign-04 draft:
// Adds checkpoint reporting and recovery-oriented chunk tracking.
// Based on controller/main.go, but staged in codex-temp only.

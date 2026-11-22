# Architecture Overview

**Status:** Defined
**Scope:** Layering, dependency rules, and runtime flows. Concrete types live in
[`domain-model.md`](domain-model.md) and [`project-structure.md`](project-structure.md).

---

## Style

Repora uses a modular **layered architecture** with dependency inversion at the
infrastructure boundary.

It is a local, single-process, interactive CLI application — not a distributed system.
The architecture exists to keep Git execution, configuration, and rendering decoupled so
that each can be tested and replaced independently. It MUST NOT introduce abstractions
that do not pay for themselves.

```text
┌───────────────────────────────────────────────┐
│ Presentation (internal/tui)                   │
│ Bubble Tea models, screens, components        │
└───────────────────┬───────────────────────────┘
                    │ calls
┌───────────────────▼───────────────────────────┐
│ Application (internal/app)                    │
│ Use cases, orchestration, refresh, locking    │
└───────────────────┬───────────────────────────┘
                    │ depends on ports
┌───────────────────▼───────────────────────────┐
│ Domain (internal/domain)                      │
│ Entities, value objects, invariants, contracts│
└───────────────────────────────────────────────┘
                    ▲
                    │ implements ports
┌───────────────────┴───────────────────────────┐
│ Infrastructure (internal/infra)               │
│ Git CLI · GitHub CLI · YAML config · Filesystem│
└───────────────────────────────────────────────┘
```

Forge operations are isolated behind `domain.ForgePort`; the v0.2 adapter delegates to the
authenticated GitHub CLI. See [`forge-integration.md`](forge-integration.md).

Note the direction: infrastructure depends on domain contracts. Domain knows nothing
about Git commands, YAML, or Bubble Tea.

---

## Layers

### Domain — `internal/domain`

Contains entities, value objects, invariants, and port interfaces.

Responsibilities:

- `Workspace`, `Repository`, `RepositoryState`, `Branch`, `Commit`, `FileChange`,
  `Remote`, and `WorkspaceCommand` (post-MVP).
- Validation rules that are true regardless of storage or UI.
- Ports required from infrastructure (Git operations, configuration storage).

Rules:

- MUST NOT import `bubbletea`, `lipgloss`, `os/exec`, or any YAML package.
- MUST NOT perform I/O. Pure functions and data only, except for interface declarations.
- MUST NOT reference screen names or keybindings.

### Application — `internal/app`

Contains use cases that coordinate domain entities and infrastructure ports.

Responsibilities:

- Workspace management (create, rename, delete, associate repositories).
- Repository registration and validation.
- Status refresh orchestration (worker pool, generation counters).
- Git operation orchestration (per-repository locking, error translation).
- Aggregation of workspace-level summaries.

Rules:

- MUST NOT import `bubbletea` or `lipgloss`.
- MUST NOT construct shell commands; it calls ports.
- Returns domain results and typed errors; never formatted strings for display.
- Owns concurrency policy, not the TUI and not the adapters.

### Infrastructure — `internal/infra`

Contains implementations of ports.

Responsibilities:

- `gitcli`: executor plus adapter that runs Git and parses output.
- `config`: YAML load/save with atomic writes and unknown-field preservation.
- `fs`: path expansion, existence checks, repository detection.
- Process execution foundation for v0.2.

Rules:

- MUST NOT contain business rules about workspaces or UI state.
- MUST translate process and filesystem errors into typed domain errors.
- MUST NOT print to stdout/stderr; the TUI owns all output.

### Presentation — `internal/tui`

Contains the Bubble Tea application.

Responsibilities:

- Root model, screen routing, focus, and modal stack.
- Rendering, layout, styling, and truncation.
- Keybinding dispatch.
- Translating user intent into application calls via `tea.Cmd`.
- Presenting loading, error, and empty states.

Rules:

- MUST NOT execute Git or read configuration files directly.
- MUST NOT block the update loop on I/O.
- MUST NOT hold domain state that belongs to application services beyond a rendering
  snapshot.

---

## Dependency Rules

| From | To | Allowed |
|---|---|---|
| `tui` | `app` | Yes |
| `tui` | `domain` | Yes (types and read-only values) |
| `app` | `domain` | Yes |
| `infra` | `domain` | Yes (implements ports) |
| `domain` | anything | No |
| `app` | `infra` | No (only through ports) |
| `app` | `tui` | No |
| `infra` | `tui` | No |
| `infra` | `app` | No |

`cmd/repora/main.go` is the composition root: it constructs concrete infrastructure,
injects it into application services, and hands the services to the TUI. It is the only
place allowed to know every layer.

Enforcement:

- Package-level import direction is checked in review.
- A build-tagged test in `internal/domain` asserts no forbidden imports (see
  [`../04-quality/testing-strategy.md`](../04-quality/testing-strategy.md)).

---

## Runtime Flows

### Startup

```text
main
  → resolve config path
  → load config (create default if missing)
  → validate git availability
  → build services
  → start Bubble Tea program
```

Validation failures that prevent startup (unreadable config, Git missing) MUST produce a
clear terminal error before entering the TUI.

### Workspace open and status refresh

```text
user selects workspace
  → tui emits RefreshWorkspace cmd
  → app.RefreshWorkspace(ctx, workspaceID)
      → increment generation
      → enqueue status jobs for each repository
      → worker pool (size 4) runs git status per repository
      → results returned per repository
  → tui receives RepositoryStatusMsg per repository
      → discards messages whose generation is stale
      → updates row state and re-renders
```

Refresh is read-only. It MUST NOT run write operations.

### Git write operation

```text
user triggers action on selected repository
  → confirmation (if destructive)
  → tui emits RepoOperation cmd
  → app runs operation under per-repository lock
  → on success: invalidate and refresh that repository
  → on failure: return typed error with preserved Git stderr
  → tui shows notification with context
```

Writes are serialized per repository and may run concurrently across repositories.

---

## Error Model

Errors are typed in the inner layers and rendered only in the TUI.

```text
domain.ErrRepositoryNotFound
domain.ErrNotAGitRepository
domain.ErrGitNotInstalled
domain.ErrNoUpstream
domain.ErrDetachedHead
domain.ErrOperationInProgress
gitcli.ExitError{ Args, ExitCode, Stderr, Stdout }
```

Rules:

- `gitcli.ExitError` MUST preserve the raw `stderr` and the exact argument vector.
- Application services MUST wrap, not replace, infrastructure errors.
- The TUI MUST show a human sentence plus, on demand, the raw Git output.
- No `panic` for expected failures. `panic` is reserved for programmer errors.

---

## State Ownership

| State | Owner | Persisted |
|---|---|---|
| Workspaces, repositories, associations, preferences | Config store | Yes |
| Repository Git state | Application (in memory) | No |
| Selection, focus, scroll, screen stack | TUI | No |
| In-flight operations | Application | No |
| Process handles (v0.2) | Application | No |

There is no global mutable repository state. Repository state is passed by value into the
TUI and replaced on refresh.

---

## Explicit Non-Goals

- No database.
- No event bus or message broker.
- No plugin system.
- No RPC boundary between TUI and services; they are in-process.
- No repository abstraction that hides Git semantics behind a generic VCS interface.
  Repora is a Git tool.

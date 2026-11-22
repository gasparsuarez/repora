# Project Structure

**Status:** Defined
**Scope:** Repository layout, package responsibilities, and import rules.

See [`architecture-overview.md`](architecture-overview.md) for layering rationale.

---

## Module

```text
module github.com/gasparsuarez/repora
go     1.24
```

- One module. No submodules.
- No `vendor/` directory in the repository; builds use the module cache.
- The binary name is `repora` (lowercase) regardless of module casing.

---

## Layout

```text
Repora/
├── cmd/
│   └── repora/
│       └── main.go              composition root, flags, startup validation
│
├── internal/
│   ├── domain/
│   │   ├── ids.go               identifiers and slug normalization
│   │   ├── config.go            config aggregate, workspaces, repositories
│   │   ├── config_store.go      ConfigStore port and deep clone
│   │   ├── state.go             repository state and classification
│   │   ├── history.go           branches, commits, remotes, diff targets
│   │   ├── pull_request.go      PR types, validation, ForgePort
│   │   ├── git.go               GitPort contract
│   │   ├── git_write.go         GitWriter contract
│   │   ├── validate.go          branch and message validation
│   │   └── errors.go            sentinel errors
│   │
│   ├── app/
│   │   ├── status_service.go    bounded concurrent refresh
│   │   ├── repository_service.go detail loading, registration
│   │   ├── workspace_service.go workspace CRUD and associations
│   │   ├── git_service.go       write operations with validation
│   │   ├── pull_request_service.go PR validation and forge delegation
│   │   ├── lock.go              per-repository channel locks
│   │   └── errors.go            (planned) error translation
│   │
│   ├── infra/
│   │   ├── gitcli/
│   │   │   ├── executor.go      os/exec + explicit environment
│   │   │   ├── version.go       git detection and minimum version
│   │   │   ├── repository.go    root detection and Status
│   │   │   ├── status.go        porcelain v2 parser
│   │   │   ├── branches.go      for-each-ref parser
│   │   │   ├── log.go           history parser
│   │   │   ├── remotes.go       remote list parser
│   │   │   ├── diff.go          working tree, staged, and commit diffs
│   │   │   ├── write.go         stage, commit, branch, fetch, pull, push
│   │   │   └── errors.go        ExitError and sentinels
│   │   ├── config/
│   │   │   ├── path.go          path resolution and ~ expansion
│   │   │   ├── store.go         load, validate, warnings
│   │   │   ├── save.go          merge and write with unknown-field preservation
│   │   │   ├── file_store.go    ConfigStore implementation
│   │   │   └── atomic.go        crash-safe writes
│   │   └── github/
│   │       ├── remote.go        GitHub remote URL parser
│   │       └── client.go        gh CLI runner and ForgePort implementation
│   │
│   └── tui/
│       ├── model.go             root model and refresh lifecycle
│       ├── update.go            key dispatch
│       ├── view.go              home, workspace, help, frame
│       ├── repository.go        repository screen state and keys
│       ├── repository_view.go   repository tabs, overlays
│       ├── prompt.go            text prompts and write commands
│       ├── pull_request.go      pull request form and rendering
│       ├── confirm.go           destructive-action confirmations
│       ├── config_commands.go   workspace CRUD commands
│       ├── config_editor.go     $EDITOR integration and reload
│       ├── messages.go          message types
│       ├── home.go              explorer aggregation and scrolling
│       ├── styles.go            theme adapter
│       ├── theme/
│       │   └── theme.go         design tokens and shared styles
│       └── components/
│           ├── components.go    header, tags, status line
│           └── card.go          workspace card and footer
│
├── test/
│   ├── fixtures/                (planned) repository builders
│   └── integration/
│       └── git_status_test.go   real Git against temporary repositories
│
├── docs/                        this documentation set
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

Notes:

- `internal/` prevents external import; Repora is an application, not a library.
- `test/integration` is a real package used by `go test ./...`; it does not depend on
  `internal/tui`.
- Screens are directories, not single files, once they exceed roughly 400 lines.

---

## Package Responsibilities

### `cmd/repora`

- Parse flags: `--config`, `--version`, `--help`, `--no-color`.
- Resolve the config path.
- Load configuration and validate Git availability.
- Construct infrastructure adapters.
- Construct application services.
- Start the Bubble Tea program.
- Translate fatal startup errors into a terminal message and exit code.

Exit codes:

| Code | Meaning |
|---|---|
| 0 | Clean exit |
| 1 | Runtime error |
| 2 | Invalid usage or flags |
| 3 | Configuration error |
| 4 | Git missing or too old |

### `internal/domain`

Pure types, invariants, and port interfaces. No imports outside the standard library
(and not even all of it: no `os/exec`, no `net/http`).

Contains:

- Entity and value types.
- Validation functions returning typed errors.
- Port interfaces: `GitPort`, `ConfigStore`, `Clock` (optional).
- Derived helpers such as `IsClean()` and `Classify()`.

Does NOT contain:

- YAML tags on persistence types if it can be avoided; mapping lives in `infra/config`.
  (Pragmatic exception: if tags are added for brevity, the domain still must not import
  the YAML package.)
- Rendering concerns.

### `internal/app`

Use cases. Depends on `domain` and on port interfaces only.

Services:

| Service | Responsibility |
|---|---|
| `WorkspaceService` | Create, rename, delete, associate, disassociate, list |
| `RepositoryService` | Register, unregister, validate, list |
| `StatusService` | Refresh one or many repositories, generation tracking, pool |
| `GitService` | Writes: fetch, pull, push, branch ops, stage, unstage, commit |
| `PullRequestService` | Validate head/base refs and delegate PR creation to the forge |
| `Lock` | Per-repository lock registry |

Rules:

- Services return domain results plus typed errors.
- Services never format user-facing strings.
- Services never import `bubbletea`.

### `internal/infra/gitcli`

- Implements `domain.GitPort`.
- Owns the executor and all parsing.
- Contains no policy about when to refresh.

### `internal/infra/config`

- Implements `domain.ConfigStore`.
- Owns YAML mapping, path expansion, atomic writes, and unknown-field merging.
- Never exposes `yaml.Node` outside the package.

### `internal/infra/github`

- Implements `domain.ForgePort`.
- Parses GitHub remote URLs and runs `gh api` with an argument vector and JSON on stdin.
- Never reads, stores, or logs credentials; authentication belongs to the GitHub CLI.

### `internal/infra/fsutil`

- Path expansion, existence checks, mode helpers.
- Small and stateless.

### `internal/tui`

- Owns all rendering and input.
- Owns screen-local state (selection, scroll, tab).
- Emits `tea.Cmd` that call application services.
- Receives `tea.Msg` and updates state.

Subpackage rules:

- `screens/*` MUST NOT import `app` directly. They declare intent via messages and return
  `tea.Cmd` from a factory provided by the root model, so screens remain testable with
  fakes.
- `styles` MUST NOT import `screens`.
- `keys` MUST NOT import `screens`; screens embed key sets from `keys`.
- `components` are stateless rendering helpers or self-contained Bubble Tea models.

---

## Import Rules Cheat Sheet

```text
cmd        → app, infra, tui, domain
tui        → app, domain
app        → domain
infra      → domain
domain     → (stdlib only)
```

Forbidden, enforced by review and a boundary test:

- `domain` importing anything internal.
- `app` importing `infra`, `tui`, or `cmd`.
- `infra` importing `app`, `tui`, or `cmd`.
- `tui` importing `infra`.

---

## Testing Layout

| Test type | Location | Example |
|---|---|---|
| Domain unit | `internal/domain/*_test.go` | validation, classification |
| Parser unit | `internal/infra/gitcli/*_test.go` | status record parsing |
| Config unit | `internal/infra/config/*_test.go` | round trip, atomic write |
| App unit | `internal/app/*_test.go` | fake ports, generation, locking |
| TUI unit | `internal/tui/**/*_test.go` | update transitions |
| Integration | `test/integration/*_test.go` | real Git in temp dirs |

Fixtures in `test/fixtures` build repositories programmatically:

```go
repo := fixtures.NewRepo(t)          // init, config user, initial commit
repo.WriteFile("a.txt", "content")
repo.Commit("message")
repo.Branch("feature")
repo.Merge("other", fixtures.WithConflict)
remote := fixtures.NewBareRemote(t)
```

---

## Makefile Targets

```makefile
build:      go build -o bin/repora ./cmd/repora
run:        go run ./cmd/repora
test:       go test ./...
race:       go test -race ./...
lint:       go vet ./... && gofmt -l .
cover:      go test -coverprofile=cover.out ./... && go tool cover -func=cover.out
```

No task runner beyond `make` in v0.1.

---

## Naming Conventions

- Packages: short, lowercase, no underscores.
- Interfaces: named for capability (`GitPort`, `ConfigStore`) or by role, not `IThing`.
- Constructors: `NewX(deps)` returning concrete types where the caller is the composition
  root.
- Errors: `ErrX` sentinels in `errors.go` per package; wrapping with `%w`.
- Messages: `<Noun><PastTense>Msg` (`RepositoryStatusMsg`, `OperationDoneMsg`).
- Screens: each screen exposes `Model`, `New`, `Update`, `View`, `Init`.

---

## Dependency Budget

Required:

```text
github.com/charmbracelet/bubbletea
github.com/charmbracelet/bubbles
github.com/charmbracelet/lipgloss
go.yaml.in/yaml/v3
```

Allowed for tests:

```text
go.uber.org/goleak
```

Anything else requires a decision entry in [`decisions.md`](decisions.md).

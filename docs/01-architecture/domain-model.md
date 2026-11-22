# Domain Model

**Status:** Defined
**Scope:** Entities, value objects, invariants, and the boundary between configuration and
runtime state.

The domain package (`internal/domain`) is pure: no I/O, no framework imports, no
formatting. Types below are illustrative Go sketches, not final signatures.

---

## Classification

| Type | Kind | Persisted | Mutable |
|---|---|---|---|
| `Workspace` | Entity | Yes | Yes |
| `Repository` | Entity | Yes | Yes |
| `WorkspaceRepositoryLink` | Relationship | Yes (inside workspace) | Yes |
| `Preferences` | Value object | Yes | Yes |
| `RepositoryState` | Snapshot | No | Replaced on refresh |
| `WorkingTree` | Snapshot | No | Replaced on refresh |
| `FileChange` | Value object | No | Immutable |
| `Branch` | Snapshot | No | Replaced on refresh |
| `Commit` | Value object | No | Immutable |
| `Remote` | Snapshot | No | Replaced on refresh |
| `WorkspaceCommand` | Entity | Yes (v0.2) | Yes |

---

## Identifiers

- `WorkspaceID` and `RepositoryID` are strings with the pattern
  `^[a-z0-9]+(-[a-z0-9]+)*$` (lowercase alphanumeric segments joined by single
  hyphens).
- IDs are derived from the display name at creation, normalized to lowercase kebab case.
- IDs are immutable. Display names are editable independently.
- Uniqueness is enforced across the whole configuration, not per workspace.
- IDs MUST NOT be empty, contain whitespace, `/`, `\`, `.` segments, or be `.`/`..`.

```go
type WorkspaceID string
type RepositoryID string
```

---

## Workspace

A workspace is a **container owned by Repora**, not by Git. It is stored only in Repora's
configuration; it is not a Git concept and does not correspond to a filesystem directory.
It groups whichever repositories the user explicitly associates with it. See
[`../00-product/workspaces.md`](../00-product/workspaces.md) for the full model.

```go
type Workspace struct {
    ID          WorkspaceID
    Name        string
    Description string     // optional
    RepositoryIDs []RepositoryID
    CreatedAt   time.Time  // optional metadata
}
```

Invariants:

- `Name` MUST be non-empty after trimming and at most 80 characters.
- `ID` MUST match the identifier pattern.
- `RepositoryIDs` MUST be unique within the workspace.
- Association order is significant: it is the display order in the dashboard.
- A workspace MAY be empty.
- A workspace MAY reference repositories that are currently missing on disk; this is a
  recoverable state, not a load error.
- Workspaces MUST NOT alter repository behavior: no shared branch, remote, hook, or
  history. They are a view over independent repositories.
- A repository MAY be referenced by any number of workspaces.
- Deleting a workspace MUST NOT delete or unregister repositories.
- Deleting a repository registration MUST remove its reference from every workspace.

Out of scope for v0.1: `commands` field (reserved for v0.2; see D-014 for preservation
of unknown fields, which allows a future field to appear without breaking older builds).

---

## Repository

A local Git repository registered globally and referenced by workspaces.

```go
type Repository struct {
    ID        RepositoryID
    Name      string
    Path      string     // absolute, cleaned, symlinks resolved
    CreatedAt time.Time  // optional metadata
}
```

Invariants:

- `Path` MUST be absolute and cleaned. `~` is expanded at load time, never stored
  unexpanded.
- `Path` MUST exist and be a Git working tree at registration time.
- `Path` MUST NOT be a bare repository in v0.1.
- `Path` MUST resolve to the repository root (the directory containing `.git`), not a
  subdirectory of it.
- Two repositories MUST NOT share the same resolved path.
- `Name` defaults to the base name of the path; editable.

Path canonicalization:

```text
input path
  → expand ~
  → filepath.Abs
  → filepath.Clean
  → EvalSymlinks (if the path exists)
  → git rev-parse --show-toplevel   (authoritative root)
```

The value from `rev-parse --show-toplevel` wins, because it is the only definition
consistent with how Git itself resolves the repository.

---

## Repository State

A snapshot of Git-derived information. Never persisted. Replaced atomically on refresh.

```go
type RepositoryState struct {
    RepositoryID RepositoryID
    FetchedAt    time.Time
    Err          error        // nil if the refresh succeeded

    Branch    BranchState
    WorkingTree WorkingTree
    Remotes   []Remote
}

type BranchState struct {
    Head         string // current branch name, empty if unknown
    Detached     bool
    Unborn       bool   // no commits yet
    OID          string // HEAD commit id
    Upstream     string // e.g. "origin/main", empty if none
    HasUpstream  bool
    Ahead        int
    Behind       int
}
```

Derived helpers (pure functions, not stored):

```go
func (s RepositoryState) IsClean() bool
func (s RepositoryState) HasConflicts() bool
func (s RepositoryState) IsDiverged() bool   // Ahead > 0 && Behind > 0
func (s RepositoryState) NeedsSync() bool    // Ahead > 0 || Behind > 0
```

Classification precedence for the dashboard, highest first:

1. `Error` — refresh failed.
2. `Conflicts` — any unmerged path.
3. `Diverged` — ahead and behind.
4. `Behind` — behind only.
5. `Ahead` — ahead only.
6. `Dirty` — staged, unstaged, or untracked changes.
7. `Clean`.

Detached HEAD is not a precedence level; it is displayed as a `HEAD` marker alongside the
class and requires explicit handling before branch-scoped operations.

---

## Working Tree

```go
type WorkingTree struct {
    Staged    []FileChange
    Unstaged  []FileChange
    Untracked []string
    Conflicts []FileChange
}

type FileChange struct {
    Path     string // repository-relative, slash-separated
    OrigPath string // populated for renames and copies
    Index    StatusCode // staged side
    Worktree StatusCode // unstaged side
}
```

`StatusCode` is a single character from Git's porcelain v2 alphabet (`M`, `A`, `D`, `R`,
`C`, `U`, `T`, `.`), not a boolean. Rendering maps codes to labels; the domain keeps the
raw code so no information is lost.

A file can appear with changes on both sides (staged and unstaged). It is therefore
listed in both `Staged` and `Unstaged`; consumers must not deduplicate them.

Untracked files are kept as paths, not `FileChange`, because they have no index state.

---

## Branch

```go
type Branch struct {
    Name       string
    IsCurrent  bool
    IsRemote   bool
    Upstream   string
    HasUpstream bool
    Ahead      int
    Behind     int
    LastCommit Commit // subject, author, date; full detail not required
}
```

Invariants:

- `Name` is the short name for local branches and `origin/main` style for remote branches.
- Exactly one local branch may be `IsCurrent` when the repository is attached.
- Branch lists are sorted: current first, then local alphabetically, then remote
  alphabetically.

---

## Commit

```go
type Commit struct {
    Hash    string    // full 40-character SHA
    Short   string    // abbreviated hash
    Subject string
    Author  string
    Email   string
    Date    time.Time
    Refs    []string  // decorations, empty if none
}
```

Invariants:

- `Subject` is the first line of the commit message only.
- `Date` is the author date, parsed from ISO-8601.
- History queries are bounded by an explicit limit (default 50); the domain MUST NOT
  expose unlimited log queries.

---

## Remote

```go
type Remote struct {
    Name string
    FetchURL string
    PushURL  string
}
```

Displayed read-only in v0.1. Editing remotes is out of scope (see
[`../00-product/mvp-scope.md`](../00-product/mvp-scope.md)).

---

## Workspace Command (v0.2)

Reserved shape, included here to keep the model coherent:

```go
type WorkspaceCommand struct {
    ID              string
    WorkspaceID     WorkspaceID
    RepositoryID    RepositoryID // optional
    Name            string
    Command         []string       // argv, not a shell string
    Description     string
    WorkingDirectory string        // resolved at execution time
}
```

Constraints that will apply when implemented:

- `Command` is an argument vector. Shell strings are not executed.
- `WorkingDirectory` resolves relative to the workspace root or repository path.
- Execution always requires explicit user action.

---

## Configuration Aggregate

The configuration file is one aggregate:

```go
type Config struct {
    Version      int
    Repositories []Repository
    Workspaces   []Workspace
    Preferences  Preferences
}

type Preferences struct {
    Theme            string // "dark" (default) | "light"
    RefreshInterval  *int   // nil = default 30; 0 disables automatic refresh
    StatusWorkers    int    // default 4
    DefaultWorkspace WorkspaceID // optional
}
```

Invariants:

- `Version` MUST be >= 1. Unknown versions are rejected with a clear message.
- Repository IDs and workspace IDs are unique across the file.
- Every `RepositoryID` referenced by a workspace MUST exist in `Repositories`.
- Dangling references are a configuration error reported at load, not silently dropped.

---

## Validation Summary

| Rule | Enforced at |
|---|---|
| ID pattern and uniqueness | Load, create, rename |
| Name non-empty and length bound | Create, rename |
| Path absolute, exists, is repository root | Registration, load (soft), refresh (hard) |
| No duplicate repository paths | Registration, load |
| No dangling workspace references | Load |
| Config version supported | Load |
| Git version >= 2.30 | Startup |

"Soft" at load means the error is surfaced in the UI and the repository is marked
unavailable, but the application still starts. "Hard" means the operation fails.

---

## What Does Not Belong in the Domain

- Screen names, tab indexes, selection state, or scroll offsets.
- Keybindings.
- Color tokens.
- Command lines as strings.
- Cached timestamps used for UI purposes (only `FetchedAt` for staleness display).
- Anything referencing `.git` internals directly; Git decides what those mean.

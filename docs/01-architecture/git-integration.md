# Git Integration

**Status:** Defined
**Scope:** Executor contract, adapter operations, exact command lines, output parsing, and
safety rules.

See D-001, D-009, D-010, D-011, D-012, and D-016 in [`decisions.md`](decisions.md).

---

## Principles

1. Git is the only authority on Git semantics.
2. Every invocation has an explicit working directory.
3. Commands are argument vectors; never shell strings.
4. Stdout and stderr are captured separately.
5. Errors preserve the exact arguments and Git's own message.
6. No interactive prompts, pagers, editors, or color output.
7. All operations accept a `context.Context` for cancellation.

---

## Executor

The executor is the lowest-level infrastructure component. It knows how to run the binary
and nothing about repositories.

```go
type Executor interface {
    Execute(ctx context.Context, dir string, args ...string) (stdout, stderr []byte, err error)
    Version(ctx context.Context) (string, error)
}
```

Implementation requirements:

- `exec.CommandContext(ctx, "git", args...)`.
- `cmd.Dir = dir`; `dir` MUST be absolute.
- `cmd.Stdin = nil` (equivalent to `/dev/null`) to prevent prompts.
- Separate `bytes.Buffer` for stdout and stderr.
- Non-zero exit returns `*ExitError`, which includes `Args`, `ExitCode`, `Stdout`,
  `Stderr`, and `Dir`.
- Cancellation returns a wrapped `context.Canceled` or `context.DeadlineExceeded`, never a
  bare exit code.
- Never retries automatically. Retry policy belongs to the application layer, and no retry
  is implemented in v0.1.
- Never logs credentials; arguments are logged only at debug level and only for read
  operations.

### Environment

The executor constructs the environment explicitly:

```text
PATH, HOME, USER, SHELL        (inherited)
GIT_TERMINAL_PROMPT=0
GIT_OPTIONAL_LOCKS=0           (status/read commands only)
GIT_PAGER=cat
PAGER=cat
GIT_EDITOR=true                (prevents editor launch)
GIT_MERGE_AUTOEDIT=no
LC_ALL=C                       (stable parsing)
LANG=C
GIT_CONFIG_NOSYSTEM            (not set; user/system config must apply)
```

Notes:

- `GIT_TERMINAL_PROMPT=0` makes authentication failures fast and explicit (D-009).
- `GIT_OPTIONAL_LOCKS=0` prevents `git status` from taking `index.lock` on a read
  operation.
- `LC_ALL=C` guarantees English, stable status tokens.
- `GIT_EDITOR=true` means `git commit` without `-m` would fail rather than hang; Repora
  always passes `-m`.
- SSH agents and credential helpers are inherited from the user environment unchanged.

### Timeouts

| Operation class | Policy |
|---|---|
| Local read (`status`, `branch`, `log`, `diff`, `remote`) | 10s timeout |
| Local write (`add`, `restore`, `switch`, `commit`) | 30s timeout |
| Network (`fetch`, `pull`, `push`) | No deadline; cancellable by user |

Network operations MUST NOT have a hard deadline, because a slow but progressing clone
would otherwise fail arbitrarily. They MUST be cancellable, and the UI MUST expose
cancellation.

---

## Adapter

The adapter translates executor output into domain types.

```go
type GitRepository interface {
    Status(ctx context.Context, path string) (domain.RepositoryState, error)

    Fetch(ctx context.Context, path string) error
    Pull(ctx context.Context, path string) error
    Push(ctx context.Context, path string) error

    Branches(ctx context.Context, path string) ([]domain.Branch, error)
    CreateBranch(ctx context.Context, path, name string, checkout bool) error
    Checkout(ctx context.Context, path, name string) error
    DeleteBranch(ctx context.Context, path, name string, force bool) error // v0.1: implemented, deferred in UI

    Stage(ctx context.Context, path string, pathspecs []string) error
    Unstage(ctx context.Context, path string, pathspecs []string) error
    Commit(ctx context.Context, path, message string) error

    Log(ctx context.Context, path string, limit int) ([]domain.Commit, error)
    Diff(ctx context.Context, path string, target DiffTarget, file string) (string, error)
    Remotes(ctx context.Context, path string) ([]domain.Remote, error)
}
```

Contract rules:

- A method returns a domain value or a typed error; it never returns raw bytes.
- Idempotent reads may be called concurrently.
- Mutating methods assume the caller holds the per-repository lock
  (see [`concurrency.md`](concurrency.md)).
- An empty result is a valid success (for example, no remotes, no branches).

---

## Repository Detection

### Is it a Git repository?

```bash
git -C <path> rev-parse --is-inside-work-tree
```

- Exit 0 with `true` → working tree.
- Exit 0 with `false` → bare repository; rejected in v0.1.
- Non-zero → not a repository.

### Resolve the repository root

```bash
git -C <path> rev-parse --show-toplevel
```

The output is canonicalized with `filepath.EvalSymlinks` and stored.

### Git availability and version

```bash
git --version
```

Parse `git version X.Y.Z`. Require `>= 2.30` (D-016). Missing Git is a fatal startup
error with installation guidance.

---

## Status Parsing

### Command

```bash
git -c core.quotepath=false status --porcelain=v2 --branch --untracked-files=all -z
```

Rationale:

- `--porcelain=v2` gives a documented, stable format including upstream and divergence.
- `--branch` adds the `#` header records.
- `--untracked-files=all` lists individual untracked files rather than collapsed
  directories.
- `-z` NUL-delimits records, making paths with spaces, tabs, newlines, and unicode safe.
- `core.quotepath=false` avoids C-style quoting of non-ASCII paths.

### Parsing strategy

```text
1. Split output on NUL.
2. For each chunk, additionally split on LF and process each line.
   (Headers are newline-terminated in some Git versions; splitting both ways is
   defensive and harmless because path fields never contain NUL.)
3. Ignore empty chunks.
4. Dispatch on the first character of each line.
```

### Record types

| Prefix | Meaning | Fields |
|---|---|---|
| `#` | Header | See header table |
| `1` | Changed entry | `1 XY sub mH mI mW hH hI path` |
| `2` | Renamed/copied entry | `2 XY sub mH mI mW hH hI X<score> path` then NUL then `origPath` |
| `u` | Unmerged entry | `u XY sub m1 m2 m3 mW h1 h2 h3 path` |
| `?` | Untracked | `? path` |
| `!` | Ignored | `! path` (only with `--ignored`; Repora does not request it) |

### Header records

| Header | Meaning | Handling |
|---|---|---|
| `# branch.oid <oid>` | HEAD commit | `(initial)` → unborn branch |
| `# branch.head <name>` | Current branch | `(detached)` → detached HEAD |
| `# branch.upstream <name>` | Upstream | Absent → no upstream |
| `# branch.ab +N -M` | Ahead/behind | Parsed only when upstream exists |

### XY status codes

`X` is the index (staged) status, `Y` is the worktree (unstaged) status.

| Code | Meaning |
|---|---|
| `.` | Unmodified |
| `M` | Modified |
| `T` | Type changed |
| `A` | Added |
| `D` | Deleted |
| `R` | Renamed |
| `C` | Copied |
| `U` | Updated but unmerged |

Classification rules:

- `X != '.'` → entry belongs to `Staged`.
- `Y != '.'` → entry belongs to `Unstaged`.
- Both may hold for the same path; the file then appears in both lists.
- Unmerged entries (`u` records) go to `Conflicts` and are excluded from Staged/Unstaged
  counts to avoid double counting in the UI.

### Edge cases that MUST be tested

- Unborn branch (`# branch.oid (initial)`), no commits.
- Detached HEAD (`# branch.head (detached)`).
- Branch with no upstream.
- Branch ahead only, behind only, diverged.
- Rename with and without staged content.
- Path containing a space, a tab, a newline, and non-ASCII characters.
- Merge conflict with multiple unmerged entries.
- Untracked directory containing many files (flattened by `-uall`).
- Submodule entries (treated as ordinary entries in v0.1; no recursive status).
- Empty repository (no commits, no files).

---

## In-Progress Operations

Git blocks several commands while a multi-step operation is pending. After reading status,
the adapter resolves the Git directory (`rev-parse --absolute-git-dir`) and checks for:

| Marker | Operation |
|---|---|
| `CHERRY_PICK_HEAD` | `cherry-pick` |
| `REVERT_HEAD` | `revert` |
| `MERGE_HEAD` | `merge` |
| `rebase-merge/` or `rebase-apply/` | `rebase` |
| `sequencer/` (leftover) | `cherry-pick` |

The operation is reported in `RepositoryState.Operation` and displayed by the UI. Repora
never resolves it automatically.

Known blocking errors are translated into an actionable hint (see `app.Explain`); the raw
Git output remains available in the error overlay.

---

## Operation Reference

All commands run with `cmd.Dir` set to the repository root. Pathspecs are separated with
`--` to prevent option injection.

### Fetch

```bash
git fetch --prune --no-recurse-submodules
```

- Default: the current branch's remote; if none, `origin`; if neither exists, an explicit
  `ErrNoRemote`.
- `--prune` removes stale remote-tracking refs.
- No tags beyond default behavior in v0.1.

### Pull

```bash
git pull --ff-only --no-recurse-submodules
```

- Fast-forward only (D-010).
- Divergence returns `ErrNotFastForward`, surfaced with a recommendation, not an automatic
  merge.

### Push

```bash
git push --no-recurse-submodules                        # upstream configured
git push -u <remote> <branch>                           # no upstream
```

- If the branch is unborn, fail with `ErrNothingToPush`.
- Force push is not exposed in v0.1.
- Protected-branch rejections are surfaced verbatim.

### Branches

```bash
git for-each-ref \
  --format='%(refname)%00%(refname:short)%00%(HEAD)%00%(upstream:short)%00%(upstream:track)%00%(objectname:short)%00%(authorname)%00%(authordate:iso-strict)%00%(contents:subject)' \
  refs/heads refs/remotes
```

Rationale: `for-each-ref` is machine-readable and avoids `git branch` formatting and
locale issues. `upstream:track` yields `[ahead 2, behind 1]` or `[gone]`, which is parsed
into counts.

Filtering rules:

- `refs/remotes/<remote>/HEAD` is excluded (symbolic ref, not a branch).
- Remote branches are grouped by remote for display.

### Create branch

```bash
git switch -c <name>              # create and checkout (default)
git branch <name>                 # create only
```

Name validation before execution:

- Non-empty, no whitespace, no `~ ^ : ? * [ \`, no `..`, no leading `-`, no trailing `/`,
  no `.lock` suffix, no `@{`.
- Not already existing (check `Branches` first or rely on Git's error).

### Checkout

```bash
git switch <name>
git switch --detach <hash>        # detached, only via explicit action
```

- If the working tree has conflicting changes, Git refuses; the error is surfaced
  unchanged.
- Detached-HEAD repositories can switch to a branch but not perform branch-scoped writes
  until attached.

### Delete branch

```bash
git branch -d <name>              # safe
git branch -D <name>              # force, requires confirmation
git push <remote> --delete <name> # deferred
```

Not exposed in the v0.1 UI (see [`../00-product/mvp-scope.md`](../00-product/mvp-scope.md))
but present in the adapter for later phases.

### Stage

```bash
git add -- <pathspec>...
git add -A --                     # stage all (modified, deleted, untracked)
```

- `-A` includes deletions; plain pathspec staging also records deletions.
- Never `git add .` without `--` separation.

### Unstage

```bash
git restore --staged -- <pathspec>...
git restore --staged -- .         # unstage all
```

- `git restore --staged` requires Git >= 2.23; the minimum is 2.30 (D-016).
- On an unborn branch, `git restore --staged` works for added files; if it fails, fall
  back to `git rm --cached -- <pathspec>`.

### Commit

```bash
git commit -m <message>
```

- Message is passed as a single argument.
- Empty or whitespace-only messages are rejected before execution.
- Commit hooks run normally. A hook failure is surfaced verbatim with the hook name when
  detectable.
- `--no-verify` is not exposed in v0.1.
- Nothing staged → `ErrNothingToCommit`, surfaced before invocation.

### Log

```bash
git log -n <limit> --date=iso-strict \
  --pretty=format:'%H%x1f%h%x1f%an%x1f%ae%x1f%aI%x1f%s%x1f%D'
```

- `%x1f` (unit separator) avoids collisions with message content.
- `%s` is the subject only, one line by construction.
- `%D` decorations are split on `, ` and trimmed.
- Default limit 50; maximum 500.

### Diff

```bash
git --no-pager diff --no-color --no-ext-diff -- <file>              # unstaged
git --no-pager diff --no-color --no-ext-diff --cached -- <file>     # staged
git --no-pager show --no-color --no-ext-diff --stat <hash>          # commit summary
```

- Color is always disabled; the TUI applies its own styling.
- `--no-ext-diff` prevents user-configured external diff tools from hijacking output.
- Diff output for a single file is bounded to 5,000 lines before rendering; truncation is
  indicated in the UI.
- Binary files produce `Binary files ... differ`; the UI shows a notice instead of
  rendering bytes.

### Remotes

```bash
git remote -v
git symbolic-ref --short refs/remotes/<remote>/HEAD   # default branch, optional
```

Parsed into name/fetch/push entries plus the remote's default branch. Displayed
read-only; the default branch pre-selects the pull request base.

### Refs

```bash
git rev-parse --verify --quiet <ref>
```

`RefOID` resolves `refs/heads/<branch>` and `refs/remotes/<remote>/<branch>` so pull
request creation can require the local and remote heads to match exactly.

---

## Error Translation

| Git condition | Domain error | UI behavior |
|---|---|---|
| Not a repository | `ErrNotAGitRepository` | Mark repository unavailable |
| Missing remote | `ErrNoRemote` | Explain and suggest `git remote add` in a shell |
| Authentication failure | `ErrAuthRequired` | Explain outside-terminal auth requirement |
| Non-fast-forward pull | `ErrNotFastForward` | Explain divergence, suggest fetch + manual integration |
| Push rejected | `ErrPushRejected` | Show Git's message verbatim |
| Nothing to commit | `ErrNothingToCommit` | Inline notice, not an error dialog |
| Nothing to push | `ErrNothingToPush` | Inline notice |
| `index.lock` exists | `ErrIndexLocked` | Explain a Git process may be running |
| Hook failure | `ErrHookFailed` | Show hook output and exit code |
| Timeout | `ErrTimeout` | Offer retry |
| Context cancelled | `ErrCancelled` | Silent, or a subtle status line notice |

Detection for authentication and lock errors is heuristic: match known stderr substrings.
The raw output is always preserved, so a false negative still shows useful text.

---

## Safety Rules

1. Never `sh -c`; never build a command string from user input.
2. Always separate pathspecs with `--`.
3. Always set `cmd.Dir` to the resolved repository root.
4. Validate branch names before invocation.
5. Reject empty commit messages and empty path lists.
6. Never run write operations implicitly (no auto-fetch on startup, no auto-pull).
7. Never delete or modify `.git` contents directly.
8. Never force-push, hard-reset, or clean from Repora in v0.1.
9. Never silently continue after a failed operation; refresh and re-read state.
10. Never discard user changes to resolve a conflict; report and stop.

---

## Testing

Integration tests use temporary repositories created in `t.TempDir()` and a local bare
remote. They MUST NOT touch real repositories.

Scenarios:

- Fresh repo: init, status semantics for unborn branch.
- Staged/unstaged/untracked mix.
- Rename detection with staged content.
- Conflict created by merging diverged branches.
- Detached HEAD.
- Ahead/behind after a local bare remote push.
- Pull fast-forward success and divergence failure.
- Push with and without upstream.
- Commit with hook that rejects.
- Paths with spaces, tabs, unicode, and newlines.
- Missing remote, missing upstream, deleted upstream (`[gone]`).

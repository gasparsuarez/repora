# MVP Scope — v0.1

**Status:** Defined
**Scope:** Authoritative scope for the first usable release.

If a capability is not listed as in scope here, it is not part of v0.1.

---

## MVP Definition

Repora v0.1 is a terminal application that lets a developer configure workspaces of
local Git repositories, see the state of every repository in a workspace, navigate into
an individual repository, and perform the basic daily Git operations there.

The MVP is delivered by phases 01, 02, and 03, plus the minimal packaging work in phase
05. Phase 04 is post-MVP.

---

## In Scope

### Configuration

- Load configuration from a YAML file.
- Persist configuration atomically.

### Workspaces

- Create, rename, and delete workspaces.
- Workspaces are Repora-owned containers: they exist only in Repora's configuration and
  have no representation in Git or on disk.
- The user explicitly chooses which repositories belong to each workspace.
- Register an existing local repository by path.
- Associate a registered repository with one or more workspaces.
- Remove a repository from a workspace without deleting the registration.
- Validate repository paths and detect invalid or missing repositories.

### Repository discovery

- Manual path entry is required.
- Detection of whether a given directory is a Git repository is required.
- Limited, explicitly bounded directory scanning MAY be supported for convenience
  (direct children only). Unrestricted recursive scanning is out of scope.

### Home / Workspace Explorer

- List all workspaces.
- Show repository count per workspace.
- Show an aggregate status summary per workspace.
- Navigate into a workspace.
- Filter/search workspaces by name.

### Workspace Dashboard

- List every repository in the workspace.
- Per repository, display: name, current branch, staged count, unstaged count, untracked
  count, ahead/behind counts, conflict indicator, and sync state.
- Concurrent refresh with a bounded worker pool.
- Per-repository loading state.
- Per-repository error state.
- Manual refresh of a single repository and of the whole workspace.
- Keyboard navigation and selection.
- Filter repositories by name.

### Repository View

- Overview of the selected repository: name, path, branch, upstream, ahead/behind,
  working tree summary, and the most recent commits.
- Changes list: staged, unstaged, and untracked files with their status codes.
- Branch list: local branches, current branch indicator, and upstream tracking state.
- History: recent commits with hash, subject, author, and date.
- Read-only diff for a selected file (staged and unstaged).

### Git operations

| Operation | Required in v0.1 |
|---|---|
| Fetch | Yes |
| Pull | Yes (fast-forward preferred; see [`git-integration.md`](../01-architecture/git-integration.md)) |
| Push | Yes |
| Create branch | Yes |
| Checkout branch | Yes |
| Stage file(s) | Yes |
| Stage all | Yes |
| Unstage all | Yes |
| Commit | Yes |
| View recent commits | Yes |
| View file diff | Yes |
| Delete branch | Yes (safe form; Git refuses unmerged) |
| Granular unstage per file | Yes |
| Amend commit | Deferred |

### Interaction

- Keyboard-first navigation across all primary screens.
- Contextual action bar showing available shortcuts.
- Configuration reachable from the UI: `,` opens the file in `$EDITOR` and reloads on exit;
  `Ctrl+R` reloads it from disk.
- Confirmation dialog for destructive or irreversible actions.
- Non-blocking operation feedback (spinners, status line).
- Cancellable long-running operations.
- Readable error surface that preserves Git's own error output.

---

## Out of Scope for v0.1

| Capability | Target |
|---|---|
| Full command palette | v0.2 |
| Workspace commands and process lifecycle | v0.2 |
| Process logs and running-process indicators | v0.2 |
| Git worktree management | v0.3 |
| Interactive rebase | v0.3 |
| Cherry-pick | v0.3 |
| Stash manager | v0.3 |
| Advanced conflict resolution | v0.3 |
| GitHub pull request creation | v0.2 |
| GitLab / Bitbucket integrations | v1.0+ |
| Remote repository creation | Not planned |
| Terminal multiplexer | Not planned |
| Workspace templates | v1.0 |
| Multi-user or shared configuration | Not planned |
| Plugin system | Not planned |
| Windows support | Not planned for v0.1 |

---

## Scope Decisions

The original specification (`/repora.md`) included more in v0.1. The following items
were deliberately reduced, with rationale:

1. **Command palette moved to v0.2.** A palette without workspace commands and process
   orchestration mostly duplicates keyboard shortcuts. Its cost is high (fuzzy matching,
   context resolution, action registry) and its MVP value is low. A minimal `Ctrl+K`
   navigation jump MAY ship at the end of phase 03 if time allows.
2. **Repository View starts as tabs, not five independent screens.** Overview, Changes,
   Branches, and History share layout and state; Remotes is folded into Overview until
   there is a real need for a dedicated screen.
3. **Branch deletion deferred.** It requires confirmation UX, unmerged detection, and
   remote-vs-local semantics. It is low frequency compared with create/checkout.
4. **Diff is read-only and paged.** No hunk staging, no side-by-side, no inline editing.
5. **No remote management UI.** Remotes are displayed read-only; adding/removing remotes
   is a shell task in v0.1.
6. **GitHub pull request creation moves to v0.2.** It is a single, explicit write behind a
   provider port and uses the authenticated GitHub CLI. GitLab, Bitbucket, forks, and PR
   listing remain later work.

---

## Acceptance Criteria

v0.1 is complete when, using a workspace of at least four real repositories:

1. Repora launches from the terminal with a single command.
2. Configuration loads; invalid repositories are reported without crashing the app.
3. The Home screen lists workspaces with aggregate status.
4. Opening a workspace shows every repository with its state, refreshed concurrently.
5. Dirty, clean, conflicted, and unsynchronized repositories are visually distinct.
6. Selecting a repository opens its view with real branch and change data.
7. Create branch and checkout branch work on the selected repository only.
8. Stage and commit work on the selected repository only.
9. Fetch, pull, and push work on the selected repository only.
10. Recent commit history is visible.
11. Returning to the dashboard preserves workspace state and refreshes correctly.
12. All operations remain responsive; no UI freeze during network operations.

---

## Success Metrics (internal, pre-release)

These are guiding metrics, not product analytics. Repora does not collect telemetry in
v0.1.

- Time from launch to a complete workspace view: under 3 seconds for 10 repositories on
  a warm machine.
- Zero ambiguous-target operations in manual testing.
- Zero data-loss incidents in integration testing.
- `go test ./...` and `go test -race ./...` pass consistently.

---

## Assumptions and Constraints

- Git (>= 2.30) is installed and on `PATH`.
- The user has configured SSH keys or credential helpers outside Repora.
- Repositories are non-bare working trees.
- Repositories are local; network-mounted paths are supported but not optimized.
- macOS and Linux only.
- Single user, single machine, one Repora instance per config directory.

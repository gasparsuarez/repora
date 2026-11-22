# Screens

**Status:** Defined
**Scope:** Per-screen specification for v0.1. Interaction principles live in
[`ux-principles.md`](ux-principles.md); visual encoding in
[`visual-system.md`](visual-system.md).

Each screen section states: purpose, data, layout, actions, states, and keyboard.

---

## 1. Home / Workspace Explorer

### Purpose

Answer "which projects do I work on, and what is their state?" and provide entry into a
workspace.

### Data

Per workspace:

- Name and optional description.
- Repository count and repository names (as tags).
- Aggregate counts derived from the cached repository states: conflicts, modified,
  synchronized (ahead/behind/diverged), errors, clean.
- Loading state while the background refresh is in flight.

The explorer refreshes every repository referenced by a workspace, using the bounded
status service, and caches the results for the session. The automatic refresh timer
updates those summaries in place without clearing the cache or showing a spinner for
already-loaded cards. Counts are derived from `RepositoryState.Class()`; they are never
hardcoded and omitted when zero.

### Layout

```text
◈  Repora   WORKSPACE MANAGER
────────────────────────────────────────────────────────

YOUR WORKSPACES

╭──────────────────────────────────────────────────────────╮
│ ◆ Product X                                      4 repos │
│ Customer-facing product                                  │
│ [API] [Web] [Infrastructure] [Shared]                    │
│ ✖ 1 conflict  ● 1 modified  ↑ 1 sync  ✓ 1 clean          │
╰──────────────────────────────────────────────────────────╯

╭──────────────────────────────────────────────────────────╮
│ ◇ Personal                                       1 repo  │
│                                                          │
│ [Dotfiles]                                               │
│ ✓ 1 clean                                                │
╰──────────────────────────────────────────────────────────╯

↑↓ Navigate   ↵ Open   n New   e Rename   d Delete   r Refresh   ? Help
```

Rules:

- Cards have a fixed height and never jitter when the selection moves.
- The selected card uses an accent border and a `◆` marker; unselected cards use a neutral
  border and `◇`.
- Repository tags collapse into `+N` when they do not fit.
- Aggregates are ordered by severity and omit zero counts.
- Only whole cards are rendered; when more remain, a `↓ N more` hint appears.
- The brand header is separated from content by a hairline rule and shows the build
  version on the right.
- The explorer has no faux buttons: `n` is advertised in the footer, not as unclickable
  text.

### Actions

| Action | Key | Behavior |
|---|---|---|
| Open | `Enter` (or `↵`) | Push the workspace dashboard |
| New workspace | `n` | Prompt for name; create; select |
| Rename workspace | `e` | Prompt prefilled; update |
| Delete workspace | `d` | Confirm; remove associations only |
| Refresh summaries | `r` | Re-read every workspace repository |
| Open config | `,` | Suspend the TUI, open the file in `$EDITOR`, reload on exit |
| Reload config | `Ctrl+R` | Re-read the file from disk without leaving Repora |
| Move selection | `↑ ↓` / `j k` | Scroll the card list |
| First / last | `g` / `G` | Jump the selection |
| Help | `?` | Help overlay |
| Quit | `q` | Quit (confirm if operations are in flight) |

### States

- **Empty:** `No workspaces yet.` plus the config path and a hint to press `n`.
- **Loading:** cards without cached states show `◐ loading status…`.
- **Partially loaded:** cards show the counts already known; no fabricated values.
- **Empty workspace:** `0 repos`, `No repositories yet`, `↵ open, then a to add`.
- **No repositories at all:** the frame stays stable; the empty state replaces the cards.
- **Warnings:** the header shows `! N warnings` on the right.

### Notes

- Deleting a workspace never deletes repositories from disk or from the global
  registration.
- Workspace IDs are immutable; renaming changes only the display name.
- Returning from a workspace reuses the states already loaded by the dashboard instead of
  issuing a second round of Git calls.

---

## 2. Workspace Dashboard

Every screen below Home shares the brand header (`◈ Repora — WORKSPACE MANAGER`) followed
by a breadcrumb whose active segment uses the accent color.

### Purpose

The central screen. Show every repository in the workspace with enough state to decide
what to act on, and allow acting on one.

### Data

Per repository:

- Name, current branch (or `HEAD` when detached, `(no commits)` when unborn).
- Staged, unstaged, untracked counts; conflict count.
- Ahead/behind counts.
- Sync state.
- Availability (path missing, not a repository).
- Refresh status (pending, loading, error) and last fetched time.

Workspace header:

- Name and description.
- Repository count.
- Aggregate counts.

### Layout

```text
◈  Repora   WORKSPACE MANAGER
──────────────────────────────────────────────────────
WORKSPACES  /  PRODUCT X                    4 repositories
Customer-facing product
╭────────────────────────────────────────────────────╮
│ > api            ● modified  feature/payment-flow  │
│   web            ✓ clean     develop               │
│   infra          ↓ behind 2  main                  │
│   shared         ! unavailable                     │
╰────────────────────────────────────────────────────╯

Enter Open   a Add   x Remove   f Fetch   r Refresh   Esc Back
```

Rules:

- The selected row may expand to two lines to show the branch when the branch column is
  dropped by width.
- Unavailable repositories remain listed, dimmed, and selectable for inspection.
- One blank line separates the header block from the list.
- The action bar shows actions applicable to the selected row; actions requiring a branch
  are omitted for unavailable repositories.

### Actions

| Action | Key | Scope | Confirmation |
|---|---|---|---|
| Open repository | `Enter` | Row | — |
| Refresh workspace | `r` | Workspace | — |
| Refresh repository | `R` | Row | — |
| Fetch | `f` | Row | — |
| Pull | `p` | Row | — |
| Push | `P` | Row | — |
| Add repository | `a` | Workspace | — |
| Remove from workspace | `x` | Row | Yes |
| Open config | `,` | — | — |
| Reload config | `Ctrl+R` | — | — |
| Change selection | `↑ ↓ j k` | — | — |
| Jump first/last | `g G` | — | — |
| Filter | `/` | Workspace | — |
| Open error detail | `e` | Row (error) | — |
| Help | `?` | — | — |
| Back | `Esc` | — | — |

### States

- **Empty workspace:** `This workspace has no repositories.` plus a visible
  `a Add a repository by path` hint and `a Add` in the action bar.
- **All clean:** no special state; rows show `✓ clean`.
- **Partial loading:** rows independently transition `queued → refreshing → ready`.
- **Row error:** state cell shows `✗ error`; `e` opens the error detail modal.
- **Operation pending:** state cell shows `⏸ cherry-pick` (or merge/rebase/revert); branch
  switching is blocked until the operation is resolved in a terminal.
- **Stale data:** if a snapshot is older than the refresh interval, the row's timestamp
  is shown muted.

### Notes

- The dashboard MUST NOT run write operations on open.
- Refresh is always safe to trigger repeatedly; duplicate requests are deduplicated.
- Selection is preserved across refresh.

---

## 3. Repository View

Shares the brand header and the breadcrumb; the active repository is the accented segment
and the current state is right-aligned.

### Purpose

Inspect and operate on one repository, with the target explicitly visible at all times.

### Common header

```text
◈  Repora   WORKSPACE MANAGER
──────────────────────────────────────────────────────────
WORKSPACES  /  PRODUCT X  /  api          ↑1 ↓0   ● modified
~/code/product-x/api
╭──────────────────────────────────────────────────────╮
│ Overview   Changes (7)   Branches   History          │
│                                                      │
│  STATUS                                              │
│   ...                                                │
╰──────────────────────────────────────────────────────╯
```

Rules:

- The repository name appears in the breadcrumb and again in the summary line.
- The path is shown with `~` shortening and middle truncation.
- The tab bar shows counts where meaningful (`Changes (7)`).
- Tab bar is always visible; `←/→` or `1-4` switch tabs.

### Tab selection keys

| Key | Tab |
|---|---|
| `1` | Overview |
| `2` | Changes |
| `3` | Branches |
| `4` | History |
| `←` `→` | Cycle tabs |
| `Tab` | Move focus into content |

---

### 3.1 Overview

#### Data

- Branch, upstream, ahead/behind, detached/unborn flags.
- Working tree summary (counts only).
- Remotes (read-only): name, fetch URL, push URL.
- Last 5 commits.
- Last refresh timestamp.

#### Layout

```text
Status
  Branch        main → origin/main
  Sync          ↑1 ahead, ↓0 behind
  Working tree  2 staged · 3 unstaged · 2 untracked
  Conflicts     none

Remotes
  origin        git@github.com:acme/api.git

Recent commits
  a1b2c3d  Add payment retry logic               Ana Ruiz · 2 hours ago
  9f8e7d6  Refactor settlement service           Ana Ruiz · yesterday
  ...

[r] Refresh   [f] Fetch   [p] Pull   [P] Push   [b] Branches   [Esc] Back
```

#### Actions

- Refresh, fetch, pull, push.
- `b` jumps to Branches.
- `Enter` on a commit opens the commit detail overlay (hash, subject, author, email, date,
  refs). A patch/stat view is a later refinement.

---

### 3.2 Changes

#### Data

- Staged entries with status codes.
- Unstaged entries with status codes.
- Untracked files.
- Conflicts (rendered first, in error color).
- Diff for the selected file (staged or unstaged depending on the section).

#### Layout

```text
Changes

Conflicts (1)
  ✖ src/payments/retry.go

Staged (2)
  M  src/payments/handler.go
  A  src/payments/retry.go

Unstaged (3)
  M  src/payments/handler.go
  M  src/config/env.go
  D  src/legacy/client.go

Untracked (2)
  ?  notes/todo.md
  ?  scripts/debug.sh

┌ Diff: src/payments/handler.go (unstaged) ─────────────┐
│ @@ -41,6 +41,8 @@                                     │
│  func (s *Service) Handle(ctx context.Context) error {│
│ -    return s.process(ctx)                             │
│ +    if err := s.process(ctx); err != nil {            │
│ +        return fmt.Errorf("process: %w", err)         │
│ +    }                                                  │
│ +    return nil                                         │
│  }                                                      │
└─────────────────────────────────────────────────────────┘

[Space] Stage   [c] Commit   [u] Unstage all   [Enter] Diff   [Esc] Back
```

Rules:

- Sections render only when non-empty.
- The diff opens as a full-screen overlay on `Enter` (see D-019), closed with `Esc`. A split
  pane beside the list is a future refinement.
- The selected file is highlighted in both its section and the diff header.
- Diff lines are never wrapped.

#### Actions

| Action | Key | Scope | Confirmation |
|---|---|---|---|
| Stage or unstage selected entry | `Space` | Staged, unstaged, or untracked entry | — |
| Stage all | `A` | Repository | — |
| Unstage all | `u` | Repository | — |
| Commit | `c` | Repository | — |
| Open diff | `Enter` | Selected entry | — |
| Scroll diff | `J K` / `PgUp PgDn` | Diff pane | — |
| Refresh | `r` | Repository | — |

Notes:

- `Space` stages an unstaged or untracked entry and unstages a staged one.
- `u` unstages everything.
- Staging an entry moves it to the Staged section after refresh.
- Commit requires at least one staged file; otherwise `c` shows an inline notice.

---

### 3.3 Branches

#### Data

- Local branches: name, current marker, upstream, ahead/behind.
- Remote branches grouped by remote.
- Detached/unborn indicators.

#### Layout

```text
Branches

Local
> ✓ main              → origin/main       ↑1
    develop           → origin/develop    ✓
    feature/pay-flow  (no upstream)

Remote
  origin/main
  origin/develop
  origin/feature/pay-flow

[n] New branch   [Enter] Checkout   [Esc] Back
```

#### Actions

| Action | Key | Confirmation |
|---|---|---|
| Create branch | `n` | Prompt for name; `Enter` creates and checks out |
| Checkout | `Enter` | Refuses if uncommitted changes conflict; Git error surfaced |
| Delete branch | `d` | Confirmation required; Git refuses unmerged branches |
| Refresh | `r` | — |

Rules:

- Remote branches are read-only in v0.1; `Enter` on a remote branch creates a local
  tracking branch only if the user confirms, otherwise it is display only. Recommended:
  read-only, no checkout from a remote row.
- Branch name validation happens inline before submission.

---

### 3.4 History

#### Data

- Recent commits, default 50.
- Hash, subject, author, date, refs.
- Commit detail on demand: full message, author, committer, changed files stat.

#### Layout

```text
History

> a1b2c3d  Add payment retry logic
    HEAD -> main, origin/main · Ana Ruiz · 2h ago
  9f8e7d6  Refactor settlement service
    Ana Ruiz · 1d ago
...

[Enter] Details   [r] Refresh   [Esc] Back
```

#### Actions

| Action | Key |
|---|---|
| Open commit detail (metadata + patch) | `Enter` |
| Scroll | `↑ ↓ PgUp PgDn` |
| Refresh | `r` |

Notes:

- Commit subject and refs are always separated: subject on the first line, refs and author
  on a second, muted line, so long refs such as `HEAD -> main` never clip.
- No graph rendering in v0.1.
- Pagination beyond the limit is not supported; `l` loads 50 more (optional, low
  priority).

---

## 4. Modals

### 4.1 New / Rename Workspace

Two steps: name, then an optional description. Forms use the same shell as every other
screen: brand header, rule, breadcrumb, a focused panel, and the footer.

```text
◈  Repora   WORKSPACE MANAGER
────────────────────────────────────────────
WORKSPACES

╭──────────────────────────────────────────╮
│ NEW WORKSPACE                            │
│                                          │
│ > Product X                              │
│                                          │
│ The workspace id is derived from the name.│
╰──────────────────────────────────────────╯

 Enter Submit   Esc Cancel
```

```text
◈  Repora   WORKSPACE MANAGER
────────────────────────────────────────────
WORKSPACES  /  Product X

╭──────────────────────────────────────────╮
│ DESCRIPTION                              │
│                                          │
│ > Optional description                   │
│                                          │
│ Optional. Enter saves without a description.│
╰──────────────────────────────────────────╯

 Enter Submit   Esc Cancel
```

- Name is required and trimmed; description is optional and trimmed.
- `Esc` cancels the whole flow, including the name typed in the first step.
- ID is derived silently from the name; a collision appends `-2`, `-3`.
- Rename keeps the ID and prefills both the current name and description.
- The description appears on the workspace card.

### 4.2 Delete Workspace

```text
┌ Delete workspace "Product X"? ───────────┐
│ Removes the workspace from Repora.       │
│ Repositories are not deleted from disk.  │
│                                          │
│           [Esc] Cancel   [Enter] Delete  │
└──────────────────────────────────────────┘
```

### 4.3 Add Repository

Two-step:

```text
┌ Add repository ──────────────────────────┐
│ Path                                     │
│ > ~/code/product-x/api                   │
│                                          │
│            [Esc] Cancel   [Enter] Detect │
└──────────────────────────────────────────┘
```

Then a confirmation summary:

```text
┌ Add repository ──────────────────────────┐
│ Name    API                              │
│ Path    ~/code/product-x/api             │
│ Branch  main                             │
│                                          │
│            [Esc] Cancel   [Enter] Add    │
└──────────────────────────────────────────┘
```

- Path is expanded and validated with `rev-parse --show-toplevel`.
- If the path is inside a repository, the resolved root is shown and requires confirmation.
- If the repository is already registered, the modal offers to associate the existing
  registration instead of duplicating it.

### 4.4 Remove Repository from Workspace

- Confirmation only; registration is untouched.

### 4.5 New Branch

```text
┌ New branch in api ───────────────────────┐
│ Name                                     │
│ > feature/payment-retry                  │
│                                          │
│ x invalid character "/" at position 8    │
│            [Esc] Cancel   [Enter] Create │
└──────────────────────────────────────────┘
```

### 4.6 Commit

```text
┌ Commit to api ───────────────────────────┐
│ Message                                  │
│ > Add payment retry logic                │
│                                          │
│ 2 files staged                           │
│            [Esc] Cancel   [Enter] Commit │
└──────────────────────────────────────────┘
```

- Single-line message in v0.1.
- On failure, the modal stays open with the message intact and the error inline.

### 4.7 Error Detail

```text
┌ Error · api · pull ──────────────────────┐
│ not possible to fast-forward, aborting.  │
│                                          │
│ git pull --ff-only                       │
│ exit code 128                            │
│                                          │
│ Diverged branch. Fetch and integrate     │
│ manually in a terminal.                  │
│                                   [Esc]  │
└──────────────────────────────────────────┘
```

### 4.8 Help

- Lists global keys and the keys for the current screen.
- Two columns at >= 100 columns wide, one column otherwise.
- Generated from the key registry, not hard-coded strings.

---

## 5. Cross-Screen Requirements

- Every screen renders correctly at 80x24.
- Every screen has an empty state where emptiness is possible.
- Every action that can fail shows the failure with the repository name and Git output.
- No screen performs I/O in `View`.
- No screen mutates domain state directly.
- Every screen can be exercised in tests with a fake application service.

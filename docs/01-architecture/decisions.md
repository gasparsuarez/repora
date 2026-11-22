# Technical Decisions

**Status:** Active
**Scope:** Recorded decisions with rationale. Each decision has a stable ID. Superseding a
decision requires a new entry that references the old one.

Format is a condensed ADR: context, decision, consequences, alternatives.

---

## D-001 — Delegate all Git work to the native Git CLI

**Status:** Accepted

**Context:** Repora needs repository status, history, diffs, branches, and
synchronization. Reimplementing Git in Go means implementing packfiles, refs, index
formats, hooks, credential helpers, SSH configuration, and merge semantics.

**Decision:** Invoke the installed `git` binary through `os/exec` with an explicit working
directory and an argument vector. Never use `sh -c`.

**Consequences:**

- Full compatibility with user configuration, credential helpers, hooks, and SSH.
- Git behavior and messages are the real ones.
- Output parsing becomes a real responsibility and must be tested carefully.
- Repora inherits Git's version differences; a minimum version must be enforced.

**Alternatives rejected:** `go-git` (incomplete edge-case coverage, different semantics,
silent divergence from user's Git), libgit2 bindings (CGo and packaging cost).

---

## D-002 — YAML configuration instead of a database

**Status:** Accepted

**Context:** Configuration is a small, human-editable set of workspaces and repositories.

**Decision:** Store configuration in a single versioned YAML file.

**Consequences:**

- Users can edit and version their configuration.
- No migration framework or embedded database.
- Schema evolution must be handled explicitly with a `version` field.
- Atomic writes and unknown-field preservation become mandatory.

**Alternatives rejected:** SQLite (overkill, opaque to users, migration burden), JSON
(no comments, noisier for human editing), TOML (acceptable but no advantage here).

---

## D-003 — Bubble Tea for the TUI

**Status:** Accepted

**Context:** The application is keyboard-driven, asynchronous, and composed of multiple
screens with independent state.

**Decision:** Use Bubble Tea (Elm architecture) with Lip Gloss for styling and Bubbles for
reusable components.

**Consequences:**

- Clear separation between state transitions (`Update`) and rendering (`View`).
- Asynchronous work maps naturally to `tea.Cmd` and messages.
- Component reuse via Bubbles (list, textinput, viewport, spinner).
- Testing relies on message-driven assertions rather than screen scraping.

**Alternatives rejected:** tview/tcell (imperative, harder to test deterministically),
custom ANSI rendering (reinventing layout and input handling).

---

## D-004 — No Cobra or Viper in the MVP

**Status:** Accepted

**Context:** The MVP has a small command surface: launch the TUI, maybe a `--config`
flag and a `version` command.

**Decision:** Use the standard library for flag parsing. Introduce a CLI framework only
when subcommands (for example `repora doctor`, `repora workspace add`) exist and are
justified.

**Consequences:** Fewer dependencies, no hidden global state from Viper, explicit
configuration loading. A future CLI layer can be added without touching domain code.

---

## D-005 — Configuration lives in `~/.config/repora/`

**Status:** Accepted

**Context:** `os.UserConfigDir()` returns `~/Library/Application Support` on macOS and
`~/.config` on Linux.

**Decision:** Use `$XDG_CONFIG_HOME/repora/config.yaml`, falling back to
`~/.config/repora/config.yaml` on both macOS and Linux, for a single documented path.
Provide `--config` and `REPORA_CONFIG` overrides.

**Consequences:** Predictable path across platforms and easy to document. Deliberately
deviates from the macOS convention; acceptable because the audience is terminal-first and
the original specification required `~/.config`.

**Alternatives rejected:** `os.UserConfigDir()` (inconsistent across platforms),
`~/.repora` (clutters home directory).

---

## D-006 — Repository state is never persisted

**Status:** Accepted

**Context:** Branch, working tree, and divergence data change constantly and are fully
derivable from Git.

**Decision:** Repository state exists only in memory. Configuration stores identity
(`id`, `name`, `path`) and nothing derived.

**Consequences:** No cache invalidation bugs, no stale data on disk, no privacy concern
about leaking repository contents into config. Cold start always reflects reality.

---

## D-007 — Refresh is read-only and bounded

**Status:** Accepted

**Context:** A workspace may contain many repositories; naive concurrent refresh can
saturate the machine or hammer remotes.

**Decision:** Refresh uses a worker pool of 4 by default. Only read-only commands run.
Write operations are never triggered by refresh, navigation, or startup.

**Consequences:** Predictable resource usage; per-repository loading states are required
because results arrive out of order. Pool size becomes a preference with a safe default.

---

## D-008 — Writes are serialized per repository

**Status:** Accepted

**Context:** Two concurrent Git writes in the same repository can corrupt the index or
produce confusing lock errors.

**Decision:** Acquire a per-repository mutex for any mutating operation. Different
repositories may mutate concurrently.

**Consequences:** `index.lock` errors from Repora itself are effectively eliminated. UI
must communicate "operation in progress" for the locked repository. Stale locks left by
crashed Git processes are surfaced, not deleted automatically.

---

## D-009 — Non-interactive Git environment

**Status:** Accepted

**Context:** A TUI cannot host Git's interactive credential prompts. A blocked prompt
would freeze the interface.

**Decision:** Execute Git with `GIT_TERMINAL_PROMPT=0`, no stdin, and a cancelled-prompt
environment. If authentication is required, Git fails fast and the error is surfaced with
guidance to authenticate in a normal terminal.

**Consequences:** Repora never hangs on credentials. Users must have SSH agents or
credential helpers configured. Interactive flows (editor, `GIT_EDITOR`, `less`) are
disabled or invoked only in explicit later features.

---

## D-010 — `pull` defaults to fast-forward

**Status:** Accepted

**Context:** A pull that creates a merge commit silently changes history, which is
surprising inside a TUI where the user may not see the merge commit.

**Decision:** Run `git pull --ff-only` by default. Divergence produces an explicit error
with a suggested next step. Merge and rebase pull strategies are opt-in (post-MVP).

**Consequences:** No accidental merge commits. Users with diverged branches get a clear
failure instead of a silent merge. Documented in the UI.

---

## D-011 — Parse `git status --porcelain=v2 --branch -z`

**Status:** Accepted

**Context:** Human-readable `git status` output is locale-dependent, truncated, and
ambiguous for renames and special characters.

**Decision:** Use porcelain v2 with NUL-delimited records and `core.quotepath=false`.

**Consequences:** Stable, documented machine format; safe handling of spaces, tabs, and
non-ASCII paths; explicit field layout for renames and conflicts. Parser must be covered
by table-driven tests for every record type.

**Alternatives rejected:** `--porcelain=v1` (loses upstream and rename detail),
`git status` human output (unstable), `go-git` status (different semantics).

---

## D-012 — Construct commands as argument vectors

**Status:** Accepted

**Context:** Shell interpolation is the classic command-injection vector, especially with
user-provided branch names and paths.

**Decision:** Every Git invocation is a `[]string` argument vector passed to
`exec.CommandContext`. Pathspecs are separated with `--`. Branch names are validated
before use.

**Consequences:** No shell injection surface. Argument handling is explicit. Validation
must reject names Git itself would reject.

---

## D-013 — Asynchronous results are versioned by generation

**Status:** Accepted

**Context:** Users navigate quickly; refresh results may arrive after the user has left a
workspace or after a newer refresh started.

**Decision:** Each refresh increments a generation counter. Messages carrying an older
generation are ignored on arrival.

**Consequences:** No flicker of stale data. Simple to implement and test. Requires every
async message to carry its generation.

---

## D-014 — Unknown configuration fields are preserved

**Status:** Accepted

**Context:** Users may add fields by hand or with a newer Repora version; older binaries
should not silently drop them.

**Decision:** On save, merge the known schema into the original YAML node tree and write
back unknown keys untouched.

**Consequences:** Forward-compatible configuration. Save logic is more complex and needs
round-trip tests.

**Refinement (D-020):** "Unknown" means keys Repora does not own. Top-level keys outside
`version`, `repositories`, `workspaces`, and `preferences` are always preserved. Inside
known objects, unknown fields are preserved by matching list items on `id`. List membership
follows the in-memory configuration, so a workspace deleted in Repora is deleted from the
file rather than resurrected from the old document.

---

## D-015 — Workspace IDs are stable slugs, not UUIDs

**Status:** Accepted

**Context:** Configuration is human-editable; opaque identifiers make it painful to read
and diff.

**Decision:** IDs are lowercase slugs (`^[a-z0-9]+(-[a-z0-9]+)*$`), unique, derived from
the display name at creation time, and immutable afterwards. Display name is editable
independently.

**Consequences:** Readable config and CLI ergonomics. Renaming a workspace does not change
its ID. Renaming a repository does not change its ID. Uniqueness is validated on load and
on create.

---

## D-016 — Minimum Git version 2.30

**Status:** Accepted

**Context:** Repora uses `git switch`/`git restore` semantics and porcelain v2 details.

**Decision:** Require Git >= 2.30 at startup and report the detected version otherwise.

**Consequences:** Predictable command availability. Users on older systems get a clear
message rather than cryptic failures.

---

## D-018 — The configuration file is always directly editable, with an in-TUI shortcut

**Status:** Accepted

**Context:** Workspaces are Repora-owned configuration (D-017). Building a full settings UI
before users can create contexts would delay the product's core value, and hand-edited
configuration stays useful for bulk edits, comments, and versioning.

**Decision:** Offer two first-class paths to configuration:

1. `,` suspends the TUI, opens the configuration file in `$VISUAL` / `$EDITOR`, and reloads
   it when the editor exits.
2. `Ctrl+R` reloads the file from disk without leaving Repora.

In-app create/rename/delete flows for workspaces and repository associations remain planned
for phase 02 and do not replace the file-based path.

**Consequences:**

- Users can configure workspaces from day one, including on machines where the TUI is
  incomplete.
- The configuration file remains the single source of truth; comments and unknown fields
  survive because Repora does not rewrite the file to read it.
- Reload is non-destructive: an invalid edit leaves the running state untouched and
  surfaces the parse error.
- Editor commands are parsed by whitespace, so quoting inside `$EDITOR` is not supported in
  v0.1.
- With no `$VISUAL` / `$EDITOR`, Repora falls back to the first of `nano`, `vim`, or `vi`
  found on `PATH`. If none exists, it shows a notice with the literal path instead of
  failing.

**Alternatives rejected:** settings UI only (slow to ship, worse for bulk edits), hot-reload
via filesystem watching (unpredictable mid-edit reloads), requiring a restart (breaks flow).

---

## D-020 — Configuration writes preserve unknown fields by object identity

**Status:** Accepted

**Context:** Repora now writes configuration for workspace CRUD. D-014 requires preserving
fields Repora does not understand, but a blind rewrite would drop them and a blind merge
would resurrect deleted entities.

**Decision:** Merge at two levels:

1. **Top level** — keys other than `version`, `repositories`, `workspaces`, and
   `preferences` are copied verbatim.
2. **Inside known lists and objects** — list membership follows the in-memory
   configuration (deletions stick), while unknown fields on surviving items are copied from
   the previous document by matching `id`. Unknown preference keys are preserved.

Comments are not guaranteed to survive; that is a documented limitation.

**Consequences:**

- A newer Repora can add fields without older builds destroying them.
- Deleting a workspace or repository in the UI is durable.
- The merge is testable with round-trip fixtures and needs tests for every future field.
- Save validates before writing, so an invalid in-memory state never reaches disk.

---

## D-019 — The diff renders as a full-screen overlay in v0.1

**Status:** Accepted

**Context:** The screen specification describes a diff pane below the file list at wide
sizes and a full-screen diff otherwise. A split pane requires its own scroll region, focus
handling, and layout math while the list also needs a cursor.

**Decision:** Ship the diff as a full-screen overlay in v0.1, opened with `Enter` and closed
with `Esc`. It is bounded to 5,000 lines and uses the same scroll keys as lists.

**Consequences:**

- Simpler, predictable layout at every terminal size.
- The file list keeps full height, which matters for repositories with many changes.
- The split pane remains a future refinement; the diff source is already abstracted behind
  `RepositoryService.Diff`.
- Users lose the ability to scan list and diff simultaneously.

---

## D-017 — Workspaces are Repora-owned logical containers

**Status:** Accepted

**Context:** Users need contexts such as "work" and "personal" whose repositories overlap
and live in unrelated directories. Coupling that grouping to Git or to the filesystem would
force repository changes, duplicate clones, or a rigid directory layout.

**Decision:** A workspace is a container owned by Repora and persisted only in Repora's
configuration. It references globally registered repositories by ID. Membership is explicit
and manual. Workspaces never alter Git state or behavior.

**Consequences:**

- Nothing is written into repositories: no `.repora` file, nothing to commit.
- Repository state, locks, and operations are keyed by repository ID, so a repository
  behaves identically in every workspace.
- The same repository can appear in several workspaces with a single working tree.
- Deleting a workspace is non-destructive; deleting a registration removes it from every
  workspace.
- Configuration is the single source of truth for membership; there is no discovery step
  that reconstructs workspaces.

**Alternatives rejected:** Git submodules (modify the repository and require commits), a
marker file inside each repository (couples workspace to version control), editor workspace
files (tool-specific and file-based), directory scanning (implicit and fragile).

---

## D-021 — Workspace Explorer visual refresh

**Status:** Accepted

**Context:** The first explorer rendered workspaces as flat text rows with no status data,
so the screen could not answer "what is the state of this project?" without opening each
workspace. The interface also had no shared component vocabulary.

**Decision:** Introduce a small design system and rebuild the explorer around it:

1. **Tokens** move to `internal/tui/theme` with the refreshed palette (D-021 palette:
   background `#17191E`, surface `#22252C`, accent `#A3E635`, warning `#F5B84B`, danger
   `#F16B6B`, info `#60A5FA`).
2. **Components** move to `internal/tui/components`: header, workspace card, repository
   tags, status indicators, footer. They take data plus a theme and never touch services.
3. **Workspace cards** have a fixed height, an accent border when selected, a `◆`/`◇`
   marker, repository tags, and aggregated counts.
4. **Aggregation uses real data**: the explorer refreshes every repository referenced by a
   workspace through the existing bounded status service and caches the results for the
   session. Counts are derived from `RepositoryState.Class()`.

**Consequences:**

- The explorer shows real state at startup; cost is one `git status` per referenced
  repository, bounded by the existing worker pool and cancellable.
- Returning from a workspace reuses the states already loaded there.
- Cards do not shift when the selection moves because height is fixed.
- Screens stay in `internal/tui`; only the design system and stateless components are
  extracted. Moving screens into subpackages would require exporting model internals for
  no functional gain.
- Success and Accent share a hue; differentiation relies on context, glyphs, and words.

**Alternatives rejected:** aggregation on demand only (empty explorer), flat text rows
(no hierarchy), per-screen styling (inconsistent and hard to change).

---

## D-022 — In-progress Git operations are surfaced, never resolved

**Status:** Accepted

**Context:** Git blocks branch switching while a cherry-pick, merge, rebase, or revert is
pending. Leftover state such as a stale `sequencer` directory produces a long error that is
easy to truncate and hard to act on.

**Decision:** Detect the operation from the Git directory markers during status refresh and
show it as a first-class repository state (`⏸ cherry-pick`, `⏸ merge`, …). Failures caused
by a known condition are translated into a one-line actionable hint, with the raw Git output
kept in the error overlay.

Repora never runs `--continue`, `--abort`, or `--quit`: resolving an operation is a user
decision that requires seeing conflict state.

**Consequences:**

- The dashboard answers "why can't I switch branches?" without opening a terminal.
- Errors stay complete: the overlay wraps and scrolls instead of clipping.
- Hints are deliberately generic about the consequence; the user runs the command.
- Detection costs one `rev-parse --absolute-git-dir` plus a few `stat` calls per refresh.

**Alternatives rejected:** parsing human error strings only (misses the cause before an
action), auto-aborting (destructive and surprising), ignoring the state (what produced the
bad experience in the first place).

---

## D-023 — Pull request creation delegates to the authenticated GitHub CLI

**Status:** Accepted

**Context:** Creating a pull request needs an authenticated hosting-provider API. Storing
tokens in Repora's configuration would break the rule that configuration never contains
credentials. Reimplementing GitHub authentication and its token lifecycle adds a large
security surface for a single operation.

**Decision:** Pull request creation uses the installed GitHub CLI through `gh api` with an
argument vector and JSON on stdin. Repora never stores forge credentials and never reads
tokens directly. The provider is behind `domain.ForgePort`, so another adapter can be added
later without changing the TUI or application service.

GitHub is the only provider in v0.2. The feature is limited to pull requests inside the
same repository; forks and remote repository creation are out of scope.

**Consequences:**

- `gh` must be installed and authenticated for the feature; the UI surfaces an actionable
  error when it is missing.
- Network work happens only after the user explicitly submits the pull request form. No
  provider call runs during startup, navigation, or refresh.
- The head branch must already be pushed to the selected remote at the local commit. Repora
  never pushes implicitly as part of creating a pull request.
- The GitHub CLI can target GitHub Enterprise through its own authentication and
  `--hostname`.

**Alternatives rejected:** unauthenticated HTTP (impossible for writes), storing a PAT in
configuration (violates the configuration rules), running `gh pr create` with interactive
prompts (the TUI cannot host them), automatic push before creating the PR (an implicit
network write).

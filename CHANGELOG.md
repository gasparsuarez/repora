# Changelog

All notable changes to this project are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- The install script honours an explicit `--dir` even when Repora is already installed
  through Homebrew.

## [0.1.5] - 2026-11-22

### Added

- Homebrew cask publishing. Install with
  `brew install --cask gasparsuarez/tap/repora` and update with
  `brew upgrade --cask repora`.
- A cross-platform `scripts/install.sh` that detects the platform, verifies
  `SHA256SUMS`, and installs or updates `~/.local/bin/repora`.

## [0.1.3] - 2026-11-22

### Fixed

- Automatic refresh now runs on Home, workspace, and repository screens at the
  configured interval.
- Changes no longer remain stale after staging or committing; out-of-order status
  snapshots are discarded.
- Selection is preserved across refresh, and mutating keys are scoped to the active tab.

## [0.1.2] - 2026-11-18

### Changed

- The repository moved to `github.com/gasparsuarez/repora`; the Go module path now matches
  the new owner, so `go install github.com/gasparsuarez/repora/cmd/repora@latest` works.

### Fixed

- Removed a timing race in the per-repository lock test that failed intermittently under
  the race detector.

## [0.1.1] - 2026-11-16

### Added

- In-progress Git operations (cherry-pick, merge, rebase, revert) are shown in the dashboard
  and repository overview.
- Blocking failures now include an actionable hint, for example how to finish or stop a
  cherry-pick.
- The error overlay wraps and scrolls instead of clipping long messages.
- `make install` with a user-local prefix (`~/.local/bin`) and `make uninstall`.

### Fixed

- Installation instructions no longer require `sudo` for `/usr/local/bin` without saying
  so.

## [0.1.0] - 2026-11-16

First public, experimental release.

### Added

- Workspace Explorer with live repository aggregation: per-workspace counts of clean,
  modified, conflicting, and out-of-sync repositories.
- Workspace Dashboard with concurrent status refresh (bounded worker pool) and per-row
  loading, error, and unavailable states.
- Repository View with Overview, Changes, Branches, and History tabs.
- Read-only diff overlay for staged and unstaged changes.
- Commit detail with the commit patch and stat.
- Git operations scoped to the selected repository:
  - stage, stage all, unstage, unstage all and commit
  - create branch, checkout, delete local branch
  - fetch, pull (`--ff-only`), push (sets upstream when needed)
- Workspace management from the UI: create (with optional description), rename, delete,
  add repository by path, remove repository from a workspace.
- Configuration editing: `,` opens the file in `$VISUAL`/`$EDITOR` and reloads on exit;
  `Ctrl+R` reloads from disk.
- Headless status output with `--debug-status <workspace>`.
- Atomic configuration writes with unknown-field preservation.
- Per-repository write locking, input validation, and automatic refresh after writes.
- Build version shown in the application header.

### Security

- Git is executed as an argument vector, never through a shell.
- Git runs non-interactively, so credential prompts cannot hang the interface.
- Destructive actions require confirmation; no force-push, reset, or clean is exposed.
- Configuration files are written with mode `0600` inside a `0700` directory.

### Known limitations

- Single-line commit messages; no amend.
- Diff renders as a full-screen overlay, not a split pane.
- Remote branches are read-only.
- No stash, rebase, cherry-pick, or worktrees.
- macOS and Linux only.

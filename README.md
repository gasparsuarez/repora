<p align="center">
  <img src="assets/branding/logo.png" width="420" alt="Repora">
</p>

<p align="center">
  <strong>One workspace. Every repository. Full control.</strong>
</p>

<p align="center">
  A terminal-native workspace manager for developers working across multiple independent
  Git repositories.
</p>

<p align="center">
  <a href="LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-A3E635.svg"></a>
  <img alt="Go 1.24+" src="https://img.shields.io/badge/go-1.24%2B-60A5FA.svg">
  <img alt="macOS and Linux" src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux-9CA3AF.svg">
</p>

![Workspace Explorer](assets/screenshots/explorer.png)

## Why Repora

Most Git tooling optimizes one repository at a time. Projects rarely look like that:
a backend, a couple of frontends, shared packages and infrastructure, each with its own
branches, remotes and state.

Repora groups those repositories into **workspaces you define** and shows the state of the
whole project in one screen, while every operation still targets exactly one repository.

Workspaces are Repora's own concept. They live in Repora's configuration, not in Git and
not in a directory, so you can keep `work` and `personal` separate, share repositories
between both, and never move code on disk.

## Features

- **Workspace Explorer** with live aggregation: how many repositories are clean, modified,
  conflicting, or out of sync.
- **Workspace Dashboard** with concurrent status refresh and per-repository loading and
  error states.
- **Repository View** with Overview, Changes, Branches and History.
- **Daily Git operations** scoped to the selected repository: stage, unstage, commit,
  create/checkout/delete branch, fetch, pull and push.
- **Diff and commit review** without leaving the terminal.
- **Configuration editing** from inside the app: `,` opens your editor and reloads on exit.
- **Safe by design**: no shell interpolation, no force push, no reset, no clean.

## Screens

<table>
  <tr>
    <td width="50%">
      <img src="assets/screenshots/workspace.png" alt="Workspace Dashboard">
      <p align="center"><sub>Workspace Dashboard — every repository in one place</sub></p>
    </td>
    <td width="50%">
      <img src="assets/screenshots/repository.png" alt="Repository View">
      <p align="center"><sub>Repository View — changes, branches and history</sub></p>
    </td>
  </tr>
</table>

## Install

### Prebuilt binaries

Each release ships compressed archives for macOS and Linux plus `SHA256SUMS`:

```text
repora_<version>_darwin_arm64.tar.gz
repora_<version>_darwin_amd64.tar.gz
repora_<version>_linux_amd64.tar.gz
repora_<version>_linux_arm64.tar.gz
SHA256SUMS
```

Download the archive for your platform, verify its checksum, extract `repora`, and
install it:

```bash
archive=repora_<version>_<os>_<arch>.tar.gz
grep "$archive" SHA256SUMS | shasum -a 256 -c -   # sha256sum -c - on Linux
tar -xzf "$archive"

# User-local install (no sudo). Add ~/.local/bin to PATH if it is not already.
install -m 0755 repora ~/.local/bin/repora

# Or system-wide:
sudo install -m 0755 repora /usr/local/bin/repora
```

### Homebrew (macOS)

Homebrew asks you to trust third-party taps explicitly:

```bash
brew trust gasparsuarez/tap
brew install --cask gasparsuarez/tap/repora
```

Update with:

```bash
brew upgrade --cask repora
```

### Install script (macOS and Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/gasparsuarez/repora/main/scripts/install.sh | sh
```

The script detects the platform, verifies `SHA256SUMS`, and installs to
`~/.local/bin`. Re-run the same command to update. To pin a version or change the
destination:

```bash
curl -fsSL https://raw.githubusercontent.com/gasparsuarez/repora/main/scripts/install.sh |
  sh -s -- --version v0.1.3 --dir ~/.local/bin
```

### From source

```bash
git clone https://github.com/gasparsuarez/repora
cd repora
make install            # installs to ~/.local/bin/repora
```

`make install` writes to `$(PREFIX)`, which defaults to `~/.local/bin`. For a system-wide
install use `sudo make install PREFIX=/usr/local/bin`.

If `repora` is not found afterwards, add `~/.local/bin` to your `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Requires Go 1.24+ and Git 2.30+.

## Quick start

```bash
repora
```

On first run Repora creates `~/.config/repora/config.yaml` with a minimal configuration.
Press `n` to create a workspace, open it, and press `a` to register a repository by path.

```yaml
version: 1

repositories:
  - id: api
    name: API
    path: /Users/you/code/product/api

workspaces:
  - id: product
    name: Product
    description: Customer-facing product
    repositories: [api]

preferences:
  theme: dark
  refresh_interval: 30
  status_workers: 4
```

Repository paths are validated at startup. A missing repository is reported and marked
unavailable; it never prevents the application from starting.

## Keyboard

| Key | Action |
|---|---|
| `↑` `↓` / `j` `k` | Move selection |
| `Enter` | Open workspace, repository, diff or commit |
| `1` – `4` | Repository tabs |
| `Space` | Stage or unstage the selected file |
| `c` / `n` | Commit / new branch |
| `d` | Delete branch (Branches tab) or workspace (Home) |
| `a` / `x` | Add / remove repository |
| `f` `p` `P` | Fetch / pull / push |
| `r` | Refresh |
| `,` | Open config in `$EDITOR` |
| `Ctrl+R` | Reload config |
| `?` | Help |
| `q` | Back one level; quit from Home |

## Documentation

- [Product vision and scope](docs/00-product/vision.md)
- [How workspaces work](docs/00-product/workspaces.md)
- [Architecture and decisions](docs/01-architecture/decisions.md)
- [Interface specification](docs/02-tui/screens.md)

## Project status

`v0.1.2` is the current public, **experimental** release. Multi-repository awareness and
the daily Git loop are in place. Known limitations:

- single-line commit messages; no amend
- diff opens as a full-screen overlay, not a split pane
- remote branches are read-only
- no stash, rebase, cherry-pick or worktrees
- macOS and Linux only

See [CHANGELOG.md](CHANGELOG.md) for details.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) and run `make gate` before opening a pull request.
Security reports go to **gaspar.mdp@gmail.com**, not to a public issue.

## License

[MIT](LICENSE) © 2026 Gaspar Suarez

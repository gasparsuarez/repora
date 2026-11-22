# Keyboard Shortcuts

**Status:** Defined
**Scope:** Global and contextual keybindings for v0.1.

---

## Rules

1. Every bindable action appears in a registry with: key, label, scope, and enabled
   predicate.
2. The action bar and Help screen are generated from the registry.
3. A key has exactly one meaning per scope. The same key may mean different things in
   different scopes only when the context is visible on screen.
4. Destructive actions require a modifier-free key plus a confirmation; they are never
   bound to `Enter` alone.
5. Lowercase and uppercase are distinct bindings.
6. `Ctrl+C` is reserved globally and cannot be rebound in v0.1.

---

## Reserved Keys

| Key | Reserved for |
|---|---|
| `Ctrl+C` | Interrupt then quit |
| `Esc` | Close modal / go back |
| `?` | Help |
| `q` | Quit (Home only) |
| `Enter` | Primary action on selection |
| `/` | Filter/focus input |
| `↑ ↓ ← →` | Navigation |
| `j k h l` | Navigation aliases |
| `g G` | First / last |
| `1 2 3 4` | Tab selection in Repository View |

---

## Global

Available on every screen unless a modal is open.

| Key | Action |
|---|---|
| `Ctrl+C` | Cancel in-flight operations; second press quits |
| `q` | Go back one level; quits from Home |
| `,` | Open the configuration file in `$VISUAL` / `$EDITOR`; reload on exit |
| `Ctrl+R` | Reload the configuration from disk |
| `?` | Toggle Help |
| `Esc` | Close modal, else go back |
| `r` | Refresh current scope |
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `g` | Jump to first |
| `G` | Jump to last |
| `PgUp` / `PgDn` | Page up / down in scrollable content |

`q` navigates back one level from the repository and workspace screens, and quits from
Home. It does nothing while a modal is open.

---

## Home

| Key | Action |
|---|---|
| `Enter` | Open selected workspace |
| `n` | New workspace |
| `e` | Rename selected workspace |
| `d` | Delete selected workspace (confirm) |
| `r` | Refresh workspace summaries |
| `q` | Quit |

---

## Workspace Dashboard

| Key | Action | Notes |
|---|---|---|
| `Enter` | Open selected repository | |
| `f` | Fetch selected repository | |
| `p` | Pull selected repository | |
| `P` | Push selected repository | |
| `R` | Refresh selected repository only | |
| `r` | Refresh all repositories in workspace | |
| `a` | Add repository to workspace | |
| `x` | Remove selected repository from workspace (confirm) | |
| `/` | Filter repositories | |
| `e` | Open error detail for selected row | Enabled only when the row has an error |
| `Esc` | Back to Home | |

Notes:

- `f`, `p`, `P` are disabled for unavailable repositories and for unborn branches.
- `p` is disabled when there is no upstream.
- `P` is disabled for detached HEAD.
- `R` and `r` are always enabled.

---

## Repository View — Common

| Key | Action |
|---|---|
| `1` | Overview tab |
| `2` | Changes tab |
| `3` | Branches tab |
| `4` | History tab |
| `←` / `h` | Previous tab |
| `→` / `l` | Next tab |
| `Tab` | Focus content |
| `f` | Fetch |
| `p` | Pull |
| `P` | Push |
| `r` | Refresh repository |
| `Esc` | Back to Workspace Dashboard |

---

## Repository View — Overview

| Key | Action |
|---|---|
| `Enter` | Open commit detail for selected recent commit |
| `b` | Jump to Branches tab |

---

## Repository View — Changes

| Key | Action | Notes |
|---|---|---|
| `Enter` | Open selected file diff | Full-screen modal when the pane is hidden |
| `Space` | Stage or unstage the selected entry | Staged entries are unstaged |
| `A` | Stage all | |
| `u` | Unstage all | |
| `c` | Commit (modal) | Enabled when staged count > 0 |
| `J` / `K` | Scroll diff down / up | |
| `PgDn` / `PgUp` | Page diff | |

Disabled states are enforced by the enabled predicate and reflected by omission from the
action bar and by a muted appearance in Help.

---

## Repository View — Branches

| Key | Action | Notes |
|---|---|---|
| `Enter` | Checkout selected local branch | Disabled on remote rows |
| `n` | New branch (modal) | |
| `o` | Create pull request (modal) | GitHub remotes only |
| `d` | Delete branch (modal) | Refused for the current branch and unmerged branches |
| `r` | Refresh branch list | |

## Pull Request Form

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` | Next / previous field |
| `←` / `→` | Change origin or base branch |
| `Ctrl+S` | Create pull request |
| `Esc` | Cancel the form or an in-flight request |

---

## Repository View — History

| Key | Action |
|---|---|
| `Enter` | Commit detail |
| `PgDn` / `PgUp` | Scroll |

---

## Modals

| Key | Action |
|---|---|
| `Enter` | Submit / confirm |
| `Esc` | Cancel / dismiss |
| `↑` `↓` / `PgUp` `PgDn` | Scroll an error overlay |
| `Tab` | Move between fields (multi-field modals only) |

Within a text input:

| Key | Action |
|---|---|
| `←` `→` | Move cursor |
| `Home` / `End` | Line start / end |
| `Ctrl+U` | Clear line |
| `Ctrl+W` | Delete word before cursor |
| `Backspace` | Delete character |

---

## Conflict Resolution

Current resolution order:

```text
Ctrl+C → modal → input → screen keys → global keys → ignored
```

When a modal is open, screen and global keys are not dispatched, except `Ctrl+C`.

When an input is focused, `j`/`k` type characters rather than navigating. This is
intentional and documented in the Help screen.

---

## Help Screen Content

Example (Repository View, Changes):

```text
Navigation            Actions
──────────            ───────
↑ ↓ / j k   move      Space   stage file
g G         first/last A      stage all
1-4         tabs      u      unstage all
Tab         focus     c      commit
Esc         back      Enter  view diff
                      f p P  fetch/pull/push
                      r      refresh
```

The Help screen is generated from the registry; nothing is hard-coded.

---

## Future Bindings (Post-MVP)

Reserved, not implemented:

| Key | Planned action |
|---|---|
| `Ctrl+K` | Command palette |
| `s` | Stash manager |
| `:` | Command line |
| `Ctrl+Shift+R` | Restart workspace command |
| `l` | Load more history |

These are documented now to avoid binding conflicts later.

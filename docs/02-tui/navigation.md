# Navigation

**Status:** Defined
**Scope:** Screen stack, focus model, modal semantics, and state preservation.

---

## Screen Graph

```text
Home (Workspace Explorer)
  │
  └── Workspace Dashboard
        │
        └── Repository View
              ├── Overview (default)
              ├── Changes
              ├── Branches
              └── History
```

Rules:

- The graph is a linear stack. No screen is reachable from two different parents in v0.1.
- `Esc` pops exactly one level.
- `Enter` on a selectable row pushes the next level.
- There is no global menu. Navigation happens through the stack or, later, the command
  palette.

---

## Screen Stack

```go
type Screen int

const (
    ScreenHome Screen = iota
    ScreenWorkspace
    ScreenRepository
)

type Route struct {
    Screen       Screen
    WorkspaceID  domain.WorkspaceID
    RepositoryID domain.RepositoryID // empty on Home and Workspace
    Tab          RepoTab             // only on Repository
}
```

- The stack is `[]Route`.
- The top of the stack is the rendered screen; lower entries keep their state.
- Pushing a route records the origin so `Esc` can restore selection and scroll.
- The stack is bounded in practice at 3 entries; no artificial limit is enforced.

### Back behavior

| From | `Esc` goes to | Restores |
|---|---|---|
| Repository tab | Repository overview tab | Same repository, tab reset to Overview |
| Repository overview | Workspace dashboard | Selection on that repository row |
| Workspace dashboard | Home | Selection on that workspace |
| Home | Quit confirmation | — |

Exception: if a modal is open, `Esc` closes the modal and does not pop the screen.

### Forward behavior

- Entering a repository from the dashboard always lands on Overview.
- Switching tabs does not push a new route; it mutates the current route's `Tab`.
- Re-entering a workspace refreshes it if the data is older than the configured interval.

---

## Focus Model

Screens contain at most three focusable regions:

| Screen | Regions |
|---|---|
| Home | Search input (optional), workspace list |
| Workspace Dashboard | Filter input (optional), repository list |
| Repository View | Tab bar, content area |

Rules:

- Exactly one region has focus at a time.
- Focus is shown by a marker in the region header or a highlighted input border.
- `Tab` / `Shift+Tab` cycle focusable regions.
- When a region holds focus, its keys take precedence over global navigation keys, except
  `Ctrl+C` and `Esc`.
- Search and filter inputs are entered explicitly (`/`) and exited with `Esc`, which
  clears the filter.

---

## Modal Stack

```go
type Modal interface {
    Update(msg tea.Msg) (Modal, tea.Cmd)
    View(width, height int) string
    Dismissable() bool
}
```

Modals in v0.1:

| Modal | Trigger | Dismiss | Blocks navigation |
|---|---|---|---|
| Confirm | Destructive action | `Esc`, `Enter` | Yes |
| Text prompt | Branch name, commit message | `Esc` | Yes |
| Error detail | `e` on an error row | `Esc` | Yes |
| Help | `?` | `Esc`, `?` | Yes |

Rules:

- Modals stack but never more than two deep (for example: prompt → confirm).
- Input modals keep their content until dismissed, including after a failed operation, so
  the user can retry.
- A modal that starts an operation stays open with a spinner until the result arrives.
- Global quit (`q`) is disabled while a modal is open; `Ctrl+C` still works.

---

## State Preservation

| State | Preserved when leaving | Preserved on refresh |
|---|---|---|
| Workspace selection | Yes | Yes, if the workspace still exists |
| Repository selection | Yes | Yes, if the repository still exists |
| Scroll offset | Yes | Clamped to content |
| Active tab | No (reset to Overview) | Yes |
| Filter text | No (cleared) | Yes |
| Pending input in modal | Yes, until dismissed | N/A |

Refresh MUST NOT change selection. If the selected repository disappears (removed from the
workspace or configuration reload), selection moves to the nearest remaining row.

---

## Refresh Triggers

| Event | Refresh scope |
|---|---|
| App start, Home shown | All workspace summaries |
| Home shown after returning | Merged dashboard states, then the timer refreshes summaries |
| Workspace opened | All repositories in the workspace |
| Repository opened | That repository and its detail sections |
| Manual `r` | Current scope: workspace or repository |
| After a write operation | The affected repository, before input is unlocked |
| Timer (if `refresh_interval > 0`) | The visible screen: Home, workspace, or repository |
| Config reload | Everything; the timer is re-armed with the new interval |

Rules:

- Automatic refresh never runs while a prompt, confirmation, help, or overlay is open.
- Automatic refresh never runs while a write is in flight or a refresh is already running.
- Automatic refresh never changes the selection.
- Background refresh is silent when a snapshot already exists: it does not replace loaded
  rows with a spinner.
- Automatic refresh is suspended when the terminal loses focus (optional; see below).

---

## Breadcrumbs

Every screen below Home shows a breadcrumb in the header:

```text
WORKSPACES / PRODUCT X / api / CHANGES
```

- Segments are separated by ` / `.
- Ancestor segments are `TextSecondary`; the current segment is `TextPrimary`.
- An ancestor segment MAY be clickable in a future version; in v0.1 it is display only.
- Truncation removes ancestor segments from the left, keeping the last two.

---

## Quit Semantics

- `q` on Home quits.
- `q` on other screens does nothing (it is a contextual key), to prevent accidental exit.
- `Ctrl+C`: first press cancels in-flight operations and shows a notice; a second press
  within 2 seconds quits.
- Quitting with a running write operation requires confirmation.
- Quitting with an unsaved modal input requires confirmation.

---

## Key Routing Order

```text
1. Ctrl+C                      (global, always)
2. Active modal                (if any)
3. Active input field          (if focused)
4. Screen-specific keys
5. Global navigation keys      (Esc, ?, q)
6. Ignored
```

Documented in [`shortcuts.md`](shortcuts.md) with the reserved key list.

---

## Testing

- Stack transitions: push and pop from every level, including with modals open.
- Selection restoration after back navigation.
- Refresh does not alter selection.
- Selection fallback when the selected item disappears.
- Quit confirmation paths.
- Key routing precedence for each combination.

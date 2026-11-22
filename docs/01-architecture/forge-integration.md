# Forge Integration

**Status:** Defined
**Scope:** Pull request creation in v0.2. GitHub is the only provider; the port leaves room
for others later.

See D-023 in [`decisions.md`](decisions.md).

---

## Flow

```text
Branches tab (`o`)
  → TUI pull request form
  → app.PullRequestService.Create
      1. validate title, head, base
      2. resolve the selected remote
      3. require refs/remotes/<origin>/<base>
      4. require refs/remotes/<origin>/<head>
      5. if a local head exists, require the same OID
  → infra/github.Client.CreatePullRequest
      → gh api --method POST repos/<owner>/<repo>/pulls --hostname <host> --input -
  → pullRequestDoneMsg
```

---

## Rules

- No credentials are stored in configuration. Authentication is owned by `gh`.
- No provider call runs during startup, navigation, or refresh. Creation happens only after
  an explicit submit.
- Pushing the head branch is a separate, explicit action (`P` in Branches). Repora never
  pushes implicitly while creating a pull request.
- The source and base branch must exist on the selected remote.
- If a local branch exists, its HEAD must equal the remote-tracking branch; otherwise the
  user is told to push or pull first.
- Only same-repository pull requests are supported in v0.2. Forks and PR listing are out of
  scope.
- A failed request keeps the form and its input; a successful request shows the PR number
  and URL.

---

## Errors

| Condition | Error |
|---|---|
| `gh` missing | `ErrForgeUnavailable` |
| `gh` not authenticated | `ErrForgeAuth` |
| GitHub rejects the request | `ErrForgeRejected` |
| Remote is not a GitHub-style URL | `ErrUnsupportedRemote` |
| Head is missing on the remote | `ErrBranchNotPushed` |
| Base is missing on the remote | `ErrBaseBranchNotFound` |
| Title, head, or base invalid | `ErrInvalidPullRequest` |

The original `gh` output is preserved in the error and shown in the form; `e` remains the
full error overlay for Git failures.

---

## Testing

- Unit: remote URL parser, `gh` runner arguments and JSON, error mapping.
- Application: fake Git refs and fake forge; head/base/remote validation.
- TUI: form defaults, selectors, submit, error preservation, success notice, paused refresh,
  golden rendering at 80x24 and 120x40.
- No tests call GitHub.

package domain

import "errors"

var (
	// ErrConfigNotFound is returned when the configuration file does not exist
	// and cannot be created.
	ErrConfigNotFound = errors.New("configuration file not found")

	// ErrUnsupportedVersion is returned when the configuration declares a schema
	// version this build does not understand.
	ErrUnsupportedVersion = errors.New("unsupported configuration version")

	// ErrDuplicateID is returned when two entities share an identifier.
	ErrDuplicateID = errors.New("duplicate id")

	// ErrDanglingReference is returned when a workspace references a repository
	// that is not registered.
	ErrDanglingReference = errors.New("dangling repository reference")

	// ErrInvalidName is returned when a display name is empty or too long.
	ErrInvalidName = errors.New("invalid name")

	// ErrDuplicatePath is returned when two repositories share a resolved path.
	ErrDuplicatePath = errors.New("duplicate repository path")

	// ErrInvalidPath is returned when a repository path is unusable.
	ErrInvalidPath = errors.New("invalid repository path")

	// ErrNotARepository is returned when a path is not a Git working tree.
	ErrNotARepository = errors.New("not a git repository")

	// ErrBareRepository is returned when a path points to a bare repository.
	ErrBareRepository = errors.New("bare repositories are not supported")

	// ErrGitNotFound is returned when the git executable is unavailable.
	ErrGitNotFound = errors.New("git executable not found")

	// ErrGitTooOld is returned when the installed git is below the minimum.
	ErrGitTooOld = errors.New("git version is too old")

	// ErrNoUpstream is returned when an operation requires an upstream branch.
	ErrNoUpstream = errors.New("branch has no upstream")

	// ErrInvalidBranch is returned when a branch name is not acceptable.
	ErrInvalidBranch = errors.New("invalid branch name")

	// ErrEmptyMessage is returned when a commit message is blank.
	ErrEmptyMessage = errors.New("commit message is empty")

	// ErrRepositoryNotFound is returned when an identifier does not resolve.
	ErrRepositoryNotFound = errors.New("repository not found")

	// ErrWorkspaceNotFound is returned when an identifier does not resolve.
	ErrWorkspaceNotFound = errors.New("workspace not found")

	// ErrUnavailable is returned when a repository cannot be inspected because
	// its path is missing or unreadable.
	ErrUnavailable = errors.New("repository unavailable")

	// ErrDetachedHead is returned when an operation requires an attached branch.
	ErrDetachedHead = errors.New("HEAD is detached")

	// ErrNoRemote is returned when an operation requires a configured remote.
	ErrNoRemote = errors.New("no remote configured")

	// ErrRemoteNotFound is returned when a named remote is not configured.
	ErrRemoteNotFound = errors.New("remote not found")

	// ErrNothingToStage is returned when a stage request has no paths.
	ErrNothingToStage = errors.New("nothing to stage")

	// ErrCurrentBranch is returned when trying to delete the checked-out branch.
	ErrCurrentBranch = errors.New("cannot delete the current branch")

	// ErrInvalidPullRequest is returned when a pull request request is incomplete
	// or inconsistent.
	ErrInvalidPullRequest = errors.New("invalid pull request")

	// ErrBranchNotPushed is returned when the head branch is not available on the
	// selected remote at the local commit.
	ErrBranchNotPushed = errors.New("branch is not pushed to the remote")

	// ErrBaseBranchNotFound is returned when the base branch is not present on
	// the selected remote.
	ErrBaseBranchNotFound = errors.New("base branch not found on the remote")

	// ErrUnsupportedRemote is returned when a remote URL is not a supported
	// hosting-provider URL.
	ErrUnsupportedRemote = errors.New("unsupported remote URL")

	// ErrForgeUnavailable is returned when the hosting-provider CLI is missing.
	ErrForgeUnavailable = errors.New("forge CLI is unavailable")

	// ErrForgeAuth is returned when the hosting-provider CLI is not authenticated.
	ErrForgeAuth = errors.New("forge authentication is required")

	// ErrForgeRejected is returned when the hosting provider rejects the request.
	ErrForgeRejected = errors.New("forge rejected the request")
)

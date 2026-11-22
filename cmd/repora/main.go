package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gasparsuarez/repora/internal/app"
	"github.com/gasparsuarez/repora/internal/domain"
	"github.com/gasparsuarez/repora/internal/infra/config"
	"github.com/gasparsuarez/repora/internal/infra/gitcli"
	githubforge "github.com/gasparsuarez/repora/internal/infra/github"
	"github.com/gasparsuarez/repora/internal/tui"
)

// Build metadata, injected with -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const (
	exitOK      = 0
	exitRuntime = 1
	exitUsage   = 2
	exitConfig  = 3
	exitGit     = 4
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	flags := flag.NewFlagSet("repora", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	configFlag := flags.String("config", "", "path to config.yaml (default ~/.config/repora/config.yaml)")
	showVersion := flags.Bool("version", false, "print version and exit")
	noColor := flags.Bool("no-color", false, "disable color output")
	debugStatus := flags.String("debug-status", "", "print the status of a workspace and exit")

	flags.Usage = func() {
		out := flags.Output()
		fmt.Fprintln(out, "Repora — multi-repository Git workspace TUI")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Usage:")
		fmt.Fprintln(out, "  repora [flags]")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Flags:")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}

	if *showVersion {
		fmt.Printf("repora %s (commit %s, built %s)\n", version, commit, date)
		return exitOK
	}

	configPath, err := config.ResolvePath(*configFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "repora: resolve config path: %v\n", err)
		return exitConfig
	}

	cfg, warnings, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "repora: load configuration: %v\n", err)
		fmt.Fprintf(os.Stderr, "        file: %s\n", configPath)
		return exitConfig
	}

	executor := gitcli.NewExecutor()

	versionCtx, versionCancel := context.WithTimeout(context.Background(), 10*time.Second)
	gitVersion, err := executor.Version(versionCtx)
	versionCancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "repora: %v\n", err)
		fmt.Fprintln(os.Stderr, "        install git (>= "+gitcli.MinGitVersion+") and ensure it is on PATH")
		return exitGit
	}
	if err := gitcli.CheckVersion(gitVersion); err != nil {
		fmt.Fprintf(os.Stderr, "repora: %v\n", err)
		return exitGit
	}

	gitRepo := gitcli.NewRepository(executor)
	warnings = append(warnings, resolveRepositoryRoots(cfg, gitRepo)...)

	store := config.NewFileStore(configPath)
	status := app.NewStatusService(gitRepo, cfg.Preferences.StatusWorkers)
	repositories := app.NewRepositoryService(gitRepo, store)
	gitService := app.NewGitService(gitRepo, app.NewLocks())
	pullRequests := app.NewPullRequestService(gitRepo, githubforge.NewClient(nil))
	workspaces := app.NewWorkspaceService(store)

	if *debugStatus != "" {
		return printWorkspaceStatus(cfg, status, *debugStatus)
	}

	// reload re-reads the configuration without restarting the TUI.
	reload := func() (*domain.Config, []string, *app.StatusService, error) {
		reloaded, reloadWarnings, err := config.Load(configPath)
		if err != nil {
			return nil, nil, nil, err
		}
		reloadWarnings = append(reloadWarnings, resolveRepositoryRoots(reloaded, gitRepo)...)
		service := app.NewStatusService(gitRepo, reloaded.Preferences.StatusWorkers)
		return reloaded, reloadWarnings, service, nil
	}

	model := tui.New(tui.Deps{
		Config:       cfg,
		ConfigPath:   configPath,
		Version:      version,
		Status:       status,
		Repos:        repositories,
		Git:          gitService,
		PullRequests: pullRequests,
		Workspaces:   workspaces,
		Warnings:     warnings,
		NoColor:      *noColor,
		Reload:       reload,
	})
	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "repora: tui: %v\n", err)
		return exitRuntime
	}
	return exitOK
}

// resolveRepositoryRoots canonicalizes registered repository paths in memory and
// reports unusable ones. Configuration is not rewritten: saving paths is a
// separate, explicit operation.
func resolveRepositoryRoots(cfg *domain.Config, git *gitcli.Repository) []string {
	var warnings []string

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i := range cfg.Repositories {
		repo := &cfg.Repositories[i]
		root, err := git.Root(ctx, repo.Path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("repository %q: %v", repo.ID, err))
			continue
		}
		repo.Path = root
	}
	return warnings
}

func printWorkspaceStatus(cfg *domain.Config, status *app.StatusService, workspaceID string) int {
	ws, ok := cfg.WorkspaceByID(domain.WorkspaceID(workspaceID))
	if !ok {
		fmt.Fprintf(os.Stderr, "repora: workspace not found: %q\n", workspaceID)
		if len(cfg.Workspaces) > 0 {
			ids := make([]string, 0, len(cfg.Workspaces))
			for _, w := range cfg.Workspaces {
				ids = append(ids, string(w.ID))
			}
			fmt.Fprintf(os.Stderr, "        available: %s\n", strings.Join(ids, ", "))
		}
		return exitRuntime
	}

	repos := cfg.RepositoriesOf(ws)
	fmt.Printf("repora — %s (%s)\n\n", ws.Name, plural(len(repos), "repository", "repositories"))

	if len(repos) == 0 {
		fmt.Println("  this workspace has no repositories")
		return exitOK
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	states := make(map[domain.RepositoryID]domain.RepositoryState, len(repos))
	status.Refresh(ctx, repos, func(state domain.RepositoryState) {
		states[state.RepositoryID] = state
	})

	for _, repo := range repos {
		state, ok := states[repo.ID]
		if !ok {
			fmt.Printf("  %-20s %-24s %s\n", repo.Name, "", "error (no result)")
			continue
		}
		fmt.Printf("  %-20s %-24s %-12s %s\n",
			repo.Name,
			state.Branch.Label(),
			state.Class(),
			statusDetail(state),
		)
	}
	return exitOK
}

func statusDetail(state domain.RepositoryState) string {
	if state.Err != nil {
		return state.Err.Error()
	}

	var parts []string
	if state.Operation != domain.OperationNone {
		parts = append(parts, "⏸ "+string(state.Operation))
	}
	if n := state.StagedCount(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d staged", n))
	}
	if n := state.UnstagedCount(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d unstaged", n))
	}
	if n := state.UntrackedCount(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d untracked", n))
	}
	if n := state.ConflictCount(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d conflicts", n))
	}
	if n := state.Branch.Ahead; n > 0 {
		parts = append(parts, fmt.Sprintf("%d ahead", n))
	}
	if n := state.Branch.Behind; n > 0 {
		parts = append(parts, fmt.Sprintf("%d behind", n))
	}
	if len(parts) == 0 {
		parts = append(parts, "up to date")
	}
	return strings.Join(parts, " · ")
}

func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, pluralForm)
}

package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/gasparsuarez/repora/internal/domain"
	"github.com/gasparsuarez/repora/internal/tui/components"
)

func (m *Model) renderRepository() string {
	if m.diffOpen {
		return m.renderDiffOverlay()
	}
	if m.commitOpen {
		return m.renderCommitOverlay()
	}

	repo, ok := m.currentRepo()
	if !ok {
		return m.frame("  "+m.st.Dim.Render("repository unavailable"), [][2]string{{"Esc", "Back"}})
	}

	var b strings.Builder
	b.WriteString(m.brandHeader())
	b.WriteString("\n" + components.Rule(m.st, m.contentWidth()) + "\n")
	b.WriteString(m.repositoryHeader(repo))
	b.WriteString("\n")
	b.WriteString(m.st.Bar.Render(truncate(shortenPath(repo.Path), m.contentWidth())))

	headerLines := 4
	panelLines := m.height - 3 - contentMarginTop - headerLines
	if panelLines < 6 {
		panelLines = 6
	}

	b.WriteString("\n" + m.repoPanel(panelLines))

	return m.frame(b.String(), m.repositoryActions())
}

// repoPanel renders the tab bar and the active tab inside the shared surface.
func (m *Model) repoPanel(panelLines int) string {
	inner := panelLines - 2
	bodyBudget := inner - 2 // tab bar plus a blank line
	if bodyBudget < 1 {
		bodyBudget = 1
	}

	body := m.repoTabBody(bodyBudget)
	return m.renderPanel(m.tabBar()+"\n\n"+body, panelLines)
}

func (m *Model) repoTabBody(budget int) string {
	switch m.repoTab {
	case tabChanges:
		return m.renderChangesTab(budget)
	case tabBranches:
		return m.renderBranchesTab(budget)
	case tabHistory:
		return m.renderHistoryTab(budget)
	default:
		return m.renderOverview(budget)
	}
}

func (m *Model) repositoryActions() [][2]string {
	var actions [][2]string

	switch m.repoTab {
	case tabChanges:
		actions = [][2]string{
			{"Space", "Stage"},
			{"c", "Commit"},
			{"A", "Stage all"},
			{"u", "Unstage"},
			{"Enter", "Diff"},
		}
	case tabBranches:
		actions = [][2]string{
			{"n", "New"},
			{"Enter", "Switch"},
			{"o", "PR"},
			{"d", "Del"},
			{"f", "Fetch"},
			{"p", "Pull"},
			{"P", "Push"},
		}
	case tabHistory:
		actions = [][2]string{
			{"Enter", "Details"},
			{"f", "Fetch"},
			{"p", "Pull"},
			{"P", "Push"},
		}
	default:
		actions = [][2]string{
			{"f", "Fetch"},
			{"p", "Pull"},
			{"P", "Push"},
			{"b", "Branches"},
		}
	}

	return append(actions, [2]string{"Esc", "Back"})
}

func (m *Model) repositoryHeader(repo domain.Repository) string {
	ws := ""
	if m.wsIndex >= 0 && m.wsIndex < len(m.cfg.Workspaces) {
		ws = m.cfg.Workspaces[m.wsIndex].Name
	}

	state := ""
	if row := m.currentRow(); row != nil {
		state = m.stateText(*row)
	}

	return components.Breadcrumb(m.st, []string{"Workspaces", ws, repo.Name}, m.contentWidth(), state)
}

func (m *Model) tabBar() string {
	parts := make([]string, 0, len(repoTabNames))
	for i, name := range repoTabNames {
		label := name
		if repoTab(i) == tabChanges && len(m.changes) > 0 {
			label = fmt.Sprintf("%s (%d)", name, len(m.changes))
		}
		if repoTab(i) == m.repoTab {
			parts = append(parts, m.st.Accent.Render(label))
			continue
		}
		parts = append(parts, m.st.Dim.Render(label))
	}
	return strings.Join(parts, "   ")
}

func (m *Model) renderOverview(budget int) string {
	var b strings.Builder
	row := m.currentRow()

	b.WriteString(m.section("Status"))

	branch := "(unknown)"
	upstream := ""
	sync := "up to date"
	if row != nil {
		branch = row.state.Branch.Label()
		if row.state.Branch.HasUpstream {
			upstream = " → " + row.state.Branch.Upstream
		}
		switch {
		case row.state.Branch.Ahead > 0 && row.state.Branch.Behind > 0:
			sync = fmt.Sprintf("↑%d ahead · ↓%d behind", row.state.Branch.Ahead, row.state.Branch.Behind)
		case row.state.Branch.Ahead > 0:
			sync = fmt.Sprintf("↑%d ahead", row.state.Branch.Ahead)
		case row.state.Branch.Behind > 0:
			sync = fmt.Sprintf("↓%d behind", row.state.Branch.Behind)
		}
		b.WriteString(m.kv("Branch", branch+upstream))
		b.WriteString(m.kv("Sync", sync))
		b.WriteString(m.kv("Working tree", workingTreeSummary(row.state)))
		conflicts := "none"
		if n := row.state.ConflictCount(); n > 0 {
			conflicts = fmt.Sprintf("%d", n)
		}
		b.WriteString(m.kv("Conflicts", conflicts))
		if row.state.Operation != domain.OperationNone {
			b.WriteString(m.kv("Operation", string(row.state.Operation)+" in progress"))
		}
	}

	b.WriteString("\n")

	b.WriteString(m.section("Remotes"))
	if m.detailLoading && len(m.detail.Remotes) == 0 {
		b.WriteString("  " + m.st.Dim.Render("◐ loading…") + "\n")
	} else if m.detail.RemoteErr != nil {
		b.WriteString("  " + m.st.Danger.Render(m.detail.RemoteErr.Error()) + "\n")
	} else if len(m.detail.Remotes) == 0 {
		b.WriteString("  " + m.st.Dim.Render("none configured") + "\n")
	} else {
		remotes := m.detail.Remotes
		if len(remotes) > 2 {
			remotes = remotes[:2]
		}
		for _, remote := range remotes {
			b.WriteString("  " + m.st.Header.Render(padRight(remote.Name, 10)) +
				m.st.Dim.Render(truncate(remote.FetchURL, max(10, m.contentWidth()-22))) + "\n")
		}
	}
	b.WriteString("\n")

	b.WriteString(m.section("Recent commits"))
	// Status block (5) plus three blank lines, three section titles, and one
	// line for the remotes block: ten lines before commit rows.
	extra := 0
	if row != nil && row.state.Operation != domain.OperationNone {
		extra = 1
	}
	k := budget - 10 - extra
	if k < 1 {
		k = 1
	}
	if k > 3 {
		k = 3
	}
	commits := m.detail.Commits
	if len(commits) > k {
		commits = commits[:k]
	}
	switch {
	case m.detailLoading && len(commits) == 0:
		b.WriteString("  " + m.st.Dim.Render("◐ loading…") + "\n")
	case m.detail.LogErr != nil:
		b.WriteString("  " + m.st.Danger.Render(m.detail.LogErr.Error()) + "\n")
	case len(commits) == 0:
		b.WriteString("  " + m.st.Dim.Render("No commits yet.") + "\n")
	default:
		for _, commit := range commits {
			b.WriteString("  " + m.commitLine(commit) + "\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) renderChangesTab(budget int) string {
	if len(m.changes) == 0 {
		return m.st.Dim.Render("Working tree clean.")
	}

	capacity := budget - 1 // reserve the "more" hint
	if capacity < 1 {
		capacity = 1
	}
	if len(m.changes) > capacity {
		capacity -= 4 // up to four section headers
	}
	if capacity < 1 {
		capacity = 1
	}

	start, end := sliceWindow(m.changeIndex, len(m.changes), capacity)

	counts := make(map[string]int, 4)
	for _, item := range m.changes {
		counts[item.Section]++
	}

	var b strings.Builder
	current := ""
	for i := start; i < end; i++ {
		item := m.changes[i]
		if item.Section != current {
			if current != "" {
				b.WriteString("\n")
			}
			current = item.Section
			b.WriteString(" " + m.st.Bar.Render(fmt.Sprintf("%s (%d)", strings.ToUpper(item.Section), counts[item.Section])) + "\n")
		}

		cursor := "  "
		style := m.st.Header
		if i == m.changeIndex {
			cursor = m.st.Accent.Render("> ")
			style = m.st.Selected
		}
		b.WriteString(cursor + m.codeStyle(item.Code).Render(fmt.Sprintf("%-2s", string(item.Code))) + " " +
			style.Render(truncate(item.Path, max(10, m.contentWidth()-12))) + "\n")
	}

	if end < len(m.changes) {
		b.WriteString(m.st.Bar.Render(fmt.Sprintf("↓ %d more", len(m.changes)-end)))
	}

	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) renderBranchesTab(budget int) string {
	if m.detailLoading && len(m.detail.Branches) == 0 {
		return m.st.Dim.Render("◐ loading…")
	}
	if m.detail.BranchErr != nil {
		return m.st.Danger.Render(m.detail.BranchErr.Error())
	}
	if len(m.detail.Branches) == 0 {
		return m.st.Dim.Render("No branches.")
	}

	capacity := budget - 1 - 2 // hint plus Local/Remote headers
	if capacity < 1 {
		capacity = 1
	}
	start, end := sliceWindow(m.branchIndex, len(m.detail.Branches), capacity)

	var b strings.Builder
	printedLocal, printedRemote := false, false

	for i := start; i < end; i++ {
		branch := m.detail.Branches[i]

		if !branch.IsRemote && !printedLocal {
			b.WriteString(m.section("Local"))
			printedLocal = true
		}
		if branch.IsRemote && !printedRemote {
			if printedLocal {
				b.WriteString("\n")
			}
			b.WriteString(m.section("Remote"))
			printedRemote = true
		}

		cursor := "  "
		style := m.st.Header
		if i == m.branchIndex {
			cursor = m.st.Accent.Render("> ")
			style = m.st.Selected
		}

		marker := " "
		if branch.IsCurrent {
			marker = m.st.Success.Render("✓")
		}

		line := cursor + marker + " " + style.Render(padRight(truncate(branch.Name, 28), 28))

		track := ""
		switch {
		case branch.Gone:
			track = m.st.Danger.Render("[gone]")
		case branch.Ahead > 0 && branch.Behind > 0:
			track = m.st.Warning.Render(fmt.Sprintf("↑%d ↓%d", branch.Ahead, branch.Behind))
		case branch.Ahead > 0:
			track = m.st.Warning.Render(fmt.Sprintf("↑%d", branch.Ahead))
		case branch.Behind > 0:
			track = m.st.Info.Render(fmt.Sprintf("↓%d", branch.Behind))
		case branch.HasUpstream && !branch.IsRemote:
			track = m.st.Dim.Render("up to date")
		}
		if track != "" {
			line += "  " + track
		}

		if subject := branch.LastCommit.Subject; subject != "" && m.contentWidth() >= 110 {
			line += "  " + m.st.Dim.Render(truncate(subject, max(0, m.contentWidth()-64)))
		}

		b.WriteString(line + "\n")
	}

	if end < len(m.detail.Branches) {
		b.WriteString(m.st.Bar.Render(fmt.Sprintf("↓ %d more", len(m.detail.Branches)-end)))
	}

	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) renderHistoryTab(budget int) string {
	if m.detailLoading && len(m.detail.Commits) == 0 {
		return m.st.Dim.Render("◐ loading…")
	}
	if m.detail.LogErr != nil {
		return m.st.Danger.Render(m.detail.LogErr.Error())
	}
	if len(m.detail.Commits) == 0 {
		return m.st.Dim.Render("No commits yet.")
	}

	capacity := (budget - 1) / 2
	if capacity < 1 {
		capacity = 1
	}
	start, end := sliceWindow(m.historyIndex, len(m.detail.Commits), capacity)

	var b strings.Builder
	for i := start; i < end; i++ {
		commit := m.detail.Commits[i]

		cursor := "  "
		style := m.st.Header
		if i == m.historyIndex {
			cursor = m.st.Accent.Render("> ")
			style = m.st.Selected
		}

		subjectWidth := max(10, m.contentWidth()-14)
		b.WriteString(cursor + m.st.Dim.Render(commit.Short) + "  " +
			style.Render(truncate(commit.Subject, subjectWidth)) + "\n")

		meta := commit.Author
		if rel := relativeTime(commit.Date); rel != "" {
			meta += " · " + rel
		}
		if len(commit.Refs) > 0 {
			meta = strings.Join(commit.Refs, ", ") + " · " + meta
		}
		b.WriteString("    " + m.st.Bar.Render(truncate(meta, max(10, m.contentWidth()-8))) + "\n")
	}

	if end < len(m.detail.Commits) {
		b.WriteString(m.st.Bar.Render(fmt.Sprintf("↓ %d more", len(m.detail.Commits)-end)))
	}

	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) renderDiffOverlay() string {
	var b strings.Builder

	title := m.diffTitle
	if m.diffLoading {
		title = "loading diff…"
	}
	b.WriteString(m.st.Title.Render("Diff · " + title))
	b.WriteString("\n")
	b.WriteString(m.st.Dim.Render(strings.Repeat("─", min(m.contentWidth()-2, 60))))
	b.WriteString("\n\n")

	if m.diffErr != nil {
		b.WriteString("  " + m.st.Danger.Render(m.diffErr.Error()) + "\n")
		return m.frame(b.String(), [][2]string{{"Esc", "Close"}})
	}

	lines := strings.Split(strings.TrimRight(m.diffContent, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		b.WriteString("  " + m.st.Dim.Render("No changes for this file.") + "\n")
		return m.frame(b.String(), [][2]string{{"Esc", "Close"}})
	}

	page := max(1, m.height-8)
	start := m.diffScroll
	if start > len(lines) {
		start = len(lines)
	}
	end := min(len(lines), start+page)

	for _, line := range lines[start:end] {
		b.WriteString(m.diffLine(line) + "\n")
	}
	if end < len(lines) {
		b.WriteString(m.st.Dim.Render(fmt.Sprintf("… %d more lines", len(lines)-end)) + "\n")
	}

	return m.frame(b.String(), [][2]string{
		{"↑↓", "Scroll"},
		{"Esc", "Close"},
	})
}

func (m *Model) renderCommitOverlay() string {
	if m.historyIndex < 0 || m.historyIndex >= len(m.detail.Commits) {
		m.commitOpen = false
		return m.renderRepository()
	}
	commit := m.detail.Commits[m.historyIndex]

	var b strings.Builder
	b.WriteString(m.st.Title.Render("Commit " + commit.Short))
	b.WriteString("\n\n")
	b.WriteString(m.kv("Hash", commit.Hash))
	b.WriteString(m.kv("Subject", commit.Subject))
	b.WriteString(m.kv("Author", commit.Author+" <"+commit.Email+">"))
	if !commit.Date.IsZero() {
		b.WriteString(m.kv("Date", commit.Date.Format("2006-01-02 15:04 -0700")))
	}
	if len(commit.Refs) > 0 {
		b.WriteString(m.kv("Refs", strings.Join(commit.Refs, ", ")))
	}
	b.WriteString("\n")

	switch {
	case m.commitLoading:
		b.WriteString(" " + m.st.Dim.Render("◐ loading patch…") + "\n")
	case m.commitErr != nil:
		b.WriteString(" " + m.st.Danger.Render(m.commitErr.Error()) + "\n")
	default:
		lines := strings.Split(strings.TrimRight(m.commitContent, "\n"), "\n")
		if len(lines) == 1 && lines[0] == "" {
			b.WriteString(" " + m.st.Dim.Render("No patch available.") + "\n")
			break
		}

		page := max(1, m.height-14)
		start := m.commitScroll
		if start > len(lines) {
			start = len(lines)
		}
		stop := min(len(lines), start+page)

		for _, line := range lines[start:stop] {
			b.WriteString(m.diffLine(line) + "\n")
		}
		if stop < len(lines) {
			b.WriteString(m.st.Dim.Render(fmt.Sprintf("… %d more lines", len(lines)-stop)) + "\n")
		}
	}

	return m.frame(b.String(), [][2]string{
		{"↑↓", "Scroll"},
		{"Esc", "Close"},
	})
}

func (m *Model) diffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
		return m.st.Dim.Render(line)
	case strings.HasPrefix(line, "+"):
		return m.st.Success.Render(line)
	case strings.HasPrefix(line, "-"):
		return m.st.Danger.Render(line)
	case strings.HasPrefix(line, "@@"):
		return m.st.Info.Render(line)
	case strings.HasPrefix(line, "diff "), strings.HasPrefix(line, "index "):
		return m.st.Header.Render(line)
	default:
		return line
	}
}

func (m *Model) codeStyle(code domain.StatusCode) lipgloss.Style {
	switch code {
	case domain.CodeAdded, "?":
		return m.st.Success
	case domain.CodeDeleted:
		return m.st.Danger
	case domain.CodeRenamed, domain.CodeCopied:
		return m.st.Info
	case domain.CodeModified, domain.CodeTypeChange:
		return m.st.Warning
	default:
		return m.st.Dim
	}
}

func (m *Model) section(title string) string {
	return " " + m.st.Bar.Render(strings.ToUpper(title)) + "\n"
}

func (m *Model) kv(label, value string) string {
	return "  " + m.st.Dim.Render(padRight(label, 14)) + m.st.Header.Render(value) + "\n"
}

func (m *Model) commitLine(commit domain.Commit) string {
	meta := commit.Author
	if rel := relativeTime(commit.Date); rel != "" {
		meta += " · " + rel
	}
	return m.st.Dim.Render(commit.Short) + "  " +
		m.st.Header.Render(truncate(commit.Subject, max(10, m.contentWidth()-42))) + "  " +
		m.st.Dim.Render(truncate(meta, 26))
}

func workingTreeSummary(state domain.RepositoryState) string {
	var parts []string
	if n := state.StagedCount(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d staged", n))
	}
	if n := state.UnstagedCount(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d unstaged", n))
	}
	if n := state.UntrackedCount(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d untracked", n))
	}
	if len(parts) == 0 {
		return "clean"
	}
	return strings.Join(parts, " · ")
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/(24*365)))
	}
}

func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (m *Model) renderErrorOverlay() string {
	width := max(20, m.contentWidth()-4)
	lines := strings.Split(m.errWrapped(), "\n")
	page := max(1, m.height-8)

	start := m.errScroll
	if start > len(lines) {
		start = len(lines)
	}
	stop := min(len(lines), start+page)

	var b strings.Builder
	b.WriteString(m.st.Danger.Render("ERROR"))
	b.WriteString("\n\n")
	for _, line := range lines[start:stop] {
		b.WriteString("  " + line + "\n")
	}
	if stop < len(lines) {
		b.WriteString(m.st.Bar.Render(fmt.Sprintf("↓ %d more lines", len(lines)-stop)) + "\n")
	}
	_ = width

	actions := [][2]string{{"Esc", "Close"}}
	if len(lines) > page {
		actions = [][2]string{{"↑↓", "Scroll"}, {"Esc", "Close"}}
	}
	return m.frame(b.String(), actions)
}

// errWrapped wraps the detail to the content width so scrolling is line-based.
func (m *Model) errWrapped() string {
	width := max(20, m.contentWidth()-6)
	var out []string
	for _, line := range strings.Split(m.errDetail, "\n") {
		out = append(out, wrapLine(line, width)...)
	}
	return strings.Join(out, "\n")
}

func (m *Model) clampErrScroll(page int) {
	total := len(strings.Split(m.errWrapped(), "\n"))
	maxScroll := max(0, total-page)
	if m.errScroll > maxScroll {
		m.errScroll = maxScroll
	}
	if m.errScroll < 0 {
		m.errScroll = 0
	}
}

func wrapLine(line string, width int) []string {
	if lipgloss.Width(line) <= width {
		return []string{line}
	}

	var out []string
	runes := []rune(line)
	for len(runes) > width {
		out = append(out, string(runes[:width]))
		runes = runes[width:]
	}
	if len(runes) > 0 {
		out = append(out, string(runes))
	}
	return out
}

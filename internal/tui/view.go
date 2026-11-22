package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/charmbracelet/x/ansi"
	"github.com/gasparsuarez/repora/internal/domain"

	"github.com/gasparsuarez/repora/internal/tui/components"
)

// View implements tea.Model.
func (m *Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "loading…"
	}
	if m.prompt.active() {
		return m.renderPrompt()
	}
	if m.confirm.active {
		return m.renderConfirm()
	}
	if m.errOpen {
		return m.renderErrorOverlay()
	}
	if m.showHelp {
		return m.renderHelp()
	}
	switch m.screen {
	case screenHome:
		return m.renderHome()
	case screenWorkspace:
		return m.renderWorkspace()
	default:
		return m.renderRepository()
	}
}

// brandHeader renders the shared application header.
func (m *Model) brandHeader() string {
	return components.Header(m.st, components.HeaderData{
		Width:    m.contentWidth(),
		Warnings: len(m.warnings),
		Version:  m.version,
	})
}

func (m *Model) renderHome() string {
	var b strings.Builder

	b.WriteString(m.brandHeader())
	b.WriteString("\n" + components.Rule(m.st, m.contentWidth()))
	b.WriteString("\n\n")

	b.WriteString(m.st.Bar.Render("YOUR WORKSPACES"))
	b.WriteString("\n\n")

	if len(m.cfg.Workspaces) == 0 {
		b.WriteString("  " + m.st.Dim.Render("No workspaces yet.") + "\n")
		b.WriteString("  " + m.st.Dim.Render("Press n to create one, or edit ") + m.st.Header.Render(m.configPath) + "\n")
		return m.frame(b.String(), homeActions())
	}

	avail := m.homeAvail()
	start, end := m.homeWindow(len(m.cfg.Workspaces), avail)

	cards := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		cards = append(cards, components.Card(m.st, m.cardData(i)))
	}
	b.WriteString(strings.Join(cards, "\n\n"))

	if end < len(m.cfg.Workspaces) {
		used := components.CardHeight*len(cards) + (len(cards) - 1)
		if used+2 <= avail {
			b.WriteString("\n\n" + m.st.Bar.Render(fmt.Sprintf("↓ %d more", len(m.cfg.Workspaces)-end)))
		}
	}

	return m.frame(b.String(), homeActions())
}

func homeActions() [][2]string {
	return [][2]string{
		{"↑↓", "Navigate"},
		{"↵", "Open"},
		{"n", "New"},
		{"e", "Rename"},
		{"d", "Delete"},
		{"r", "Refresh"},
		{"?", "Help"},
	}
}

func (m *Model) cardData(i int) components.CardData {
	ws := m.cfg.Workspaces[i]
	agg := m.aggregateFor(ws)

	tags := make([]string, 0, len(ws.RepositoryIDs))
	for _, id := range ws.RepositoryIDs {
		repo, ok := m.cfg.RepositoryByID(id)
		if !ok {
			tags = append(tags, string(id))
			continue
		}
		tags = append(tags, repo.Name)
	}

	return components.CardData{
		Name:        ws.Name,
		Description: ws.Description,
		RepoCount:   len(ws.RepositoryIDs),
		Tags:        tags,
		Status:      agg.counts(),
		Loading:     m.homeLoading && agg.known == 0 && len(ws.RepositoryIDs) > 0,
		Loaded:      m.homeLoaded,
		Selected:    i == m.wsIndex,
		Width:       m.contentWidth(),
	}
}

func (m *Model) renderWorkspace() string {
	ws := m.cfg.Workspaces[m.wsIndex]

	var b strings.Builder
	b.WriteString(m.brandHeader())
	b.WriteString("\n" + components.Rule(m.st, m.contentWidth()) + "\n")
	b.WriteString(components.Breadcrumb(m.st, []string{"Workspaces", ws.Name}, m.contentWidth(),
		plural(len(m.rows), "repository", "repositories")))
	b.WriteString("\n")

	headerLines := 3
	if ws.Description != "" {
		b.WriteString(m.st.Bar.Render(ws.Description))
		b.WriteString("\n")
		headerLines++
	}

	panelLines := m.height - 3 - contentMarginTop - headerLines
	if panelLines < 4 {
		panelLines = 4
	}

	body := m.workspaceBody(ws, panelLines)
	if needed := strings.Count(body, "\n") + 1 + 2; needed < panelLines {
		panelLines = needed
	}
	b.WriteString(m.renderPanel(body, panelLines))

	return m.frame(b.String(), [][2]string{
		{"Enter", "Open"},
		{"a", "Add"},
		{"x", "Remove"},
		{"f", "Fetch"},
		{"r", "Refresh"},
		{"Esc", "Back"},
	})
}

func (m *Model) workspaceBody(ws domain.Workspace, panelLines int) string {
	inner := panelLines - 2
	if inner < 1 {
		inner = 1
	}

	if len(m.rows) == 0 {
		return strings.Join([]string{
			m.st.Dim.Render("This workspace has no repositories."),
			"",
			m.st.Key.Render("a") + " " + m.st.Label.Render("Add a repository by path"),
		}, "\n")
	}

	capacity := inner
	if len(m.rows) > capacity {
		capacity--
	}
	start, end := sliceWindow(m.rowIndex, len(m.rows), capacity)

	nameWidth := m.nameWidth()

	var b strings.Builder
	for i := start; i < end; i++ {
		row := m.rows[i]
		cursor := " "
		nameStyle := m.st.Header
		if i == m.rowIndex {
			cursor = m.st.Accent.Render(">")
			nameStyle = m.st.Selected
		}

		name := truncate(row.repo.Name, nameWidth)
		line := cursor + " " + nameStyle.Render(padRight(name, nameWidth))
		line += "  " + m.stateText(row)

		if branch := m.branchLabel(row); branch != "" {
			remaining := m.contentWidth() - lipgloss.Width(line) - 2
			if remaining > 4 {
				line += "  " + m.st.Bar.Render(truncate(branch, remaining))
			}
		}

		if detail := m.detailText(row); detail != "" && m.contentWidth() >= 104 {
			remaining := m.contentWidth() - lipgloss.Width(line) - 2
			if remaining > 8 {
				line += "  " + m.st.Dim.Render(truncate(detail, remaining))
			}
		}
		b.WriteString(line + "\n")
	}

	if end < len(m.rows) {
		b.WriteString(m.st.Bar.Render(fmt.Sprintf("↓ %d more", len(m.rows)-end)))
	}

	return strings.TrimRight(b.String(), "\n")
}

// renderPanel wraps body in the standard surface, truncating it to fit so the
// bottom border is never clipped.
func (m *Model) renderPanel(body string, maxLines int) string {
	return m.renderPanelWith(body, maxLines, m.st.Panel)
}

func (m *Model) renderPanelWith(body string, maxLines int, style lipgloss.Style) string {
	if maxLines < 3 {
		maxLines = 3
	}
	inner := maxLines - 2

	lines := strings.Split(body, "\n")
	if len(lines) > inner {
		lines = lines[:inner]
		lines[len(lines)-1] = m.st.Bar.Render("…")
	}
	for len(lines) < inner {
		lines = append(lines, "")
	}

	return style.Width(m.contentWidth() - 2).Render(strings.Join(lines, "\n"))
}

// sliceWindow returns the visible range that keeps selected in view.
func sliceWindow(selected, total, capacity int) (start, end int) {
	if capacity <= 0 || total <= 0 {
		return 0, 0
	}
	if total <= capacity {
		return 0, total
	}

	start = selected - capacity/2
	if start < 0 {
		start = 0
	}
	if start > total-capacity {
		start = total - capacity
	}
	return start, start + capacity
}

func (m *Model) nameWidth() int {
	effective := m.contentWidth() - 4 // panel borders and padding
	switch {
	case effective < 86:
		return 16
	case effective < 106:
		return 20
	default:
		return 22
	}
}

// branchLabel returns the branch to show next to the state, if it adds context.
func (m *Model) branchLabel(row repoRow) string {
	if row.loading || row.unavailable || row.state.Err != nil {
		return ""
	}
	label := row.state.Branch.Label()
	if label == "(unknown)" {
		return ""
	}
	return label
}

func (m *Model) stateText(row repoRow) string {
	switch {
	case row.unavailable:
		return m.st.Danger.Render("! unavailable")
	case row.busy:
		return m.st.Info.Render("◐ " + truncate(row.busyLabel, max(10, m.contentWidth()-45)) + "…")
	case row.loading:
		return m.st.Dim.Render("◐ refreshing")
	case row.state.Err != nil:
		return m.st.Danger.Render("✗ error")
	}

	branch := row.state.Branch
	switch row.state.Class() {
	case domain.ClassConflicts:
		return m.st.Danger.Render("✖ conflicts")
	case domain.ClassOperation:
		return m.st.Warning.Render("⏸ " + string(row.state.Operation))
	case domain.ClassDiverged:
		return m.st.Danger.Render(fmt.Sprintf("⇅ %d↑ %d↓", branch.Ahead, branch.Behind))
	case domain.ClassBehind:
		return m.st.Info.Render(fmt.Sprintf("↓ behind %d", branch.Behind))
	case domain.ClassAhead:
		return m.st.Warning.Render(fmt.Sprintf("↑ ahead %d", branch.Ahead))
	case domain.ClassDirty:
		return m.st.Warning.Render("● modified")
	default:
		return m.st.Success.Render("✓ clean")
	}
}

func (m *Model) detailText(row repoRow) string {
	if row.loading || row.unavailable || row.state.Err != nil {
		return ""
	}

	var parts []string
	if n := row.state.StagedCount(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d staged", n))
	}
	if n := row.state.UnstagedCount(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d unstaged", n))
	}
	if n := row.state.UntrackedCount(); n > 0 {
		parts = append(parts, fmt.Sprintf("%d untracked", n))
	}
	return strings.Join(parts, " · ")
}

func (m *Model) renderHelp() string {
	lines := []string{
		m.st.Title.Render("Help"),
		"",
		m.st.Dim.Render("Config   ") + m.configPath,
		m.st.Dim.Render("Git      ") + m.st.Header.Render("native executable on PATH"),
		"",
		m.st.Header.Render("Global"),
		"  ↑ ↓ / j k    move selection",
		"  g / G        first / last",
		"  Enter        open",
		"  Esc          back",
		"  r            refresh repositories",
		"  ,            open config in $EDITOR (reloads on exit)",
		"  Ctrl+R       reload config from disk",
		"  ?            close help",
		"  Ctrl+C       cancel operation, then quit",
		"  q            quit (home only)",
		"",
		m.st.Header.Render("Repository"),
		"  1-4          Overview / Changes / Branches / History",
		"  Space        stage selected file (Changes)",
		"  A / u        stage all / unstage all (Changes)",
		"  c            commit (Changes)",
		"  n / d        new / delete branch (Branches)",
		"  Enter        diff, commit details, or checkout",
		"  f / p / P    fetch / pull / push",
		"  e            show the last error in full",
		"",
		m.st.Header.Render("Workspaces"),
		"  n / e / d    new / rename / delete workspace (home)",
		"  a / x        add / remove repository (workspace)",
		"",
		m.st.Header.Render("Warnings"),
	}

	if len(m.warnings) == 0 {
		lines = append(lines, "  none")
	} else {
		for _, warning := range m.warnings {
			lines = append(lines, "  "+m.st.Warning.Render("!")+" "+warning)
		}
	}

	return m.frame(strings.Join(lines, "\n"), [][2]string{{"Esc", "Close"}})
}

func (m *Model) frame(body string, actions [][2]string) string {
	width := m.contentWidth()
	indent := strings.Repeat(" ", contentMarginX)

	lines := strings.Split(body, "\n")

	// Reserve the top margin, a gap above the status line, and the footer.
	maxContent := m.height - 3 - contentMarginTop
	if maxContent < 1 {
		maxContent = 1
	}
	if len(lines) > maxContent {
		lines = lines[:maxContent]
	}
	for len(lines) < maxContent {
		lines = append(lines, "")
	}
	for i, line := range lines {
		lines[i] = indent + ansi.Truncate(line, width, "…")
	}

	top := strings.Repeat("\n", contentMarginTop)
	status := indent + m.statusLine()
	bar := indent + m.actionBar(actions)

	return top + strings.Join(lines, "\n") + "\n\n" + status + "\n" + bar
}

func (m *Model) statusLine() string {
	switch {
	case m.notice != "":
		style := m.st.Info
		if m.noticeError {
			style = m.st.Danger
		}
		return style.Render(m.notice)
	case m.diffLoading:
		return m.st.Dim.Render("◐ loading diff…")
	case m.detailLoading && m.screen == screenRepository:
		return m.st.Dim.Render("◐ loading repository…")
	case m.loadingCount() > 0:
		return m.st.Dim.Render(fmt.Sprintf("◐ refreshing %d of %d repositories…", m.loadingCount(), len(m.rows)))
	default:
		return ""
	}
}

func (m *Model) actionBar(actions [][2]string) string {
	items := make([]components.FooterItem, 0, len(actions))
	for _, action := range actions {
		items = append(items, components.FooterItem{Key: action[0], Label: action[1]})
	}
	return components.Footer(m.st, m.contentWidth(), items)
}

func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, pluralForm)
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(runes[:max-1]) + "…"
}

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

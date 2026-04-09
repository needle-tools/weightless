package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"weightless/internal/branding"
	"weightless/internal/scan"
)

const (
	tabSummary = iota
	tabModels
)

const titleArt = branding.Banner

type keyMap struct {
	Left    key.Binding
	Right   key.Binding
	Drill   key.Binding
	Open    key.Binding
	Refresh key.Binding
	Back    key.Binding
	Quit    key.Binding
}

type Model struct {
	loadReport      func() (scan.Report, error)
	baseReport      scan.Report
	report          scan.Report
	activeTab       int
	providerFilter  string
	providerRoot    string
	summaryCursor   int
	tableCursors    []int
	help            help.Model
	keys            keyMap
	tables          []table.Model
	openPaths       [][]string
	width           int
	height          int
	titleStyle      lipgloss.Style
	tabStyle        lipgloss.Style
	liveStyle       lipgloss.Style
	mutedStyle      lipgloss.Style
	filterLineStyle lipgloss.Style
	statusLineStyle lipgloss.Style
	statusMessage   string
}

type helpBindings struct {
	short []key.Binding
}

func (h helpBindings) ShortHelp() []key.Binding {
	return h.short
}

func (h helpBindings) FullHelp() [][]key.Binding {
	return [][]key.Binding{h.short}
}

type refreshDoneMsg struct {
	report scan.Report
	err    error
}

func New(report scan.Report, loadReport func() (scan.Report, error)) Model {
	m := Model{
		loadReport: loadReport,
		baseReport: report,
		report:     report,
		help:       help.New(),
		keys: keyMap{
			Left:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←", "prev tab")),
			Right:   key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→", "next tab")),
			Drill:   key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("↵/space", "drill")),
			Open:    key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open")),
			Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
			Back:    key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("esc", "back")),
			Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		},
		titleStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("212")),
		tabStyle:        lipgloss.NewStyle().Padding(0, 1),
		liveStyle:       lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")),
		mutedStyle:      lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		filterLineStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("81")),
		statusLineStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
	}
	m.rebuildTables()
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rebuildTables()
	case refreshDoneMsg:
		if msg.err != nil {
			m.statusMessage = "Refresh failed: " + msg.err.Error()
			return m, nil
		}
		m.statusMessage = fmt.Sprintf("Refreshed %s", time.Now().Format("15:04:05"))
		previousTab := m.activeTab
		m.baseReport = msg.report
		if m.providerFilter != "" && hasProvider(msg.report.Summary, m.providerFilter) {
			m.report = filterReport(msg.report, m.providerFilter)
			m.providerRoot = providerRootFor(msg.report, m.providerFilter)
		} else {
			m.providerFilter = ""
			m.providerRoot = ""
			m.report = msg.report
		}
		if previousTab >= len(m.tables) {
			previousTab = tabSummary
		}
		if previousTab < 0 {
			previousTab = tabSummary
		}
		m.activeTab = previousTab
		m.rebuildTables()
		if m.providerFilter == "" {
			m.setTableCursor(tabSummary, m.summaryCursor)
		}
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Refresh):
			if m.loadReport == nil {
				return m, nil
			}
			m.statusMessage = "Refreshing..."
			return m, func() tea.Msg {
				report, err := m.loadReport()
				return refreshDoneMsg{report: report, err: err}
			}
		case key.Matches(msg, m.keys.Back):
			if m.providerFilter != "" {
				m.providerFilter = ""
				m.providerRoot = ""
				m.report = m.baseReport
				m.activeTab = tabSummary
				m.rebuildTables()
				m.setTableCursor(tabSummary, m.summaryCursor)
			}
		case key.Matches(msg, m.keys.Left):
			if m.activeTab == 0 {
				m.activeTab = len(m.tables) - 1
			} else {
				m.activeTab--
			}
		case key.Matches(msg, m.keys.Right):
			m.activeTab = (m.activeTab + 1) % len(m.tables)
		case key.Matches(msg, m.keys.Drill):
			if m.activeTab == tabSummary {
				row := m.tables[tabSummary].Cursor()
				if row >= 0 && row < len(m.report.Summary) {
					m.summaryCursor = row
					m.providerFilter = m.report.Summary[row].Provider
					m.report = filterReport(m.baseReport, m.providerFilter)
					m.providerRoot = providerRootFor(m.baseReport, m.providerFilter)
					m.activeTab = tabModels
					m.rebuildTables()
				}
			}
		case key.Matches(msg, m.keys.Open):
			return m, openSelected(m.activeTab, m.tables, m.openPaths)
		default:
			var cmd tea.Cmd
			m.tables[m.activeTab], cmd = m.tables[m.activeTab].Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) View() string {
	tabs := []string{"Summary", "Models"}
	var renderedTabs []string
	for i, label := range tabs {
		style := m.tabStyle.Copy()
		if i == m.activeTab {
			style = style.Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("63"))
		} else {
			style = style.Foreground(lipgloss.Color("244"))
		}
		renderedTabs = append(renderedTabs, style.Render(label))
	}

	header := strings.Join(renderedTabs, " ")
	titleText := renderTitle(m.width)
	title := m.titleStyle.Render(titleText)
	leading := ""
	if titleText != "weightless" {
		leading = ""
	}
	summary := fmt.Sprintf(
		"%s across %d models",
		m.liveStyle.Render(m.report.TotalBytesHuman),
		m.report.TotalArtifacts,
	)

	filterLine := ""
	if m.providerFilter != "" {
		filterLine = m.filterLineStyle.Render("Provider: " + m.providerFilter + ", path: " + m.providerRoot)
	}

	statusLine := ""
	if strings.TrimSpace(m.statusMessage) != "" {
		statusLine = m.statusLineStyle.Render(m.statusMessage)
	}

	body := m.tables[m.activeTab].View()
	footer := m.footerView()

	return "\n\n\n\n\n\n" + lipgloss.JoinVertical(
		lipgloss.Left,
		leading,
		title,
		header,
		summary,
		filterLine,
		statusLine,
		body,
		footer,
	)
}

func (m Model) footerView() string {
	left := m.help.View(helpBindings{short: m.helpBindings()})
	right := m.liveStyle.Copy().Bold(false).Render("by 🌵 needle")
	if strings.TrimSpace(left) == "" {
		return right
	}
	if m.width <= 0 {
		return left
	}
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	if leftWidth+rightWidth+2 > m.width {
		return left
	}
	return left + strings.Repeat(" ", m.width-leftWidth-rightWidth) + right
}

func renderTitle(width int) string {
	if width >= 56 {
		return titleArt
	}
	return "weightless"
}

func (m *Model) rebuildTables() {
	if len(m.tables) > 0 {
		m.tableCursors = captureTableCursors(m.tables)
	}
	m.tables, m.openPaths = buildTables(m.baseReport, m.report, m.width, m.providerFilter, m.providerRoot)
	tableHeight := m.availableTableHeight()
	for i := range m.tables {
		m.tables[i].SetWidth(max(40, m.width-2))
		m.tables[i].SetHeight(tableHeight)
		if i < len(m.tableCursors) {
			m.setTableCursor(i, m.tableCursors[i])
		}
	}
}

func (m Model) availableTableHeight() int {
	if m.height <= 0 {
		return 8
	}

	titleText := renderTitle(m.width)
	header := strings.Join([]string{"Summary", "Models"}, " ")
	summary := fmt.Sprintf("%s across %d models", m.report.TotalBytesHuman, m.report.TotalArtifacts)
	filterLine := ""
	if m.providerFilter != "" {
		filterLine = "Provider: " + m.providerFilter + ", path: " + m.providerRoot
	}
	statusLine := strings.TrimSpace(m.statusMessage)
	footer := m.help.View(helpBindings{short: m.helpBindings()})

	chromeHeight := 0
	for _, block := range []string{titleText, header, summary, filterLine, statusLine, footer} {
		if strings.TrimSpace(block) == "" {
			continue
		}
		chromeHeight += lipgloss.Height(block)
	}

	remaining := m.height - chromeHeight
	if remaining < 3 {
		return 3
	}
	return remaining
}

func buildTables(base scan.Report, report scan.Report, width int, providerFilter, providerRoot string) ([]table.Model, [][]string) {
	var paths [][]string
	summaryCols := summaryColumns(width)
	modelCols := artifactColumns(width, providerFilter != "")
	tables := []table.Model{
		newTable(
			summaryCols,
			func() []table.Row {
				rows := make([]table.Row, 0, len(report.Summary))
				tabPaths := make([]string, 0, len(report.Summary))
				for _, item := range report.Summary {
					root := providerRootFor(base, item.Provider)
					rows = append(rows, table.Row{
						item.Provider,
						fmt.Sprintf("%d", item.Artifacts),
						item.BytesHuman,
						root,
					})
					tabPaths = append(tabPaths, root)
				}
				paths = append(paths, tabPaths)
				return rows
			}(),
		),
		newTable(
			modelCols,
			func() []table.Row {
				rows := make([]table.Row, 0, len(report.Artifacts))
				tabPaths := make([]string, 0, len(report.Artifacts))
				for _, item := range report.Artifacts {
					displayPath := relativeToRoot(item.Path, providerRoot)
					if providerFilter == "" {
						displayPath = item.Path
					}
					rows = append(rows, table.Row{
						item.Name,
						item.SizeHuman,
						item.PrimaryProvider,
						displayTimestamp(item.Timestamp),
						displayPath,
					})
					tabPaths = append(tabPaths, item.Path)
				}
				paths = append(paths, tabPaths)
				return rows
			}(),
		),
	}
	return tables, paths
}

func newTable(cols []table.Column, rows []table.Row) table.Model {
	t := table.New(table.WithColumns(cols), table.WithRows(rows), table.WithFocused(true))
	t.SetStyles(table.Styles{
		Header: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252")).BorderBottom(true).Padding(0, 1),
		Cell:   lipgloss.NewStyle().Padding(0, 1),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("63")).
			Bold(true),
	})
	t.SetHeight(20)
	return t
}

func summaryColumns(width int) []table.Column {
	w := max(96, width-2)
	pathWidth := max(32, w-(18+7+10))
	return []table.Column{
		{Title: "Provider", Width: 18},
		{Title: "Models", Width: 7},
		{Title: "Size", Width: 10},
		{Title: "Path", Width: pathWidth},
	}
}

func artifactColumns(width int, drilled bool) []table.Column {
	_ = drilled
	w := max(104, width-2)
	nameWidth := max(24, int(float64(w)*0.24))
	pathWidth := max(22, w-(nameWidth+10+12+10+12))
	return []table.Column{
		{Title: "Name", Width: nameWidth},
		{Title: "Size", Width: 10},
		{Title: "Provider", Width: 12},
		{Title: "Timestamp", Width: 12},
		{Title: "Path", Width: pathWidth},
	}
}

func filterReport(base scan.Report, provider string) scan.Report {
	out := base
	out.Summary = filterSummaries(base.Summary, provider)
	out.Artifacts = filterArtifacts(base.Artifacts, provider)
	out.Locations = filterLocations(base.Locations, provider)
	out.ProviderSummaries = out.Summary
	out.LocationSummaries = out.Locations
	out.TotalArtifacts = len(out.Artifacts)
	out.TotalBytes = 0
	for _, item := range out.Artifacts {
		out.TotalBytes += item.SizeBytes
	}
	out.TotalBytesHuman = humanBytes(out.TotalBytes)
	return out
}

func filterSummaries(items []scan.ProviderSummary, provider string) []scan.ProviderSummary {
	out := make([]scan.ProviderSummary, 0, 1)
	for _, item := range items {
		if item.Provider == provider {
			out = append(out, item)
		}
	}
	return out
}

func filterArtifacts(items []scan.Artifact, provider string) []scan.Artifact {
	out := make([]scan.Artifact, 0)
	for _, item := range items {
		if item.PrimaryProvider == provider {
			out = append(out, item)
		}
	}
	return out
}

func filterLocations(items []scan.LocationSummary, provider string) []scan.LocationSummary {
	out := make([]scan.LocationSummary, 0)
	for _, item := range items {
		if item.Provider == provider {
			out = append(out, item)
		}
	}
	return out
}

func humanBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func providerRootFor(report scan.Report, provider string) string {
	filtered := filterArtifacts(report.Artifacts, provider)
	if len(filtered) == 0 {
		for _, loc := range report.Locations {
			if loc.Provider == provider && loc.Exists {
				return loc.Root
			}
		}
		return ""
	}
	paths := make([]string, 0, len(filtered))
	for _, item := range filtered {
		paths = append(paths, item.Path)
	}
	return commonPathPrefix(paths)
}

func commonPathPrefix(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	segments := splitPath(paths[0])
	for _, path := range paths[1:] {
		current := splitPath(path)
		n := 0
		for n < len(segments) && n < len(current) && segments[n] == current[n] {
			n++
		}
		segments = segments[:n]
		if len(segments) == 0 {
			return ""
		}
	}
	return joinPath(segments)
}

func splitPath(path string) []string {
	clean := strings.TrimPrefix(path, "/")
	if clean == "" {
		return []string{""}
	}
	return strings.Split(clean, "/")
}

func joinPath(segments []string) string {
	if len(segments) == 1 && segments[0] == "" {
		return "/"
	}
	return "/" + strings.Join(segments, "/")
}

func relativeToRoot(path, root string) string {
	if path == "" || root == "" {
		return path
	}
	trimmedRoot := strings.TrimSuffix(root, "/")
	if path == trimmedRoot {
		return "."
	}
	prefix := trimmedRoot + "/"
	if strings.HasPrefix(path, prefix) {
		return strings.TrimPrefix(path, prefix)
	}
	return path
}

func (m Model) helpBindings() []key.Binding {
	bindings := []key.Binding{m.keys.Left, m.keys.Right}
	switch m.activeTab {
	case tabSummary:
		bindings = append(bindings, m.keys.Drill, m.keys.Open)
	case tabModels:
		bindings = append(bindings, m.keys.Open)
	}
	bindings = append(bindings, m.keys.Refresh)
	if m.providerFilter != "" {
		bindings = append(bindings, m.keys.Back)
	}
	bindings = append(bindings, m.keys.Quit)
	return bindings
}

func hasProvider(items []scan.ProviderSummary, provider string) bool {
	for _, item := range items {
		if item.Provider == provider {
			return true
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func dimCell(style lipgloss.Style, value string, width int) string {
	return style.Render(truncateForCell(value, width))
}

func truncateForCell(value string, width int) string {
	if width <= 0 || lipgloss.Width(value) <= width {
		return value
	}
	if width <= 1 {
		return "…"
	}
	runes := []rune(value)
	out := make([]rune, 0, len(runes))
	for _, r := range runes {
		candidate := string(append(out, r))
		if lipgloss.Width(candidate+"…") > width {
			break
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return "…"
	}
	return string(out) + "…"
}

func captureTableCursors(tables []table.Model) []int {
	out := make([]int, len(tables))
	for i := range tables {
		out[i] = tables[i].Cursor()
	}
	return out
}

func (m *Model) setTableCursor(index, cursor int) {
	if index < 0 || index >= len(m.tables) || index >= len(m.openPaths) {
		return
	}
	rowCount := len(m.openPaths[index])
	if rowCount == 0 {
		return
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= rowCount {
		cursor = rowCount - 1
	}
	m.tables[index].SetCursor(cursor)
}

func openSelected(tabIndex int, tables []table.Model, openPaths [][]string) tea.Cmd {
	if tabIndex < 0 || tabIndex >= len(tables) || tabIndex >= len(openPaths) {
		return nil
	}
	row := tables[tabIndex].Cursor()
	if row < 0 || row >= len(openPaths[tabIndex]) {
		return nil
	}
	target := openPaths[tabIndex][row]
	if target == "" {
		return nil
	}
	cmd := openCommand(target)
	if cmd == nil {
		return nil
	}
	return tea.ExecProcess(cmd, nil)
}

func openCommand(target string) *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", target)
	case "windows":
		return exec.Command("explorer.exe", "/select,", target)
	default:
		return exec.Command("xdg-open", target)
	}
}

func displayTimestamp(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		if len(value) >= 10 {
			return value[:10]
		}
		return value
	}
	return parsed.Format("2006-01-02")
}

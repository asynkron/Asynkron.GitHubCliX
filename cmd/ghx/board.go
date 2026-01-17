package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type boardLane struct {
	Name   string
	Label  string
	Color  string
	Issues []*Issue
}

type boardLayout struct {
	colWidth    int
	colGap      int
	boardTop    int
	boardHeight int
	cardHeight  int
	cardGap     int
}

type boardModel struct {
	owner         string
	repo          string
	lanes         []boardLane
	laneIndex     int
	rowByCol      []int
	offsetByCol   []int
	width         int
	height        int
	layout        boardLayout
	loading       bool
	busy          bool
	moving        bool
	status        string
	errMsg        string
	detailsOpen   bool
	detailsIssue  *Issue
	detailsView   viewport.Model
	detailsLoad   bool
	mdRenderer    *glamour.TermRenderer
	mdWidth       int
	issueState    string
	issueLimit    int
	movingIssue   int
	dragging      bool
	moveSticky    bool
	pendingMove   *pendingMove
	debounceSeq   int
	debounceReady bool
}

type issuesLoadedMsg struct {
	issues []Issue
	err    error
}

type issueDetailsMsg struct {
	number int
	body   string
	err    error
}

type pendingMove struct {
	number int
	target int
	seq    int
}

type debounceMsg struct {
	seq int
}

type commitMoveMsg struct {
	number int
	target int
	add    string
	remove []string
	err    error
}

const (
	boardHeaderHeight = 2
	boardHelpHeight   = 1
)

type Theme struct {
	Background   string
	Paper        string
	Foreground   string
	Muted        string
	AccentBlue   string
	AccentPurple string
	AccentYellow string
	AccentGreen  string
	AccentRed    string
	AccentCyan   string
}

var defaultTheme = Theme{
	Background:   "#21252B",
	Paper:        "#282C34",
	Foreground:   "#ABB2BF",
	Muted:        "#5C6370",
	AccentBlue:   "#61AFEF",
	AccentPurple: "#C678DD",
	AccentYellow: "#E5C07B",
	AccentGreen:  "#98C379",
	AccentRed:    "#E06C75",
	AccentCyan:   "#56B6C2",
}

var theme = defaultTheme

var (
	boardTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.AccentYellow))
	boardMutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Muted))
	boardErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.AccentRed))
	boardHelpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.AccentPurple))
)

func defaultBoardLanes() []boardLane {
	return []boardLane{
		{Name: "TODO", Label: "TODO", Color: stripHash(theme.AccentBlue)},
		{Name: "Doing", Label: "Doing", Color: stripHash(theme.AccentYellow)},
		{Name: "Done", Label: "Done", Color: stripHash(theme.AccentGreen)},
		{Name: "Blocked", Label: "Blocked", Color: stripHash(theme.AccentRed)},
	}
}

func runBoard(args []string) int {
	for _, a := range args {
		if a == "--help" || a == "-h" || a == "help" {
			printBoardHelp()
			return 0
		}
	}

	state := "all"
	limit := 200
	for i := 0; i < len(args); i++ {
		if (args[i] == "--state" || args[i] == "-state") && i+1 < len(args) {
			state = args[i+1]
			i++
			continue
		}
		if (args[i] == "--limit" || args[i] == "-limit") && i+1 < len(args) {
			limit = atoi(args[i+1])
			i++
			continue
		}
	}
	if limit <= 0 {
		limit = 200
	}
	if state != "open" && state != "closed" && state != "all" {
		state = "all"
	}

	owner, repo, err := getRepoInfo()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ghx: failed to get repo info: %v\n", err)
		return 1
	}

	lanes := defaultBoardLanes()
	if err := ensureLaneLabels(lanes); err != nil {
		fmt.Fprintf(os.Stderr, "ghx: failed to ensure board labels: %v\n", err)
		return 1
	}

	m := newBoardModel(owner, repo, lanes, state, limit)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ghx: board failed: %v\n", err)
		return 1
	}
	return 0
}

func printBoardHelp() {
	fmt.Println("Open a full-screen Kanban board for GitHub issues.")
	fmt.Println()
	fmt.Println("USAGE")
	fmt.Println("  ghx board [--state, -state <open|closed|all>] [--limit, -limit <n>]")
	fmt.Println()
	fmt.Println("KEYS")
	fmt.Println("  arrows:  move cursor")
	fmt.Println("  enter:   pick/drop issue for lane move")
	fmt.Println("  i:       issue details")
	fmt.Println("  r:       refresh issues")
	fmt.Println("  q:       quit")
	fmt.Println()
}

func newBoardModel(owner, repo string, lanes []boardLane, state string, limit int) *boardModel {
	rowByCol := make([]int, len(lanes))
	offsetByCol := make([]int, len(lanes))
	return &boardModel{
		owner:       owner,
		repo:        repo,
		lanes:       lanes,
		rowByCol:    rowByCol,
		offsetByCol: offsetByCol,
		layout:      defaultBoardLayout(0, 0, len(lanes)),
		issueState:  state,
		issueLimit:  limit,
		loading:     true,
		status:      "Loading issues...",
	}
}

func defaultBoardLayout(width, height, cols int) boardLayout {
	layout := boardLayout{
		colGap:     0,
		cardHeight: 5,
		cardGap:    2,
	}
	layout.boardTop = boardHeaderHeight
	if height > 0 {
		layout.boardHeight = height - boardHeaderHeight - boardHelpHeight
		if layout.boardHeight < 4 {
			layout.boardHeight = 4
		}
	}
	if width > 0 && cols > 0 {
		layout.colWidth = (width - layout.colGap*(cols-1)) / cols
		if layout.colWidth < 16 {
			layout.colWidth = 16
		}
	}
	return layout
}

func (m *boardModel) Init() tea.Cmd {
	return loadIssuesCmd(m.issueState, m.issueLimit)
}

func loadIssuesCmd(state string, limit int) tea.Cmd {
	return func() tea.Msg {
		issues, err := fetchIssues(state, limit)
		return issuesLoadedMsg{issues: issues, err: err}
	}
}

func commitMoveCmd(number int, target int, add string, remove []string) tea.Cmd {
	return func() tea.Msg {
		err := editIssueLabels(number, add, remove)
		return commitMoveMsg{
			number: number,
			target: target,
			add:    add,
			remove: remove,
			err:    err,
		}
	}
}

func (m *boardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout = defaultBoardLayout(m.width, m.height, len(m.lanes))
		if m.detailsOpen {
			m.resizeDetails()
		}
		return m, nil
	case issuesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.status = "Failed to load issues."
			return m, nil
		}
		m.errMsg = ""
		m.status = fmt.Sprintf("Loaded %d issues.", len(msg.issues))
		m.lanes = assignIssues(m.lanes, msg.issues)
		m.pendingMove = nil
		m.debounceReady = false
		m.movingIssue = 0
		m.moving = false
		m.moveSticky = false
		m.dragging = false
		m.busy = false
		m.normalizeSelection()
		return m, nil
	case debounceMsg:
		if msg.seq != m.debounceSeq {
			return m, nil
		}
		if m.busy {
			m.debounceReady = true
			return m, nil
		}
		cmd := m.commitPendingMove()
		return m, cmd
	case commitMoveMsg:
		m.busy = false
		if msg.err != nil {
			m.status = fmt.Sprintf("Move failed: %v", msg.err)
			m.loading = true
			return m, loadIssuesCmd(m.issueState, m.issueLimit)
		}
		if issue := m.findIssueByNumber(msg.number); issue != nil && msg.target >= 0 && msg.target < len(m.lanes) {
			issue.Labels = applyLabelChanges(issue.Labels, msg.add, msg.remove, m.lanes[msg.target].Color)
			m.status = fmt.Sprintf("Moved #%d to %s.", msg.number, m.lanes[msg.target].Name)
		}
		if m.debounceReady && m.pendingMove != nil {
			m.debounceReady = false
			cmd := m.commitPendingMove()
			return m, cmd
		}
		return m, nil
	case issueDetailsMsg:
		m.detailsLoad = false
		if msg.err != nil {
			m.status = fmt.Sprintf("Details failed: %v", msg.err)
			return m, nil
		}
		if issue := m.findIssueByNumber(msg.number); issue != nil {
			issue.Body = msg.body
		}
		if m.detailsIssue != nil && m.detailsIssue.Number == msg.number {
			m.detailsView.SetContent(m.detailsContent(m.detailsIssue))
		}
		return m, nil
	case tea.MouseMsg:
		if m.detailsOpen || m.loading {
			return m, nil
		}
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			m.handleMousePress(msg.X, msg.Y)
			return m, nil
		}
		if msg.Action == tea.MouseActionMotion && msg.Button == tea.MouseButtonLeft {
			cmd := m.handleMouseDrag(msg.X, msg.Y)
			return m, cmd
		}
		if msg.Action == tea.MouseActionRelease {
			m.handleMouseRelease()
			return m, nil
		}
		if msg.Type == tea.MouseLeft {
			m.handleMousePress(msg.X, msg.Y)
			return m, nil
		}
		if msg.Type == tea.MouseRelease {
			m.handleMouseRelease()
			return m, nil
		}
		return m, nil
	case tea.KeyMsg:
		if m.detailsOpen {
			return m.updateDetails(msg)
		}
		return m.updateBoardKeys(msg)
	}
	return m, nil
}

func (m *boardModel) updateBoardKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "r":
		if !m.loading && !m.busy {
			m.loading = true
			m.status = "Refreshing issues..."
			return m, loadIssuesCmd(m.issueState, m.issueLimit)
		}
	case "enter":
		issue := m.currentIssue()
		if m.moving {
			m.moving = false
			m.movingIssue = 0
			m.moveSticky = false
			m.status = "Move mode off."
			return m, nil
		}
		if issue != nil {
			m.moving = true
			m.movingIssue = issue.Number
			m.moveSticky = true
			m.status = "Move mode: use left/right to change lane."
		}
	case "esc":
		m.moving = false
		m.movingIssue = 0
		m.moveSticky = false
	case "left", "h":
		if m.moving {
			cmd := m.previewMove(-1)
			return m, cmd
		}
		m.moveLane(-1)
	case "right", "l":
		if m.moving {
			cmd := m.previewMove(1)
			return m, cmd
		}
		m.moveLane(1)
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "i", "o":
		issue := m.currentIssue()
		if issue != nil {
			cmd := m.openDetails(issue)
			return m, cmd
		}
	}
	return m, nil
}

func (m *boardModel) updateDetails(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.detailsOpen = false
		return m, nil
	}
	var cmd tea.Cmd
	m.detailsView, cmd = m.detailsView.Update(msg)
	return m, cmd
}

func (m *boardModel) resizeDetails() {
	if !m.detailsOpen {
		return
	}
	width := m.width - 10
	height := m.height - 6
	if width < 30 {
		width = 30
	}
	if height < 6 {
		height = 6
	}
	m.detailsView.Width = width
	m.detailsView.Height = height
	m.ensureMarkdownRenderer(width)
	m.detailsView.SetContent(m.detailsContent(m.detailsIssue))
}

func (m *boardModel) openDetails(issue *Issue) tea.Cmd {
	m.detailsOpen = true
	m.detailsIssue = issue
	m.detailsLoad = issue.Body == ""
	m.detailsView = viewport.New(0, 0)
	m.resizeDetails()
	m.detailsView.YPosition = 0
	if m.detailsLoad {
		m.detailsView.SetContent(m.detailsContent(issue))
		return loadIssueDetailsCmd(issue.Number)
	}
	return nil
}

func (m *boardModel) moveLane(delta int) {
	next := m.laneIndex + delta
	if next < 0 || next >= len(m.lanes) {
		return
	}
	m.laneIndex = next
	m.normalizeSelection()
}

func (m *boardModel) moveCursor(delta int) {
	if len(m.lanes) == 0 {
		return
	}
	lane := m.lanes[m.laneIndex]
	if len(lane.Issues) == 0 {
		m.rowByCol[m.laneIndex] = 0
		return
	}
	row := m.rowByCol[m.laneIndex] + delta
	if row < 0 {
		row = 0
	}
	if row >= len(lane.Issues) {
		row = len(lane.Issues) - 1
	}
	m.rowByCol[m.laneIndex] = row
	m.adjustOffset(m.laneIndex)
	m.syncMovingIssue()
}

func (m *boardModel) previewMove(delta int) tea.Cmd {
	if m.loading {
		return nil
	}
	if m.movingIssue == 0 {
		if issue := m.currentIssue(); issue != nil {
			m.movingIssue = issue.Number
		} else {
			return nil
		}
	}
	fromLane, _ := m.findIssueLane(m.movingIssue)
	if fromLane < 0 {
		return nil
	}
	target := fromLane + delta
	return m.previewMoveTo(target)
}

func (m *boardModel) previewMoveTo(target int) tea.Cmd {
	if m.loading {
		return nil
	}
	if m.movingIssue == 0 {
		if issue := m.currentIssue(); issue != nil {
			m.movingIssue = issue.Number
		} else {
			return nil
		}
	}
	fromLane, _ := m.findIssueLane(m.movingIssue)
	if fromLane < 0 {
		return nil
	}
	if target == fromLane {
		return nil
	}
	if target < 0 || target >= len(m.lanes) {
		return nil
	}
	m.moveIssueBetweenLanes(m.movingIssue, fromLane, target)
	m.laneIndex = target
	m.debounceSeq++
	seq := m.debounceSeq
	m.pendingMove = &pendingMove{
		number: m.movingIssue,
		target: target,
		seq:    seq,
	}
	m.debounceReady = false
	m.status = fmt.Sprintf("Queued move to %s...", m.lanes[target].Name)
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return debounceMsg{seq: seq}
	})
}

func (m *boardModel) commitPendingMove() tea.Cmd {
	if m.pendingMove == nil || m.busy {
		return nil
	}
	pending := m.pendingMove
	if pending.target < 0 || pending.target >= len(m.lanes) {
		m.pendingMove = nil
		return nil
	}
	issue := m.findIssueByNumber(pending.number)
	if issue == nil {
		m.pendingMove = nil
		return nil
	}
	add, remove := m.buildLabelEdit(issue, m.lanes[pending.target])
	if add == "" && len(remove) == 0 {
		m.pendingMove = nil
		m.status = "Issue already matches target lane."
		return nil
	}
	m.busy = true
	m.pendingMove = nil
	m.status = fmt.Sprintf("Updating #%d...", issue.Number)
	return commitMoveCmd(issue.Number, pending.target, add, remove)
}

func (m *boardModel) moveIssueBetweenLanes(number int, from int, to int) {
	if from == to || from < 0 || to < 0 || from >= len(m.lanes) || to >= len(m.lanes) {
		return
	}
	issue, idx := findIssue(m.lanes[from].Issues, number)
	if issue == nil {
		return
	}
	m.lanes[from].Issues = removeIssueAt(m.lanes[from].Issues, idx)
	m.lanes[to].Issues = append(m.lanes[to].Issues, issue)
	sortLaneIssues(m.lanes[to].Issues)
	m.rowByCol[to] = indexOfIssue(m.lanes[to].Issues, number)
	if m.rowByCol[to] < 0 {
		m.rowByCol[to] = 0
	}
	if m.rowByCol[from] >= len(m.lanes[from].Issues) {
		m.rowByCol[from] = len(m.lanes[from].Issues) - 1
		if m.rowByCol[from] < 0 {
			m.rowByCol[from] = 0
		}
	}
	m.adjustOffset(from)
	m.adjustOffset(to)
}

func (m boardModel) findIssueLane(number int) (int, int) {
	for i := range m.lanes {
		if _, idx := findIssue(m.lanes[i].Issues, number); idx >= 0 {
			return i, idx
		}
	}
	return -1, -1
}

func (m *boardModel) syncMovingIssue() {
	if !m.moving {
		return
	}
	issue := m.currentIssue()
	if issue == nil {
		m.movingIssue = 0
		return
	}
	if m.movingIssue != issue.Number {
		m.movingIssue = issue.Number
		m.pendingMove = nil
		m.debounceReady = false
	}
}

func sortLaneIssues(issues []*Issue) {
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].Number < issues[j].Number
	})
}

func ensureLaneOrder(issues []*Issue) []*Issue {
	sortLaneIssues(issues)
	return issues
}

func findIssue(issues []*Issue, number int) (*Issue, int) {
	for i, issue := range issues {
		if issue.Number == number {
			return issue, i
		}
	}
	return nil, -1
}

func indexOfIssue(issues []*Issue, number int) int {
	for i, issue := range issues {
		if issue.Number == number {
			return i
		}
	}
	return -1
}

func removeIssueAt(issues []*Issue, idx int) []*Issue {
	if idx < 0 || idx >= len(issues) {
		return issues
	}
	return append(issues[:idx], issues[idx+1:]...)
}

func (m boardModel) buildLabelEdit(issue *Issue, target boardLane) (string, []string) {
	var remove []string
	targetLower := lower(target.Label)
	hasTarget := false
	labelMap := make(map[string]Label, len(issue.Labels))
	for _, l := range issue.Labels {
		labelMap[lower(l.Name)] = l
		if lower(l.Name) == targetLower {
			hasTarget = true
		}
	}
	for _, lane := range m.lanes {
		nameLower := lower(lane.Label)
		if nameLower == targetLower {
			continue
		}
		if _, ok := labelMap[nameLower]; ok {
			remove = append(remove, lane.Label)
		}
	}
	add := ""
	if !hasTarget {
		add = target.Label
	}
	return add, remove
}

func applyLabelChanges(labels []Label, add string, remove []string, addColor string) []Label {
	if len(remove) > 0 {
		keep := labels[:0]
		for _, l := range labels {
			if !stringInSliceCaseInsensitive(l.Name, remove) {
				keep = append(keep, l)
			}
		}
		labels = keep
	}
	if add != "" && !labelExists(labels, add) {
		labels = append(labels, Label{Name: add, Color: addColor})
	}
	return labels
}

func labelExists(labels []Label, name string) bool {
	for _, l := range labels {
		if lower(l.Name) == lower(name) {
			return true
		}
	}
	return false
}

func (m boardModel) adjustOffset(col int) {
	if col < 0 || col >= len(m.lanes) {
		return
	}
	cardBlock := m.layout.cardHeight + m.layout.cardGap
	visible := (m.layout.boardHeight - 1) / cardBlock
	if visible < 1 {
		visible = 1
	}
	row := m.rowByCol[col]
	offset := m.offsetByCol[col]
	if row < offset {
		offset = row
	}
	if row >= offset+visible {
		offset = row - visible + 1
	}
	if offset < 0 {
		offset = 0
	}
	m.offsetByCol[col] = offset
}

func (m *boardModel) normalizeSelection() {
	if m.laneIndex < 0 || m.laneIndex >= len(m.lanes) {
		m.laneIndex = 0
	}
	for i := range m.lanes {
		if m.rowByCol[i] >= len(m.lanes[i].Issues) {
			m.rowByCol[i] = len(m.lanes[i].Issues) - 1
		}
		if m.rowByCol[i] < 0 {
			m.rowByCol[i] = 0
		}
		m.adjustOffset(i)
	}
}

func (m boardModel) currentIssue() *Issue {
	if m.laneIndex < 0 || m.laneIndex >= len(m.lanes) {
		return nil
	}
	issues := m.lanes[m.laneIndex].Issues
	if len(issues) == 0 {
		return nil
	}
	row := m.rowByCol[m.laneIndex]
	if row < 0 || row >= len(issues) {
		return nil
	}
	return issues[row]
}

func (m *boardModel) View() string {
	if m.detailsOpen {
		return m.detailsViewString()
	}
	header, status, help := m.renderChrome()
	m.applyLayout(header, status, help)
	board := m.renderBoard()
	content := lipgloss.JoinVertical(lipgloss.Left, header, status, board, help)
	root := lipgloss.NewStyle().Background(lipgloss.Color(theme.Background))
	if m.width > 0 {
		root = root.Width(m.width)
	}
	if m.height > 0 {
		root = root.Height(m.height)
	}
	return root.Render(content)
}

func (m *boardModel) renderChrome() (string, string, string) {
	headerText := fmt.Sprintf("GHX Board  %s/%s", m.owner, m.repo)
	statusText := m.status
	if m.loading {
		statusText = "Loading issues..."
	}
	statusStyle := boardMutedStyle
	if m.errMsg != "" {
		statusText = m.errMsg
		statusStyle = boardErrorStyle
	}
	helpText := "Arrows: move  Enter: pick/drop  i: details  r: refresh  q: quit"

	headerStyle := boardTitleStyle.Background(lipgloss.Color(theme.Background))
	statusStyle = statusStyle.Background(lipgloss.Color(theme.Background))
	helpStyle := boardHelpStyle.Background(lipgloss.Color(theme.Background))
	if m.width > 0 {
		headerStyle = headerStyle.Width(m.width)
		statusStyle = statusStyle.Width(m.width)
		helpStyle = helpStyle.Width(m.width)
	}
	return headerStyle.Render(headerText), statusStyle.Render(statusText), helpStyle.Render(helpText)
}

func (m *boardModel) applyLayout(header, status, help string) {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	cols := len(m.lanes)
	if cols == 0 {
		return
	}
	top := lipgloss.Height(header) + lipgloss.Height(status)
	helpHeight := lipgloss.Height(help)
	if helpHeight < 1 {
		helpHeight = boardHelpHeight
	}
	m.layout.boardTop = top
	m.layout.boardHeight = m.height - top - helpHeight
	if m.layout.boardHeight < 4 {
		m.layout.boardHeight = 4
	}
	m.layout.colWidth = (m.width - m.layout.colGap*(cols-1)) / cols
	if m.layout.colWidth < 16 {
		m.layout.colWidth = 16
	}
	m.layout.cardHeight = m.measureCardHeight()
}

func (m boardModel) renderBoard() string {
	if m.layout.boardHeight < 4 {
		return boardMutedStyle.Render("Window too small for board.")
	}
	columns := make([]string, 0, len(m.lanes))
	for i := range m.lanes {
		columns = append(columns, m.renderColumn(i))
	}
	gap := lipgloss.NewStyle().
		Background(lipgloss.Color(theme.Background)).
		Width(m.layout.colGap).
		Render(strings.Repeat(" ", m.layout.colGap))
	return lipgloss.JoinHorizontal(lipgloss.Top, joinWithGap(columns, gap)...)
}

func joinWithGap(cols []string, gap string) []string {
	if len(cols) == 0 {
		return cols
	}
	result := make([]string, 0, len(cols)*2-1)
	for i, col := range cols {
		if i > 0 {
			result = append(result, gap)
		}
		result = append(result, col)
	}
	return result
}

func (m boardModel) renderColumn(idx int) string {
	lane := m.lanes[idx]
	header := laneHeaderStyle(lane, idx == m.laneIndex, m.layout.colWidth)
	cardAreaHeight := m.layout.boardHeight - 1
	var content strings.Builder
	content.WriteString(header)
	content.WriteString("\n")

	cardBlock := m.layout.cardHeight + m.layout.cardGap
	visible := cardAreaHeight / cardBlock
	if visible < 1 {
		visible = 1
	}
	offset := m.offsetByCol[idx]
	end := offset + visible
	if end > len(lane.Issues) {
		end = len(lane.Issues)
	}
	for i := offset; i < end; i++ {
		selected := idx == m.laneIndex && i == m.rowByCol[idx]
		card := renderCard(lane.Issues[i], selected, m.moving && selected, m.layout.colWidth, m.lanes)
		content.WriteString(card)
		if i < end-1 {
			for g := 0; g < m.layout.cardGap; g++ {
				content.WriteString("\n")
			}
		}
	}
	colStyle := lipgloss.NewStyle().
		Width(m.layout.colWidth).
		Height(m.layout.boardHeight).
		Background(lipgloss.Color(theme.Background))
	return colStyle.Render(content.String())
}

func renderCard(issue *Issue, selected bool, moving bool, width int, lanes []boardLane) string {
	boxWidth := width - 2
	if boxWidth < 1 {
		boxWidth = 1
	}
	contentWidth := boxWidth - 2
	if contentWidth < 1 {
		contentWidth = 1
	}
	cardStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(theme.Paper)).
		Foreground(lipgloss.Color(theme.Foreground)).
		Padding(0, 1).
		Width(boxWidth)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(theme.AccentBlue))
	if selected {
		titleStyle = titleStyle.Foreground(lipgloss.Color(theme.AccentYellow))
	}
	if moving {
		titleStyle = titleStyle.Foreground(lipgloss.Color(theme.AccentPurple))
	}
	title := fmt.Sprintf("#%d %s", issue.Number, issue.Title)
	title = titleStyle.Render(truncateString(title, contentWidth))
	labels := renderIssueBadges(issue, lanes, contentWidth)
	content := title + "\n" + labels
	return cardStyle.Render(content)
}

func renderIssueBadges(issue *Issue, lanes []boardLane, width int) string {
	if width <= 0 {
		return ""
	}
	laneLabels := make(map[string]struct{}, len(lanes))
	for _, lane := range lanes {
		laneLabels[lower(lane.Label)] = struct{}{}
	}
	var parts []string
	for _, label := range issue.Labels {
		if _, ok := laneLabels[lower(label.Name)]; ok {
			continue
		}
		parts = append(parts, label.Name)
		if len(parts) == 2 {
			break
		}
	}
	if len(parts) == 0 {
		if issue.State != "" {
			parts = append(parts, strings.ToLower(issue.State))
		} else {
			parts = append(parts, "no labels")
		}
	}
	line := strings.Join(parts, ", ")
	line = truncateString(line, width)
	return boardMutedStyle.Render(line)
}

func laneHeaderStyle(lane boardLane, active bool, width int) string {
	name := fmt.Sprintf("%s (%d)", lane.Name, len(lane.Issues))
	contentWidth := width
	if contentWidth < 1 {
		contentWidth = 1
	}
	name = truncateString(name, contentWidth)
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#" + lane.Color)).
		Width(width)
	if active {
		style = style.Underline(true)
	}
	return style.Render(name)
}

func (m *boardModel) detailsContent(issue *Issue) string {
	if issue == nil {
		return "No issue selected."
	}
	var md strings.Builder
	md.WriteString(fmt.Sprintf("#%d %s\n\n", issue.Number, issue.Title))
	if issue.State != "" {
		md.WriteString("**State:** ")
		md.WriteString(strings.ToLower(issue.State))
		md.WriteString("  \n")
	}
	if issue.URL != "" {
		md.WriteString("**URL:** ")
		md.WriteString(issue.URL)
		md.WriteString("  \n")
	}
	if len(issue.Labels) > 0 {
		md.WriteString("**Labels:** ")
		for i, label := range issue.Labels {
			if i > 0 {
				md.WriteString(", ")
			}
			md.WriteString(label.Name)
		}
		md.WriteString("  \n")
	}
	md.WriteString("\n---\n\n")
	if issue.Body != "" {
		md.WriteString(issue.Body)
	} else if m.detailsLoad {
		md.WriteString("_Loading details..._")
	} else {
		md.WriteString("_No description._")
	}
	return m.renderMarkdown(md.String())
}

func (m boardModel) detailsViewString() string {
	width := m.width
	height := m.height
	if width <= 0 || height <= 0 {
		return ""
	}
	content := m.detailsView.View()
	modalStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(theme.Paper)).
		Foreground(lipgloss.Color(theme.Foreground)).
		Padding(1, 2).
		Width(m.detailsView.Width + 4).
		Height(m.detailsView.Height + 2)
	modal := modalStyle.Render(content)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, modal)
}

func (m *boardModel) ensureMarkdownRenderer(width int) {
	if width <= 0 {
		width = 80
	}
	if m.mdRenderer != nil && m.mdWidth == width {
		return
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		m.mdRenderer = nil
		m.mdWidth = width
		return
	}
	m.mdRenderer = renderer
	m.mdWidth = width
}

func (m *boardModel) renderMarkdown(content string) string {
	wrapWidth := m.detailsView.Width
	if wrapWidth <= 0 {
		wrapWidth = 80
	}
	m.ensureMarkdownRenderer(wrapWidth)
	if m.mdRenderer == nil {
		return content
	}
	rendered, err := m.mdRenderer.Render(content)
	if err != nil {
		return content
	}
	return rendered
}

func (m *boardModel) handleMousePress(x, y int) {
	m.syncLayoutForInput()
	col, row, ok := m.hitTest(x, y)
	if !ok {
		return
	}
	if col >= 0 {
		m.laneIndex = col
	}
	if row < 0 {
		return
	}
	m.rowByCol[col] = row
	m.adjustOffset(col)
	m.syncMovingIssue()
	m.startDrag()
}

func (m *boardModel) handleMouseDrag(x, y int) tea.Cmd {
	if !m.dragging || m.movingIssue == 0 {
		return nil
	}
	m.syncLayoutForInput()
	if y < m.layout.boardTop || y >= m.layout.boardTop+m.layout.boardHeight {
		return nil
	}
	col := m.laneAtX(x)
	if col < 0 {
		return nil
	}
	return m.previewMoveTo(col)
}

func (m *boardModel) handleMouseRelease() {
	if !m.dragging {
		return
	}
	m.dragging = false
	m.moving = m.moveSticky
	if m.moving {
		m.syncMovingIssue()
	} else {
		m.movingIssue = 0
	}
}

func (m *boardModel) startDrag() {
	m.dragging = true
	m.moving = true
	if issue := m.currentIssue(); issue != nil {
		m.movingIssue = issue.Number
	} else {
		m.movingIssue = 0
	}
}

func (m *boardModel) hitTest(x, y int) (int, int, bool) {
	if y < m.layout.boardTop || y >= m.layout.boardTop+m.layout.boardHeight {
		return -1, -1, false
	}
	col := m.laneAtX(x)
	if col < 0 {
		return -1, -1, false
	}
	relY := y - m.layout.boardTop
	if relY == 0 {
		return col, -1, true
	}
	relY--
	cardBlock := m.layout.cardHeight + m.layout.cardGap
	row := relY / cardBlock
	if relY%cardBlock >= m.layout.cardHeight {
		return col, -1, true
	}
	row += m.offsetByCol[col]
	if row < 0 || row >= len(m.lanes[col].Issues) {
		return col, -1, true
	}
	return col, row, true
}

func (m *boardModel) laneAtX(x int) int {
	colSpan := m.layout.colWidth + m.layout.colGap
	if colSpan <= 0 {
		return -1
	}
	col := x / colSpan
	if col < 0 || col >= len(m.lanes) {
		return -1
	}
	if x >= col*colSpan+m.layout.colWidth {
		return -1
	}
	return col
}

func (m *boardModel) syncLayoutForInput() {
	header, status, help := m.renderChrome()
	m.applyLayout(header, status, help)
}

func (m *boardModel) measureCardHeight() int {
	if m.layout.colWidth <= 0 {
		return m.layout.cardHeight
	}
	dummy := &Issue{Number: 1, Title: "Issue"}
	card := renderCard(dummy, false, false, m.layout.colWidth, m.lanes)
	measured := lipgloss.Height(card)
	if measured < 2 {
		measured = 2
	}
	return measured
}

func ensureLaneLabels(lanes []boardLane) error {
	cmd := exec.Command("gh", "label", "list", "--limit", "200", "--json", "name,color")
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	var existing []Label
	if err := json.Unmarshal(out, &existing); err != nil {
		return err
	}
	existingMap := make(map[string]struct{}, len(existing))
	for _, l := range existing {
		existingMap[lower(l.Name)] = struct{}{}
	}
	for _, lane := range lanes {
		if _, ok := existingMap[lower(lane.Label)]; ok {
			continue
		}
		create := exec.Command("gh", "label", "create", lane.Label, "--color", lane.Color, "--description", "Kanban status: "+lane.Name)
		out, err := create.CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				return err
			}
			return fmt.Errorf("%v: %s", err, msg)
		}
	}
	return nil
}

func fetchIssues(state string, limit int) ([]Issue, error) {
	args := []string{"issue", "list", "--state", state, "--limit", fmt.Sprintf("%d", limit), "--json", "number,title,labels,state,url"}
	cmd := exec.Command("gh", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var issues []Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, err
	}
	return issues, nil
}

func assignIssues(lanes []boardLane, issues []Issue) []boardLane {
	for i := range lanes {
		lanes[i].Issues = nil
	}
	for i := range issues {
		laneIdx := laneForIssue(issues[i], lanes)
		lanes[laneIdx].Issues = append(lanes[laneIdx].Issues, &issues[i])
	}
	for i := range lanes {
		sortLaneIssues(lanes[i].Issues)
	}
	return lanes
}

func laneForIssue(issue Issue, lanes []boardLane) int {
	for i, lane := range lanes {
		if issueHasLabel(issue, lane.Label) {
			return i
		}
	}
	return 0
}

func issueHasLabel(issue Issue, label string) bool {
	for _, l := range issue.Labels {
		if lower(l.Name) == lower(label) {
			return true
		}
	}
	return false
}

func editIssueLabels(number int, add string, remove []string) error {
	args := []string{"issue", "edit", fmt.Sprintf("%d", number)}
	for _, label := range remove {
		args = append(args, "--remove-label", label)
	}
	if add != "" {
		args = append(args, "--add-label", add)
	}
	if len(remove) == 0 && add == "" {
		return nil
	}
	cmd := exec.Command("gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return fmt.Errorf("%v: %s", err, msg)
	}
	return nil
}

func stringInSliceCaseInsensitive(s string, list []string) bool {
	for _, item := range list {
		if lower(item) == lower(s) {
			return true
		}
	}
	return false
}

func truncateString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func stripHash(color string) string {
	if len(color) > 0 && color[0] == '#' {
		return color[1:]
	}
	return color
}

func loadIssueDetailsCmd(number int) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("gh", "issue", "view", fmt.Sprintf("%d", number), "--json", "body")
		out, err := cmd.Output()
		if err != nil {
			return issueDetailsMsg{number: number, err: err}
		}
		var resp struct {
			Body string `json:"body"`
		}
		if err := json.Unmarshal(out, &resp); err != nil {
			return issueDetailsMsg{number: number, err: err}
		}
		return issueDetailsMsg{number: number, body: resp.Body}
	}
}

func (m boardModel) findIssueByNumber(number int) *Issue {
	for i := range m.lanes {
		for _, issue := range m.lanes[i].Issues {
			if issue.Number == number {
				return issue
			}
		}
	}
	return nil
}

func getRepoInfo() (string, string, error) {
	repoCmd := exec.Command("gh", "repo", "view", "--json", "owner,name")
	repoOut, err := repoCmd.Output()
	if err != nil {
		return "", "", err
	}
	var repoInfo struct {
		Owner struct{ Login string } `json:"owner"`
		Name  string                 `json:"name"`
	}
	if err := json.Unmarshal(repoOut, &repoInfo); err != nil {
		return "", "", err
	}
	return repoInfo.Owner.Login, repoInfo.Name, nil
}

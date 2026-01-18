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
	"github.com/charmbracelet/lipgloss"
)

type boardLane struct {
	Name         string
	Label        string
	Color        string                 // Hex color for GitHub labels (without #)
	DisplayColor lipgloss.AdaptiveColor // Adaptive color for UI display
	Issues       []*Issue
}

type boardLayout struct {
	colWidth    int
	colGap      int
	boardTop    int
	boardLeft   int
	boardWidth  int
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
	issueState    string
	issueLimit    int
	movingIssue   int
	dragging      bool
	moveSticky    bool
	pendingMove   *pendingMove
	debounceSeq   int
	debounceReady bool
	menuOpen      bool
	menuIndex     int
	subMenuOpen   bool
	subMenuIndex  int
	agentMenuOpen bool
	agentIndex    int
	modelMenuOpen bool
	modelIndex    int
}

type issuesLoadedMsg struct {
	issues []Issue
	err    error
}

type issueDetailsMsg struct {
	issue Issue
	err   error
}

type pendingMove struct {
	number int
	target int
	seq    int
}

type debounceMsg struct {
	seq int
}

type assignIssueMsg struct {
	number   int
	assignee string
	err      error
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
	boardCardHeight   = 6
)

type Theme struct {
	Background   lipgloss.AdaptiveColor
	Paper        lipgloss.AdaptiveColor
	Foreground   lipgloss.AdaptiveColor
	Muted        lipgloss.AdaptiveColor
	AccentBlue   lipgloss.AdaptiveColor
	AccentPurple lipgloss.AdaptiveColor
	AccentYellow lipgloss.AdaptiveColor
	AccentGreen  lipgloss.AdaptiveColor
	AccentRed    lipgloss.AdaptiveColor
	AccentCyan   lipgloss.AdaptiveColor
}

var defaultTheme = Theme{
	Background:   lipgloss.AdaptiveColor{Light: "#FAFAFA", Dark: "#21252B"},
	Paper:        lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#282C34"},
	Foreground:   lipgloss.AdaptiveColor{Light: "#383A42", Dark: "#ABB2BF"},
	Muted:        lipgloss.AdaptiveColor{Light: "#A0A1A7", Dark: "#5C6370"},
	AccentBlue:   lipgloss.AdaptiveColor{Light: "#4078F2", Dark: "#61AFEF"},
	AccentPurple: lipgloss.AdaptiveColor{Light: "#A626A4", Dark: "#C678DD"},
	AccentYellow: lipgloss.AdaptiveColor{Light: "#C18401", Dark: "#E5C07B"},
	AccentGreen:  lipgloss.AdaptiveColor{Light: "#50A14F", Dark: "#98C379"},
	AccentRed:    lipgloss.AdaptiveColor{Light: "#E45649", Dark: "#E06C75"},
	AccentCyan:   lipgloss.AdaptiveColor{Light: "#0184BC", Dark: "#56B6C2"},
}

var theme = defaultTheme

var (
	boardTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(theme.AccentYellow)
	boardMutedStyle = lipgloss.NewStyle().Foreground(theme.Muted)
	boardErrorStyle = lipgloss.NewStyle().Foreground(theme.AccentRed)
	boardHelpStyle  = lipgloss.NewStyle().Foreground(theme.AccentPurple)
)

func defaultBoardLanes() []boardLane {
	return []boardLane{
		{Name: "TODO", Label: "TODO", Color: stripHash(theme.AccentBlue.Dark), DisplayColor: theme.AccentBlue},
		{Name: "Doing", Label: "Doing", Color: stripHash(theme.AccentYellow.Dark), DisplayColor: theme.AccentYellow},
		{Name: "Done", Label: "Done", Color: stripHash(theme.AccentGreen.Dark), DisplayColor: theme.AccentGreen},
		{Name: "Blocked", Label: "Blocked", Color: stripHash(theme.AccentRed.Dark), DisplayColor: theme.AccentRed},
	}
}

func runBoard(args []string) int {
	for _, a := range args {
		if a == "--help" || a == "-h" || a == "help" {
			printBoardHelp()
			return 0
		}
	}

	state := "open"
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
		state = "open"
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
	fmt.Println("  space:   quick actions menu (assign to agent, create worktree)")
	fmt.Println("  i:       issue details")
	fmt.Println("  c:       toggle closed issues")
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
		colGap:     2,
		cardHeight: boardCardHeight,
		cardGap:    1,
	}
	layout.boardTop = boardHeaderHeight
	if height > 0 {
		layout.boardHeight = height - boardHeaderHeight - boardHelpHeight
		if layout.boardHeight < 4 {
			layout.boardHeight = 4
		}
	}
	if width > 0 && cols > 0 {
		layout.colWidth, layout.colGap, layout.boardLeft, layout.boardWidth = computeBoardColumns(width, cols)
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

func assignIssueCmd(number int, assignee string) tea.Cmd {
	return func() tea.Msg {
		err := assignIssue(number, assignee)
		return assignIssueMsg{
			number:   number,
			assignee: assignee,
			err:      err,
		}
	}
}

func createWorktreeAndAssignCmd(owner, repo string, number int, agent, model string) tea.Cmd {
	return func() tea.Msg {
		err := createWorktreeAndAssign(owner, repo, number, agent, model)
		return assignIssueMsg{
			number:   number,
			assignee: agent,
			err:      err,
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
		stateLabel := m.issueState
		switch stateLabel {
		case "open":
			stateLabel = "open"
		case "closed":
			stateLabel = "closed"
		case "all":
			stateLabel = "open+closed"
		}
		m.status = fmt.Sprintf("Loaded %d %s issues.", len(msg.issues), stateLabel)
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
	case assignIssueMsg:
		m.busy = false
		if msg.err != nil {
			m.status = fmt.Sprintf("Assign failed: %v", msg.err)
			return m, nil
		}
		m.status = fmt.Sprintf("Assigned #%d to @%s", msg.number, msg.assignee)
		// Reload issue details if this issue is currently open
		if m.detailsIssue != nil && m.detailsIssue.Number == msg.number {
			return m, loadIssueDetailsCmd(msg.number)
		}
		return m, nil
	case issueDetailsMsg:
		m.detailsLoad = false
		if msg.err != nil {
			m.status = fmt.Sprintf("Details failed: %v", msg.err)
			return m, nil
		}
		var updated *Issue
		if issue := m.findIssueByNumber(msg.issue.Number); issue != nil {
			issue.Title = msg.issue.Title
			issue.Body = msg.issue.Body
			issue.URL = msg.issue.URL
			issue.State = msg.issue.State
			issue.StateReason = msg.issue.StateReason
			issue.CreatedAt = msg.issue.CreatedAt
			issue.UpdatedAt = msg.issue.UpdatedAt
			issue.ClosedAt = msg.issue.ClosedAt
			issue.Author = msg.issue.Author
			issue.Assignees = msg.issue.Assignees
			issue.Milestone = msg.issue.Milestone
			issue.Labels = msg.issue.Labels
			issue.Comments = msg.issue.Comments
			issue.LinkedPRs = msg.issue.LinkedPRs
			issue.DetailsLoaded = true
			updated = issue
		}
		if m.detailsIssue != nil && m.detailsIssue.Number == msg.issue.Number {
			if updated != nil {
				m.detailsIssue = updated
			}
			m.detailsView.SetContent(m.detailsContent(m.detailsIssue))
		}
		return m, nil
	case tea.MouseMsg:
		if m.detailsOpen {
			var cmd tea.Cmd
			m.detailsView, cmd = m.detailsView.Update(msg)
			return m, cmd
		}
		if m.loading {
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
	// Handle menu navigation if menu is open
	if m.menuOpen {
		return m.updateMenuKeys(msg)
	}
	if m.agentMenuOpen {
		return m.updateAgentMenuKeys(msg)
	}
	if m.modelMenuOpen {
		return m.updateModelMenuKeys(msg)
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case " ":
		// Open quick actions menu
		issue := m.currentIssue()
		if issue != nil && !m.loading && !m.busy {
			m.menuOpen = true
			m.menuIndex = 0
			m.status = "Quick Actions Menu"
			return m, nil
		}
	case "c":
		if m.loading || m.busy {
			return m, nil
		}
		if m.moving || m.dragging {
			m.moving = false
			m.movingIssue = 0
			m.moveSticky = false
			m.dragging = false
		}
		if m.issueState == "open" {
			m.issueState = "all"
			m.status = "Showing open + closed issues..."
		} else {
			m.issueState = "open"
			m.status = "Showing open issues..."
		}
		m.loading = true
		return m, loadIssuesCmd(m.issueState, m.issueLimit)
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
		if m.moving {
			return m, nil
		}
		m.moveCursor(-1)
	case "down", "j":
		if m.moving {
			return m, nil
		}
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

func (m *boardModel) updateMenuKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.menuOpen = false
		m.menuIndex = 0
		m.status = ""
		return m, nil
	case "up", "k":
		if m.menuIndex > 0 {
			m.menuIndex--
		}
	case "down", "j":
		if m.menuIndex < 1 { // We have 2 menu items (0 and 1)
			m.menuIndex++
		}
	case "enter":
		issue := m.currentIssue()
		if issue == nil {
			m.menuOpen = false
			return m, nil
		}
		
		switch m.menuIndex {
		case 0:
			// Assign to Copilot Web
			m.menuOpen = false
			m.busy = true
			m.status = fmt.Sprintf("Assigning #%d to @copilot...", issue.Number)
			return m, assignIssueCmd(issue.Number, "copilot")
		case 1:
			// Create local worktree + enter
			m.menuOpen = false
			m.agentMenuOpen = true
			m.agentIndex = 0
			m.status = "Select Agent"
			return m, nil
		}
	}
	return m, nil
}

func (m *boardModel) updateAgentMenuKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.agentMenuOpen = false
		m.agentIndex = 0
		m.status = ""
		return m, nil
	case "up", "k":
		if m.agentIndex > 0 {
			m.agentIndex--
		}
	case "down", "j":
		if m.agentIndex < 2 { // We have 3 agents (0, 1, 2)
			m.agentIndex++
		}
	case "enter":
		// Move to model selection
		m.agentMenuOpen = false
		m.modelMenuOpen = true
		m.modelIndex = 0
		m.status = "Select Model"
		return m, nil
	}
	return m, nil
}

func (m *boardModel) updateModelMenuKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.modelMenuOpen = false
		m.modelIndex = 0
		m.agentMenuOpen = true
		m.status = "Select Agent"
		return m, nil
	case "up", "k":
		if m.modelIndex > 0 {
			m.modelIndex--
		}
	case "down", "j":
		if m.modelIndex < 2 { // We have 3 models (0, 1, 2)
			m.modelIndex++
		}
	case "enter":
		issue := m.currentIssue()
		if issue == nil {
			m.modelMenuOpen = false
			return m, nil
		}
		
		agents := []string{"copilot", "claude", "codex"}
		models := []string{"standard", "advanced", "reasoning"}
		
		agent := agents[m.agentIndex]
		model := models[m.modelIndex]
		
		m.modelMenuOpen = false
		m.busy = true
		m.status = fmt.Sprintf("Creating worktree and assigning #%d to %s (%s)...", issue.Number, agent, model)
		
		return m, createWorktreeAndAssignCmd(m.owner, m.repo, issue.Number, agent, model)
	}
	return m, nil
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
	m.detailsView.SetContent(m.detailsContent(m.detailsIssue))
}

func (m *boardModel) openDetails(issue *Issue) tea.Cmd {
	m.detailsOpen = true
	m.detailsIssue = issue
	m.detailsLoad = issue == nil || !issue.DetailsLoaded
	m.detailsView = viewport.New(0, 0)
	m.detailsView.MouseWheelEnabled = true
	m.resizeDetails()
	m.detailsView.GotoTop()
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
	if m.menuOpen {
		return m.menuViewString()
	}
	if m.agentMenuOpen {
		return m.agentMenuViewString()
	}
	if m.modelMenuOpen {
		return m.modelMenuViewString()
	}
	header, status, help := m.renderChrome()
	m.applyLayout(header, status, help)
	board := m.renderBoard()
	content := lipgloss.JoinVertical(lipgloss.Left, header, status, board, help)
	return content
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
	helpText := "Arrows: move  Enter: pick/drop  Space: actions  i: details  c: closed  r: refresh  q: quit"
	if m.moving {
		helpText = "Move mode: left/right lane  Enter: drop  Esc: cancel  q: quit"
	}

	headerStyle := boardTitleStyle.Background(theme.Background).Padding(0, 1)
	statusStyle = statusStyle.Background(theme.Background).Padding(0, 1)
	helpStyle := boardHelpStyle.Background(theme.Background).Padding(0, 1)
	if m.width > 0 {
		contentWidth := m.width - 2
		if contentWidth < 1 {
			contentWidth = 1
		}
		headerText = truncateString(headerText, contentWidth)
		statusText = truncateString(statusText, contentWidth)
		helpText = truncateString(helpText, contentWidth)

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
	m.layout.colWidth, m.layout.colGap, m.layout.boardLeft, m.layout.boardWidth = computeBoardColumns(m.width, cols)
	m.layout.cardHeight = boardCardHeight
}

func (m boardModel) renderBoard() string {
	if m.layout.boardHeight < 4 {
		return boardMutedStyle.Background(theme.Background).Padding(0, 1).Width(m.width).Render("Window too small for board.")
	}
	if m.layout.colWidth <= 0 || len(m.lanes) == 0 || m.width <= 0 {
		return ""
	}
	columns := make([]string, 0, len(m.lanes))
	for i := range m.lanes {
		columns = append(columns, m.renderColumn(i))
	}
	gap := boardBackgroundBlock(m.layout.colGap, m.layout.boardHeight)
	left := boardBackgroundBlock(m.layout.boardLeft, m.layout.boardHeight)
	rightWidth := m.width - m.layout.boardLeft - m.layout.boardWidth
	right := boardBackgroundBlock(rightWidth, m.layout.boardHeight)

	parts := make([]string, 0, 2+len(m.lanes)*2)
	if left != "" {
		parts = append(parts, left)
	}
	for i, col := range columns {
		if i > 0 && gap != "" {
			parts = append(parts, gap)
		}
		parts = append(parts, col)
	}
	if right != "" {
		parts = append(parts, right)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
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
	if m.layout.boardHeight <= 1 {
		return header
	}

	blank := boardBackgroundLine(m.layout.colWidth)
	lines := make([]string, 0, m.layout.boardHeight)
	lines = append(lines, header)

	cardAreaHeight := m.layout.boardHeight - 1
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
		moving := m.moving && lane.Issues[i].Number == m.movingIssue
		card := renderCard(lane.Issues[i], selected, moving, m.layout.colWidth, m.layout.cardHeight, m.lanes)
		lines = append(lines, strings.Split(card, "\n")...)
		if i < end-1 {
			for g := 0; g < m.layout.cardGap; g++ {
				lines = append(lines, blank)
			}
		}
	}
	for len(lines) < m.layout.boardHeight {
		lines = append(lines, blank)
	}
	if len(lines) > m.layout.boardHeight {
		lines = lines[:m.layout.boardHeight]
	}
	return strings.Join(lines, "\n")
}

func renderCard(issue *Issue, selected bool, moving bool, width int, height int, lanes []boardLane) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	contentWidth := width - 2
	if contentWidth < 1 {
		contentWidth = 1
	}

	cardBg := theme.Paper
	if selected {
		cardBg = lipgloss.AdaptiveColor{Light: "#EDEDED", Dark: "#2C313C"}
	}
	if moving {
		cardBg = lipgloss.AdaptiveColor{Light: "#F2EAF7", Dark: "#2A2730"}
	}

	title := fmt.Sprintf("#%d %s", issue.Number, issue.Title)
	title = truncateString(title, contentWidth)
	badges := renderIssueBadges(issue, lanes, contentWidth)

	var metaParts []string
	if issue.Author != nil && issue.Author.Login != "" {
		metaParts = append(metaParts, "@"+issue.Author.Login)
	}
	if updated := formatTimeShort(issue.UpdatedAt); updated != "" {
		metaParts = append(metaParts, updated)
	}
	if issue.State != "" {
		state := strings.ToLower(issue.State)
		if state != "open" {
			metaParts = append(metaParts, state)
		}
	}
	// Add PR indicator
	if len(issue.LinkedPRs) > 0 {
		metaParts = append(metaParts, fmt.Sprintf("PR:%d", len(issue.LinkedPRs)))
	}
	if badges != "" {
		metaParts = append(metaParts, badges)
	}
	meta := truncateString(strings.Join(metaParts, " · "), contentWidth)

	snippetLines := height - 2
	if snippetLines < 0 {
		snippetLines = 0
	}
	snippet := summarizeText(issue.Body)
	body := wrapText(snippet, contentWidth, snippetLines)

	titleColor := theme.AccentBlue
	if selected {
		titleColor = theme.AccentYellow
	}
	if moving {
		titleColor = theme.AccentPurple
	}

	titleLineStyle := lipgloss.NewStyle().
		Background(cardBg).
		Foreground(titleColor).
		Bold(true).
		Padding(0, 1).
		Width(width)
	metaLineStyle := lipgloss.NewStyle().
		Background(cardBg).
		Foreground(theme.Muted).
		Padding(0, 1).
		Width(width)
	bodyLineStyle := lipgloss.NewStyle().
		Background(cardBg).
		Foreground(theme.Foreground).
		Padding(0, 1).
		Width(width)

	lines := make([]string, 0, height)
	lines = append(lines, titleLineStyle.Render(title))
	lines = append(lines, metaLineStyle.Render(meta))
	for _, l := range body {
		lines = append(lines, bodyLineStyle.Render(l))
	}
	for len(lines) < height {
		lines = append(lines, bodyLineStyle.Render(""))
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
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
		return ""
	}
	line := strings.Join(parts, ", ")
	return truncateString(line, width)
}

func laneHeaderStyle(lane boardLane, active bool, width int) string {
	name := fmt.Sprintf("%s (%d)", lane.Name, len(lane.Issues))
	contentWidth := width - 2
	if contentWidth < 1 {
		contentWidth = 1
	}
	name = truncateString(name, contentWidth)
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lane.DisplayColor).
		Background(theme.Background).
		Padding(0, 1).
		Width(width)
	if active {
		style = style.Underline(true)
	}
	return style.Render(name)
}

func (m *boardModel) detailsContent(issue *Issue) string {
	width := m.detailsView.Width
	if width <= 0 {
		width = 80
	}
	paper := theme.Paper
	baseLine := lipgloss.NewStyle().Background(paper).Foreground(theme.Foreground).Width(width)
	mutedLine := lipgloss.NewStyle().Background(paper).Foreground(theme.Muted).Width(width)
	titleLine := lipgloss.NewStyle().Background(paper).Foreground(theme.AccentYellow).Bold(true).Width(width)
	sectionLine := lipgloss.NewStyle().Background(paper).Foreground(theme.AccentCyan).Bold(true).Width(width)
	commentHeaderLine := lipgloss.NewStyle().Background(paper).Foreground(theme.AccentGreen).Bold(true).Width(width)
	keyStyle := lipgloss.NewStyle().Background(paper).Foreground(theme.Muted).Bold(true)
	valStyle := lipgloss.NewStyle().Background(paper).Foreground(theme.Foreground)

	kv := func(key, value string) string {
		keyWidth := 12
		if keyWidth >= width-1 {
			keyWidth = max(1, width/3)
		}
		keyText := truncateString(key+":", keyWidth-1)
		keyText = padRightRunes(keyText, keyWidth-1) + " "
		valWidth := width - keyWidth
		if valWidth < 1 {
			valWidth = 1
		}
		return keyStyle.Width(keyWidth).Render(keyText) + valStyle.Width(valWidth).Render(truncateString(value, valWidth))
	}

	if issue == nil {
		return mutedLine.Render("No issue selected.")
	}

	lines := make([]string, 0, 128)
	lines = append(lines, titleLine.Render(truncateString(fmt.Sprintf("#%d %s", issue.Number, issue.Title), width)))
	if m.detailsLoad && !issue.DetailsLoaded {
		lines = append(lines, mutedLine.Render("Loading details..."))
	}
	lines = append(lines, baseLine.Render(""))

	state := strings.ToLower(issue.State)
	if state == "" {
		state = "unknown"
	}
	lines = append(lines, kv("State", state))
	if issue.StateReason != "" {
		lines = append(lines, kv("Reason", issue.StateReason))
	}
	if issue.Author != nil && issue.Author.Login != "" {
		author := "@" + issue.Author.Login
		if issue.Author.Name != "" {
			author += " (" + issue.Author.Name + ")"
		}
		lines = append(lines, kv("Author", author))
	}
	if len(issue.Assignees) > 0 {
		var assignees []string
		for _, a := range issue.Assignees {
			if a.Login != "" {
				assignees = append(assignees, "@"+a.Login)
			}
		}
		if len(assignees) > 0 {
			lines = append(lines, kv("Assignees", strings.Join(assignees, ", ")))
		}
	}
	if issue.Milestone != nil && issue.Milestone.Title != "" {
		lines = append(lines, kv("Milestone", issue.Milestone.Title))
	}
	if !issue.CreatedAt.IsZero() {
		lines = append(lines, kv("Created", formatTimeLong(issue.CreatedAt)))
	}
	if !issue.UpdatedAt.IsZero() {
		lines = append(lines, kv("Updated", formatTimeLong(issue.UpdatedAt)))
	}
	if issue.ClosedAt != nil && !issue.ClosedAt.IsZero() {
		lines = append(lines, kv("Closed", formatTimeLong(*issue.ClosedAt)))
	}
	if issue.URL != "" {
		lines = append(lines, kv("URL", issue.URL))
	}
	if len(issue.Labels) > 0 {
		var labels []string
		for _, l := range issue.Labels {
			if l.Name != "" {
				labels = append(labels, l.Name)
			}
		}
		if len(labels) > 0 {
			lines = append(lines, kv("Labels", strings.Join(labels, ", ")))
		}
	}

	lines = append(lines, baseLine.Render(""))
	lines = append(lines, sectionLine.Render("Description"))
	lines = append(lines, baseLine.Render(""))

	body := issue.Body
	if body == "" {
		if m.detailsLoad && !issue.DetailsLoaded {
			lines = append(lines, mutedLine.Render("Loading description..."))
		} else {
			lines = append(lines, mutedLine.Render("No description."))
		}
	} else {
		for _, l := range wrapTextBlock(body, width) {
			lines = append(lines, baseLine.Render(l))
		}
	}

	// Add linked PRs section
	lines = append(lines, baseLine.Render(""))
	lines = append(lines, sectionLine.Render(fmt.Sprintf("Linked Pull Requests (%d)", len(issue.LinkedPRs))))
	lines = append(lines, baseLine.Render(""))

	if len(issue.LinkedPRs) == 0 {
		if m.detailsLoad && !issue.DetailsLoaded {
			lines = append(lines, mutedLine.Render("Loading linked PRs..."))
		} else {
			lines = append(lines, mutedLine.Render("No linked pull requests."))
		}
	} else {
		for _, pr := range issue.LinkedPRs {
			prState := strings.ToLower(pr.State)
			stateColor := theme.AccentGreen
			if prState == "closed" || prState == "merged" {
				stateColor = theme.AccentPurple
			}
			stateStyle := lipgloss.NewStyle().Background(paper).Foreground(stateColor).Bold(true)
			prLine := fmt.Sprintf("#%d: %s", pr.Number, pr.Title)
			lines = append(lines, stateStyle.Width(width).Render(truncateString(prLine, width)))
			if pr.URL != "" {
				lines = append(lines, mutedLine.Render(truncateString(pr.URL, width)))
			}
			lines = append(lines, baseLine.Render(""))
		}
	}

	lines = append(lines, baseLine.Render(""))
	lines = append(lines, sectionLine.Render(fmt.Sprintf("Comments (%d)", len(issue.Comments))))
	lines = append(lines, baseLine.Render(""))

	if len(issue.Comments) == 0 {
		if m.detailsLoad && !issue.DetailsLoaded {
			lines = append(lines, mutedLine.Render("Loading comments..."))
		} else {
			lines = append(lines, mutedLine.Render("No comments."))
		}
	} else {
		for i, c := range issue.Comments {
			author := "unknown"
			if c.Author != nil && c.Author.Login != "" {
				author = "@" + c.Author.Login
			}
			when := ""
			if !c.CreatedAt.IsZero() {
				when = formatTimeLong(c.CreatedAt)
			}
			header := fmt.Sprintf("%d. %s", i+1, author)
			if when != "" {
				header = fmt.Sprintf("%d. %s · %s", i+1, author, when)
			}
			lines = append(lines, commentHeaderLine.Render(truncateString(header, width)))
			if c.Body == "" {
				lines = append(lines, mutedLine.Render("No comment text."))
			} else {
				for _, l := range wrapTextBlock(c.Body, width) {
					lines = append(lines, baseLine.Render(l))
				}
			}
			if i < len(issue.Comments)-1 {
				lines = append(lines, baseLine.Render(""))
			}
		}
	}

	return strings.Join(lines, "\n")
}

func (m boardModel) detailsViewString() string {
	width := m.width
	height := m.height
	if width <= 0 || height <= 0 {
		return ""
	}
	content := m.detailsView.View()
	modalStyle := lipgloss.NewStyle().
		Background(theme.Paper).
		Foreground(theme.Foreground).
		Padding(1, 2).
		Width(m.detailsView.Width + 4).
		Height(m.detailsView.Height + 2)
	modal := modalStyle.Render(content)
	modalWidth := lipgloss.Width(modal)
	modalHeight := lipgloss.Height(modal)
	x0 := (width - modalWidth) / 2
	y0 := (height - modalHeight) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	bgLine := boardBackgroundLine(width)
	leftPad := boardBackgroundLine(x0)
	rightPad := boardBackgroundLine(width - x0 - modalWidth)
	modalLines := strings.Split(modal, "\n")

	lines := make([]string, 0, height)
	for y := 0; y < height; y++ {
		if y < y0 || y-y0 >= len(modalLines) {
			lines = append(lines, bgLine)
			continue
		}
		lines = append(lines, leftPad+modalLines[y-y0]+rightPad)
	}
	return strings.Join(lines, "\n")
}

func (m *boardModel) menuViewString() string {
	width := m.width
	height := m.height
	if width <= 0 || height <= 0 {
		return ""
	}

	issue := m.currentIssue()
	if issue == nil {
		return ""
	}

	menuItems := []string{
		"Assign to Copilot Web",
		"Create local worktree + assign",
	}

	var menuLines []string
	menuLines = append(menuLines, fmt.Sprintf("Quick Actions - Issue #%d", issue.Number))
	menuLines = append(menuLines, "")
	for i, item := range menuItems {
		if i == m.menuIndex {
			menuLines = append(menuLines, "> "+item)
		} else {
			menuLines = append(menuLines, "  "+item)
		}
	}
	menuLines = append(menuLines, "")
	menuLines = append(menuLines, "↑↓: navigate  Enter: select  Esc: cancel")

	content := strings.Join(menuLines, "\n")
	modalStyle := lipgloss.NewStyle().
		Background(theme.Paper).
		Foreground(theme.Foreground).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.AccentBlue)
	modal := modalStyle.Render(content)
	return m.renderModalOverBoard(modal)
}

func (m *boardModel) agentMenuViewString() string {
	width := m.width
	height := m.height
	if width <= 0 || height <= 0 {
		return ""
	}

	issue := m.currentIssue()
	if issue == nil {
		return ""
	}

	agents := []string{"Copilot", "Claude", "Codex"}

	var menuLines []string
	menuLines = append(menuLines, fmt.Sprintf("Select Agent - Issue #%d", issue.Number))
	menuLines = append(menuLines, "")
	for i, agent := range agents {
		if i == m.agentIndex {
			menuLines = append(menuLines, "> "+agent)
		} else {
			menuLines = append(menuLines, "  "+agent)
		}
	}
	menuLines = append(menuLines, "")
	menuLines = append(menuLines, "↑↓: navigate  Enter: next  Esc: back")

	content := strings.Join(menuLines, "\n")
	modalStyle := lipgloss.NewStyle().
		Background(theme.Paper).
		Foreground(theme.Foreground).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.AccentPurple)
	modal := modalStyle.Render(content)
	return m.renderModalOverBoard(modal)
}

func (m *boardModel) modelMenuViewString() string {
	width := m.width
	height := m.height
	if width <= 0 || height <= 0 {
		return ""
	}

	issue := m.currentIssue()
	if issue == nil {
		return ""
	}

	models := []string{"Standard", "Advanced", "Reasoning"}

	var menuLines []string
	menuLines = append(menuLines, fmt.Sprintf("Select Model - Issue #%d", issue.Number))
	menuLines = append(menuLines, "")
	for i, model := range models {
		if i == m.modelIndex {
			menuLines = append(menuLines, "> "+model)
		} else {
			menuLines = append(menuLines, "  "+model)
		}
	}
	menuLines = append(menuLines, "")
	menuLines = append(menuLines, "↑↓: navigate  Enter: create  Esc: back")

	content := strings.Join(menuLines, "\n")
	modalStyle := lipgloss.NewStyle().
		Background(theme.Paper).
		Foreground(theme.Foreground).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.AccentGreen)
	modal := modalStyle.Render(content)
	return m.renderModalOverBoard(modal)
}

func (m *boardModel) renderModalOverBoard(modal string) string {
	width := m.width
	height := m.height
	
	modalWidth := lipgloss.Width(modal)
	modalHeight := lipgloss.Height(modal)
	x0 := (width - modalWidth) / 2
	y0 := (height - modalHeight) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	bgLine := boardBackgroundLine(width)
	leftPad := boardBackgroundLine(x0)
	rightPad := boardBackgroundLine(width - x0 - modalWidth)
	modalLines := strings.Split(modal, "\n")

	lines := make([]string, 0, height)
	for y := 0; y < height; y++ {
		if y < y0 || y-y0 >= len(modalLines) {
			lines = append(lines, bgLine)
			continue
		}
		lines = append(lines, leftPad+modalLines[y-y0]+rightPad)
	}
	return strings.Join(lines, "\n")
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
	if x < m.layout.boardLeft || x >= m.layout.boardLeft+m.layout.boardWidth {
		return -1
	}
	x -= m.layout.boardLeft
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
	return boardCardHeight
}

func computeBoardColumns(width, cols int) (colWidth int, colGap int, boardLeft int, boardWidth int) {
	if width <= 0 || cols <= 0 {
		return 0, 0, 0, 0
	}
	colGap = 2
	if width < cols*14 {
		colGap = 1
	}
	if width < cols*12 {
		colGap = 0
	}
	available := width - colGap*(cols-1)
	if available < cols {
		available = cols
	}
	colWidth = available / cols
	boardWidth = colWidth*cols + colGap*(cols-1)
	if boardWidth > width {
		boardWidth = width
	}
	boardLeft = (width - boardWidth) / 2
	if boardLeft < 0 {
		boardLeft = 0
	}
	return colWidth, colGap, boardLeft, boardWidth
}

func boardBackgroundBlock(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	line := boardBackgroundLine(width)
	lines := make([]string, height)
	for i := range lines {
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

func boardBackgroundLine(width int) string {
	if width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Background(theme.Background).
		Width(width).
		Render("")
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
	args := []string{"issue", "list", "--state", state, "--limit", fmt.Sprintf("%d", limit), "--json", "number,title,labels,state,url,body,author,createdAt,updatedAt,closedAt"}
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

func assignIssue(number int, assignee string) error {
	args := []string{"issue", "edit", fmt.Sprintf("%d", number), "--add-assignee", assignee}
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

func createWorktreeAndAssign(owner, repo string, number int, agent, model string) error {
	// Create branch name from issue number
	branchName := fmt.Sprintf("issue-%d-%s", number, agent)
	
	// Create worktree in a subdirectory
	worktreePath := fmt.Sprintf("../worktrees/%s", branchName)
	
	// Create worktree with new branch
	cmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("failed to create worktree: %v", err)
		}
		return fmt.Errorf("failed to create worktree: %v: %s", err, msg)
	}
	
	// Assign the issue
	if err := assignIssue(number, agent); err != nil {
		return fmt.Errorf("worktree created but assign failed: %v", err)
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func padRightRunes(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) >= width {
		return string(runes[:width])
	}
	return string(runes) + strings.Repeat(" ", width-len(runes))
}

func formatTimeLong(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}

func formatTimeShort(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	now := time.Now()
	d := now.Sub(t)
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		if t.Year() == now.Year() {
			return t.Local().Format("Jan 2")
		}
		return t.Local().Format("2006-01-02")
	}
}

func summarizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	var parts []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts = append(parts, line)
		if len(parts) >= 6 {
			break
		}
	}
	out := strings.Join(parts, " ")
	return collapseSpaces(out)
}

func collapseSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if space {
				continue
			}
			space = true
			b.WriteRune(' ')
			continue
		}
		space = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func wrapText(text string, width int, maxLines int) []string {
	if maxLines <= 0 || width <= 0 {
		return nil
	}
	text = collapseSpaces(text)
	if text == "" {
		return nil
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	lines, truncated := wrapWords(words, width, maxLines)
	if truncated && len(lines) > 0 {
		last := lines[len(lines)-1]
		if width <= 1 {
			lines[len(lines)-1] = "…"
		} else {
			lines[len(lines)-1] = truncateString(last, width-1) + "…"
		}
	}
	return lines
}

func wrapTextBlock(text string, width int) []string {
	if width <= 0 {
		return nil
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	linesIn := strings.Split(text, "\n")
	out := make([]string, 0, len(linesIn))
	for _, line := range linesIn {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			out = append(out, "")
			continue
		}
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") || strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "    ") {
			out = append(out, truncateString(line, width))
			continue
		}

		firstPrefix, nextPrefix, rest := splitWrapPrefix(line)
		availFirst := width - len([]rune(firstPrefix))
		availNext := width - len([]rune(nextPrefix))
		if availFirst < 1 {
			availFirst = 1
		}
		if availNext < 1 {
			availNext = 1
		}

		words := strings.Fields(rest)
		if len(words) == 0 {
			out = append(out, truncateString(line, width))
			continue
		}

		wrapped, _ := wrapWords(words, availFirst, 0)
		for i, w := range wrapped {
			prefix := firstPrefix
			avail := availFirst
			if i > 0 {
				prefix = nextPrefix
				avail = availNext
			}
			out = append(out, prefix+truncateString(w, avail))
		}
	}
	return out
}

func splitWrapPrefix(line string) (firstPrefix string, nextPrefix string, rest string) {
	leadingSpaces := 0
	for leadingSpaces < len(line) && line[leadingSpaces] == ' ' {
		leadingSpaces++
	}
	indent := strings.Repeat(" ", leadingSpaces)
	trimmed := line[leadingSpaces:]

	switch {
	case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ "):
		firstPrefix = indent + trimmed[:2]
		nextPrefix = indent + "  "
		rest = strings.TrimSpace(trimmed[2:])
		return firstPrefix, nextPrefix, rest
	case strings.HasPrefix(trimmed, "> "):
		firstPrefix = indent + "> "
		nextPrefix = indent + "  "
		rest = strings.TrimSpace(trimmed[2:])
		return firstPrefix, nextPrefix, rest
	default:
		if n := parseNumberedListPrefix(trimmed); n > 0 {
			firstPrefix = indent + trimmed[:n]
			nextPrefix = indent + strings.Repeat(" ", n)
			rest = strings.TrimSpace(trimmed[n:])
			return firstPrefix, nextPrefix, rest
		}
		firstPrefix = indent
		nextPrefix = indent
		rest = strings.TrimSpace(trimmed)
		return firstPrefix, nextPrefix, rest
	}
}

func parseNumberedListPrefix(s string) int {
	if len(s) < 3 {
		return 0
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i+1 >= len(s) {
		return 0
	}
	if s[i] != '.' || s[i+1] != ' ' {
		return 0
	}
	return i + 2
}

func wrapWords(words []string, width int, maxLines int) ([]string, bool) {
	if width <= 0 || len(words) == 0 {
		return nil, false
	}

	lines := make([]string, 0, max(4, maxLines))
	var line []rune
	truncated := false

	flush := func() {
		if len(line) == 0 {
			return
		}
		lines = append(lines, string(line))
		line = line[:0]
	}

	for _, w := range words {
		if maxLines > 0 && len(lines) >= maxLines {
			truncated = true
			break
		}
		word := []rune(w)
		if len(word) > width {
			word = []rune(truncateString(string(word), width))
		}
		if len(line) == 0 {
			line = append(line, word...)
			continue
		}
		if len(line)+1+len(word) <= width {
			line = append(line, ' ')
			line = append(line, word...)
			continue
		}
		flush()
		if maxLines > 0 && len(lines) >= maxLines {
			truncated = true
			break
		}
		line = append(line, word...)
	}
	flush()

	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		truncated = true
	}

	return lines, truncated
}

func stripHash(color string) string {
	if len(color) > 0 && color[0] == '#' {
		return color[1:]
	}
	return color
}

func loadIssueDetailsCmd(number int) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("gh", "issue", "view", fmt.Sprintf("%d", number), "--json", "number,title,state,stateReason,url,body,author,assignees,milestone,labels,createdAt,updatedAt,closedAt,comments")
		out, err := cmd.Output()
		if err != nil {
			return issueDetailsMsg{issue: Issue{Number: number}, err: err}
		}
		var issue Issue
		if err := json.Unmarshal(out, &issue); err != nil {
			return issueDetailsMsg{issue: Issue{Number: number}, err: err}
		}
		
		// Fetch linked PRs using GraphQL
		linkedPRs, err := fetchLinkedPRs(number)
		if err == nil {
			issue.LinkedPRs = linkedPRs
		}
		
		return issueDetailsMsg{issue: issue}
	}
}

func fetchLinkedPRs(issueNumber int) ([]PullRequest, error) {
	// Use GraphQL to find PRs that reference this issue
	query := fmt.Sprintf(`{
		repository(owner: "", name: "") {
			issue(number: %d) {
				timelineItems(itemTypes: [CROSS_REFERENCED_EVENT], first: 50) {
					nodes {
						... on CrossReferencedEvent {
							source {
								... on PullRequest {
									number
									title
									url
									state
								}
							}
						}
					}
				}
			}
		}
	}`, issueNumber)
	
	// Get repo info
	owner, repo, err := getRepoInfo()
	if err != nil {
		return nil, err
	}
	
	// Inject owner and repo into query
	query = strings.Replace(query, `owner: ""`, fmt.Sprintf(`owner: "%s"`, owner), 1)
	query = strings.Replace(query, `name: ""`, fmt.Sprintf(`name: "%s"`, repo), 1)
	
	cmd := exec.Command("gh", "api", "graphql", "-f", "query="+query)
	out, err := cmd.Output()
	if err != nil {
		// Return empty list on error - not critical
		return []PullRequest{}, nil
	}
	
	var resp struct {
		Data struct {
			Repository struct {
				Issue struct {
					TimelineItems struct {
						Nodes []struct {
							Source *struct {
								Number int    `json:"number"`
								Title  string `json:"title"`
								URL    string `json:"url"`
								State  string `json:"state"`
							} `json:"source"`
						} `json:"nodes"`
					} `json:"timelineItems"`
				} `json:"issue"`
			} `json:"repository"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(out, &resp); err != nil {
		return []PullRequest{}, nil
	}
	
	var prs []PullRequest
	seen := make(map[int]bool)
	for _, node := range resp.Data.Repository.Issue.TimelineItems.Nodes {
		if node.Source != nil && !seen[node.Source.Number] {
			seen[node.Source.Number] = true
			prs = append(prs, PullRequest{
				Number: node.Source.Number,
				Title:  node.Source.Title,
				URL:    node.Source.URL,
				State:  node.Source.State,
			})
		}
	}
	
	return prs, nil
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

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"

	"github.com/charmbracelet/lipgloss"
)

type Label struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type ParentRef struct {
	Number int `json:"number"`
}

type Issue struct {
	Number int        `json:"number"`
	Title  string     `json:"title"`
	Body   string     `json:"body"`
	URL    string     `json:"url"`
	State  string     `json:"state"`
	Labels []Label    `json:"labels"`
	Parent *ParentRef `json:"parent"`
}

type Node struct {
	Issue    Issue
	Parent   int
	Children []*Node
}

func parseParent(body string) int {
	re := regexp.MustCompile(`(?i)^Parent(\s+issue)?:\s*#?(\d+)`)
	for _, line := range regexp.MustCompile("\r?\n").Split(body, -1) {
		m := re.FindStringSubmatch(line)
		if len(m) == 3 {
			return atoi(m[2])
		}
	}
	return 0
}

func atoi(s string) int {
	var n int
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	// naive search to avoid importing strings
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if s[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func lower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		} else {
			b[i] = c
		}
	}
	return string(b)
}

func isBug(is Issue) bool {
	// label named "bug" (case-insensitive) OR title starts with "Bug:" or "Bug " (case-insensitive)
	for _, l := range is.Labels {
		if lower(l.Name) == "bug" {
			return true
		}
	}
	lt := lower(is.Title)
	return indexOf(lt, "bug:") == 0 || indexOf(lt, "bug ") == 0
}

func isBlocked(is Issue) bool {
	// label named "blocked" (case-insensitive) OR title starts with "Blocked:" or "Blocked " (case-insensitive)
	for _, l := range is.Labels {
		if lower(l.Name) == "blocked" {
			return true
		}
	}
	lt := lower(is.Title)
	return indexOf(lt, "blocked:") == 0 || indexOf(lt, "blocked ") == 0
}

func maxIssueNumber(nodes []*Node) int {
	max := 0
	for _, n := range nodes {
		if n.Issue.Number > max {
			max = n.Issue.Number
		}
		if childMax := maxIssueNumber(n.Children); childMax > max {
			max = childMax
		}
	}
	return max
}

func numDigits(n int) int {
	if n == 0 {
		return 1
	}
	d := 0
	for n > 0 {
		d++
		n /= 10
	}
	return d
}

func maxIssueNumberMap(nodes map[int]*Node) int {
	max := 0
	for _, n := range nodes {
		if n.Issue.Number > max {
			max = n.Issue.Number
		}
	}
	return max
}

func renderSection(header string, nodes []*Node, width int, showLink bool, maxWidth int) {
	if len(nodes) == 0 {
		return
	}
	fmt.Println(headerStyle.Render(header))
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Issue.Number < nodes[j].Issue.Number })
	for _, n := range nodes {
		renderNode(n, "", false, width, showLink, maxWidth, true)
	}
	fmt.Println()
}

func calcNodeWidth(n *Node, prefix string, idWidth int, isRoot bool) int {
	connector := "├─ "
	if isRoot {
		connector = ""
	}
	full := n.Issue.Title
	labels := renderSpecialLabels(n.Issue)
	base := prefix + connector + fmt.Sprintf("○ %*d", idWidth, n.Issue.Number) + " " + full
	return lipgloss.Width(base) + lipgloss.Width(labels)
}

func calcMaxWidth(nodes []*Node, prefix string, idWidth int, isRoot bool) int {
	max := 0
	for _, n := range nodes {
		w := calcNodeWidth(n, prefix, idWidth, isRoot)
		if w > max {
			max = w
		}
		nextPrefix := "   "
		if !isRoot {
			nextPrefix = prefix + "│  "
		}
		if childMax := calcMaxWidth(n.Children, nextPrefix, idWidth, false); childMax > max {
			max = childMax
		}
	}
	return max
}

func renderTree(roots []*Node, width int, showLink bool) {
	sort.Slice(roots, func(i, j int) bool { return roots[i].Issue.Number < roots[j].Issue.Number })
	maxWidth := 0
	if showLink {
		maxWidth = calcMaxWidth(roots, "", width, true)
	}
	for _, r := range roots {
		renderNode(r, "", false, width, showLink, maxWidth, true)
	}
}

// Color definitions using AdaptiveColor for automatic light/dark switching
var (
	colorBranch      = lipgloss.AdaptiveColor{Light: "#A0A1A7", Dark: "#5C6370"} // comment gray
	colorOpenID      = lipgloss.AdaptiveColor{Light: "#50A14F", Dark: "#98C379"} // green
	colorClosedID    = lipgloss.AdaptiveColor{Light: "#A0A1A7", Dark: "#5C6370"} // gray (same as branch)
	colorTitlePrefix = lipgloss.AdaptiveColor{Light: "#4078F2", Dark: "#61AFEF"} // blue
	colorTitleSuffix = lipgloss.AdaptiveColor{Light: "#383A42", Dark: "#D4D8DE"} // darker/lighter gray
	colorHeader      = lipgloss.AdaptiveColor{Light: "#C18401", Dark: "#E5C07B"} // yellow

	branchStyle   = lipgloss.NewStyle().Foreground(colorBranch)
	openIDStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorOpenID)
	closedIDStyle = lipgloss.NewStyle().Bold(true).Foreground(colorClosedID)
	prefixStyle   = lipgloss.NewStyle().Foreground(colorTitlePrefix)
	suffixStyle   = lipgloss.NewStyle().Foreground(colorTitleSuffix)
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorHeader)
)

func renderLabel(label Label) string {
	color := "#" + label.Color
	// Use adaptive colors for the foreground text on labels
	labelTextColor := lipgloss.AdaptiveColor{Light: "#000000", Dark: "#0A0B0D"}
	return lipgloss.NewStyle().Background(lipgloss.Color(color)).Foreground(labelTextColor).Render(label.Name)
}

func renderLabels(labels []Label) string {
	if len(labels) == 0 {
		return ""
	}
	result := ""
	for _, l := range labels {
		result += " " + renderLabel(l)
	}
	return result
}

func renderSpecialLabels(issue Issue) string {
	result := ""
	if isBug(issue) {
		result += " " + renderLabel(Label{Name: "bug", Color: "E06C75"}) // One Dark red
	}
	if isBlocked(issue) {
		result += " " + renderLabel(Label{Name: "blocked", Color: "C678DD"}) // One Dark purple
	}
	return result
}

func renderNode(n *Node, prefix string, isLast bool, width int, showLink bool, maxWidth int, isRoot bool) {
	connector := "├─ "
	nextPrefix := prefix + "│  "
	if isLast {
		connector = "└─ "
		nextPrefix = prefix + "   "
	}
	if isRoot {
		connector = ""
		nextPrefix = "   "
	}
	idStyle := openIDStyle
	s := n.Issue.State
	closed := len(s) > 0 && (s[0] == 'c' || s[0] == 'C')
	if closed {
		idStyle = closedIDStyle
	}
	stateDot := "○"
	if closed {
		stateDot = "●"
	}
	full := n.Issue.Title
	colon := indexOf(full, ":")
	var rendered string
	if colon > 0 {
		pre := prefixStyle.Render(full[:colon])
		suf := suffixStyle.Render(full[colon+1:])
		rendered = pre + ":" + suf
	} else {
		rendered = suffixStyle.Render(full)
	}
	rendered += renderSpecialLabels(n.Issue)
	line := branchStyle.Render(prefix+connector) + idStyle.Render(fmt.Sprintf("%s %*d", stateDot, width, n.Issue.Number)) + " " + rendered
	if showLink && n.Issue.URL != "" {
		currentWidth := calcNodeWidth(n, prefix, width, isRoot)
		padding := maxWidth - currentWidth + 2
		if padding < 1 {
			padding = 1
		}
		for i := 0; i < padding; i++ {
			line += " "
		}
		line += branchStyle.Render(n.Issue.URL)
	}
	fmt.Println(line)
	sort.Slice(n.Children, func(i, j int) bool { return n.Children[i].Issue.Number < n.Children[j].Issue.Number })
	for i, c := range n.Children {
		renderNode(c, nextPrefix, i == len(n.Children)-1, width, showLink, maxWidth, false)
	}
}

func runIssueTree(args []string) int {
	state := "open"
	hasOpen, hasClosed, showLink := false, false, false
	var rootQuery string
	var rootNum int
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--open" || a == "-open" {
			hasOpen = true
			continue
		}
		if a == "--closed" || a == "-closed" {
			hasClosed = true
			continue
		}
		if a == "--link" || a == "-link" {
			showLink = true
			continue
		}
		if a == "--root" || a == "-root" {
			if i+1 < len(args) {
				val := args[i+1]
				if len(val) > 0 && val[0] == '#' {
					val = val[1:]
				}
				if n := atoi(val); n > 0 {
					rootNum = n
				} else {
					rootQuery = val
				}
				i++
			}
			continue
		}
	}
	if hasOpen && hasClosed {
		state = "all"
	} else if hasClosed {
		state = "closed"
	} else if hasOpen {
		state = "open"
	}
	// Get repo info
	repoCmd := exec.Command("gh", "repo", "view", "--json", "owner,name")
	repoOut, err := repoCmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ghx: failed to get repo info: %v\n", err)
		return 1
	}
	var repoInfo struct {
		Owner struct{ Login string } `json:"owner"`
		Name  string                 `json:"name"`
	}
	if err := json.Unmarshal(repoOut, &repoInfo); err != nil {
		fmt.Fprintf(os.Stderr, "ghx: failed to parse repo info: %v\n", err)
		return 1
	}
	owner := repoInfo.Owner.Login
	repo := repoInfo.Name

	// Build state filters to query
	var stateFilters []string
	if state == "all" {
		stateFilters = []string{"OPEN", "CLOSED"}
	} else if state == "closed" {
		stateFilters = []string{"CLOSED"}
	} else {
		stateFilters = []string{"OPEN"}
	}

	// GraphQL query to fetch issues with parent relationship
	query := `query($owner: String!, $repo: String!, $state: IssueState!, $cursor: String) {
		repository(owner: $owner, name: $repo) {
			issues(first: 100, after: $cursor, states: [$state]) {
				pageInfo { hasNextPage endCursor }
				nodes {
					number
					title
					body
					url
					state
					labels(first: 10) { nodes { name color } }
					parent { number }
				}
			}
		}
	}`

	var allIssues []Issue
	for _, stateFilter := range stateFilters {
		var cursor *string
		for {
			args := []string{"api", "graphql", "-f", "query=" + query, "-f", "owner=" + owner, "-f", "repo=" + repo, "-f", "state=" + stateFilter}
			if cursor != nil {
				args = append(args, "-f", "cursor="+*cursor)
			}
			cmd := exec.Command("gh", args...)
			cmd.Stdin = os.Stdin
			out, err := cmd.Output()
			if err != nil {
				fmt.Fprintf(os.Stderr, "ghx: failed to query issues: %v\n", err)
				if exitErr, ok := err.(*exec.ExitError); ok {
					return exitErr.ExitCode()
				}
				return 127
			}

			var resp struct {
				Data struct {
					Repository struct {
						Issues struct {
							PageInfo struct {
								HasNextPage bool   `json:"hasNextPage"`
								EndCursor   string `json:"endCursor"`
							} `json:"pageInfo"`
							Nodes []struct {
								Number int    `json:"number"`
								Title  string `json:"title"`
								Body   string `json:"body"`
								URL    string `json:"url"`
								State  string `json:"state"`
								Labels struct {
									Nodes []Label `json:"nodes"`
								} `json:"labels"`
								Parent *ParentRef `json:"parent"`
							} `json:"nodes"`
						} `json:"issues"`
					} `json:"repository"`
				} `json:"data"`
			}
			if err := json.Unmarshal(out, &resp); err != nil {
				fmt.Fprintf(os.Stderr, "ghx: failed to parse issues json: %v\n", err)
				return 1
			}

			for _, n := range resp.Data.Repository.Issues.Nodes {
				allIssues = append(allIssues, Issue{
					Number: n.Number,
					Title:  n.Title,
					Body:   n.Body,
					URL:    n.URL,
					State:  n.State,
					Labels: n.Labels.Nodes,
					Parent: n.Parent,
				})
			}

			if !resp.Data.Repository.Issues.PageInfo.HasNextPage {
				break
			}
			c := resp.Data.Repository.Issues.PageInfo.EndCursor
			cursor = &c
		}
	}

	nodes := make(map[int]*Node)
	for _, is := range allIssues {
		parentNum := 0
		if is.Parent != nil {
			parentNum = is.Parent.Number
		}
		if parentNum == 0 {
			parentNum = parseParent(is.Body) // fallback to body parsing
		}
		n := &Node{Issue: is, Parent: parentNum}
		nodes[is.Number] = n
	}
	var roots []*Node
	for _, n := range nodes {
		if p := n.Parent; p != 0 {
			if parentNode, ok := nodes[p]; ok {
				parentNode.Children = append(parentNode.Children, n)
			} else {
				roots = append(roots, n)
			}
		} else {
			roots = append(roots, n)
		}
	}
	var start []*Node
	if rootNum > 0 {
		if n, ok := nodes[rootNum]; ok {
			start = []*Node{n}
		}
	} else if rootQuery != "" {
		// case-insensitive substring match on title
		q := lower(rootQuery)
		for _, n := range nodes {
			if contains(lower(n.Issue.Title), q) {
				start = append(start, n)
			}
		}
	} else {
		start = roots
	}
	if len(start) == 0 {
		fmt.Println("No matching root issues.")
		return 0
	}

	// Compute global width for consistent padding
	width := numDigits(maxIssueNumberMap(nodes))

	// Render tree
	renderTree(start, width, showLink)
	return 0
}

func getIssueNodeID(num int) (string, error) {
	cmd := exec.Command("gh", "issue", "view", fmt.Sprintf("%d", num), "--json", "id", "--jq", ".id")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// Trim whitespace/newlines
	id := string(out)
	for len(id) > 0 && (id[len(id)-1] == '\n' || id[len(id)-1] == '\r' || id[len(id)-1] == ' ') {
		id = id[:len(id)-1]
	}
	return id, nil
}

func getIssueParentID(num int) (string, error) {
	query := `query($num: Int!) {
		repository(owner: "%s", name: "%s") {
			issue(number: $num) {
				parent { id }
			}
		}
	}`
	// Get repo info first
	repoCmd := exec.Command("gh", "repo", "view", "--json", "owner,name")
	repoOut, err := repoCmd.Output()
	if err != nil {
		return "", err
	}
	var repoInfo struct {
		Owner struct{ Login string } `json:"owner"`
		Name  string                 `json:"name"`
	}
	if err := json.Unmarshal(repoOut, &repoInfo); err != nil {
		return "", err
	}
	query = fmt.Sprintf(query, repoInfo.Owner.Login, repoInfo.Name)
	cmd := exec.Command("gh", "api", "graphql",
		"-H", "GraphQL-Features: sub_issues",
		"-f", "query="+query,
		"-F", fmt.Sprintf("num=%d", num))
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	var resp struct {
		Data struct {
			Repository struct {
				Issue struct {
					Parent *struct {
						ID string `json:"id"`
					} `json:"parent"`
				} `json:"issue"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", err
	}
	if resp.Data.Repository.Issue.Parent == nil {
		return "", nil
	}
	return resp.Data.Repository.Issue.Parent.ID, nil
}

func runIssueLink(args []string) int {
	var targetNum, parentNum, childNum int
	var unlink bool

	// Parse args: <issue> [--parent <num>] [--child <num>] [--unlink]
	for i := 0; i < len(args); i++ {
		a := args[i]
		if (a == "--parent" || a == "-parent") && i+1 < len(args) {
			val := args[i+1]
			if len(val) > 0 && val[0] == '#' {
				val = val[1:]
			}
			parentNum = atoi(val)
			i++
			continue
		}
		if (a == "--child" || a == "-child") && i+1 < len(args) {
			val := args[i+1]
			if len(val) > 0 && val[0] == '#' {
				val = val[1:]
			}
			childNum = atoi(val)
			i++
			continue
		}
		if a == "--unlink" || a == "-unlink" {
			unlink = true
			continue
		}
		// First positional arg is the target issue
		if targetNum == 0 {
			val := a
			if len(val) > 0 && val[0] == '#' {
				val = val[1:]
			}
			targetNum = atoi(val)
		}
	}

	if targetNum == 0 {
		fmt.Fprintln(os.Stderr, "Usage: ghx issue link <issue> --parent, -parent <parent-issue>")
		fmt.Fprintln(os.Stderr, "       ghx issue link <issue> --child, -child <child-issue>")
		fmt.Fprintln(os.Stderr, "       ghx issue link <issue> --unlink, -unlink")
		return 1
	}

	if !unlink && parentNum == 0 && childNum == 0 {
		fmt.Fprintln(os.Stderr, "Error: must specify --parent, -parent, --child, -child, or --unlink, -unlink")
		return 1
	}

	// Get node IDs
	targetID, err := getIssueNodeID(targetNum)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ghx: failed to get issue #%d: %v\n", targetNum, err)
		return 1
	}

	if unlink {
		// Get current parent ID
		parentID, err := getIssueParentID(targetNum)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ghx: failed to get parent of #%d: %v\n", targetNum, err)
			return 1
		}
		if parentID == "" {
			fmt.Printf("Issue #%d has no parent.\n", targetNum)
			return 0
		}
		// Remove sub-issue
		mutation := fmt.Sprintf(`mutation {
			removeSubIssue(input: {issueId: "%s", subIssueId: "%s"}) {
				issue { number title }
				subIssue { number title }
			}
		}`, parentID, targetID)
		cmd := exec.Command("gh", "api", "graphql",
			"-H", "GraphQL-Features: sub_issues",
			"-f", "query="+mutation)
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ghx: failed to unlink issue: %v\n", err)
			return 1
		}
		var resp struct {
			Data struct {
				RemoveSubIssue struct {
					Issue struct {
						Number int
						Title  string
					} `json:"issue"`
					SubIssue struct {
						Number int
						Title  string
					} `json:"subIssue"`
				} `json:"removeSubIssue"`
			} `json:"data"`
		}
		if err := json.Unmarshal(out, &resp); err != nil {
			fmt.Fprintf(os.Stderr, "ghx: failed to parse response: %v\n", err)
			return 1
		}
		fmt.Printf("Unlinked #%d from parent #%d\n",
			resp.Data.RemoveSubIssue.SubIssue.Number,
			resp.Data.RemoveSubIssue.Issue.Number)
		return 0
	}

	if parentNum > 0 {
		// Set target's parent
		parentID, err := getIssueNodeID(parentNum)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ghx: failed to get issue #%d: %v\n", parentNum, err)
			return 1
		}
		mutation := fmt.Sprintf(`mutation {
			addSubIssue(input: {issueId: "%s", subIssueId: "%s"}) {
				issue { number title }
				subIssue { number title }
			}
		}`, parentID, targetID)
		cmd := exec.Command("gh", "api", "graphql",
			"-H", "GraphQL-Features: sub_issues",
			"-f", "query="+mutation)
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ghx: failed to link issue: %v\n", err)
			return 1
		}
		var resp struct {
			Data struct {
				AddSubIssue struct {
					Issue struct {
						Number int
						Title  string
					} `json:"issue"`
					SubIssue struct {
						Number int
						Title  string
					} `json:"subIssue"`
				} `json:"addSubIssue"`
			} `json:"data"`
		}
		if err := json.Unmarshal(out, &resp); err != nil {
			fmt.Fprintf(os.Stderr, "ghx: failed to parse response: %v\n", err)
			return 1
		}
		fmt.Printf("Linked #%d as child of #%d\n",
			resp.Data.AddSubIssue.SubIssue.Number,
			resp.Data.AddSubIssue.Issue.Number)
		return 0
	}

	if childNum > 0 {
		// Add child to target (target becomes parent)
		childID, err := getIssueNodeID(childNum)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ghx: failed to get issue #%d: %v\n", childNum, err)
			return 1
		}
		mutation := fmt.Sprintf(`mutation {
			addSubIssue(input: {issueId: "%s", subIssueId: "%s"}) {
				issue { number title }
				subIssue { number title }
			}
		}`, targetID, childID)
		cmd := exec.Command("gh", "api", "graphql",
			"-H", "GraphQL-Features: sub_issues",
			"-f", "query="+mutation)
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ghx: failed to link issue: %v\n", err)
			return 1
		}
		var resp struct {
			Data struct {
				AddSubIssue struct {
					Issue struct {
						Number int
						Title  string
					} `json:"issue"`
					SubIssue struct {
						Number int
						Title  string
					} `json:"subIssue"`
				} `json:"addSubIssue"`
			} `json:"data"`
		}
		if err := json.Unmarshal(out, &resp); err != nil {
			fmt.Fprintf(os.Stderr, "ghx: failed to parse response: %v\n", err)
			return 1
		}
		fmt.Printf("Linked #%d as child of #%d\n",
			resp.Data.AddSubIssue.SubIssue.Number,
			resp.Data.AddSubIssue.Issue.Number)
		return 0
	}

	return 0
}

func printIssueHelp() {
	fmt.Println("Work with GHX issues.")
	fmt.Println()
	fmt.Println("USAGE")
	fmt.Println("  ghx issue <command> [flags]")
	fmt.Println("  ghx \"<title>: <body>\"        Create an issue from a string")
	fmt.Println()
	fmt.Println("GHX COMMANDS")
	fmt.Println("  link:        Set parent/child relationship between issues")
	fmt.Println("  tree:        List issues in tree format")
	fmt.Println()
	fmt.Println("EXAMPLES")
	fmt.Println("  ghx \"Bug: Fix login form validation\"")
	fmt.Println("  ghx \"Feature: Add dark mode support\"")
	fmt.Println()
	fmt.Println("-----")
	fmt.Println()
}

func createIssueFromString(issueStr string) int {
	// Split on first colon
	colonIdx := indexOf(issueStr, ":")
	if colonIdx < 0 {
		fmt.Fprintf(os.Stderr, "ghx: issue string must contain ':' separator (format: 'title: body')\n")
		return 1
	}

	title := issueStr[:colonIdx]
	body := issueStr[colonIdx+1:]

	// Trim leading/trailing whitespace from body
	for len(body) > 0 && (body[0] == ' ' || body[0] == '\t' || body[0] == '\n' || body[0] == '\r') {
		body = body[1:]
	}
	for len(body) > 0 && (body[len(body)-1] == ' ' || body[len(body)-1] == '\t' || body[len(body)-1] == '\n' || body[len(body)-1] == '\r') {
		body = body[:len(body)-1]
	}

	// Create issue using gh CLI
	args := []string{"issue", "create", "--title", title}
	if len(body) > 0 {
		args = append(args, "--body", body)
	}

	cmd := exec.Command("gh", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "ghx: failed to create issue: %v\n", err)
		return 127
	}
	return 0
}

func main() {
	args := os.Args[1:]

	// If first arg contains ":" and is not a known subcommand, treat it as issue creation
	if len(args) == 1 && indexOf(args[0], ":") >= 0 {
		// Check if it looks like a command flag or known pattern
		firstArg := args[0]
		if len(firstArg) > 0 && firstArg[0] != '-' && indexOf(firstArg, ":") < len(firstArg)-1 {
			os.Exit(createIssueFromString(firstArg))
		}
	}

	if len(args) >= 1 && args[0] == "board" {
		os.Exit(runBoard(args[1:]))
	}
	if len(args) >= 2 && args[0] == "issue" && args[1] == "tree" {
		os.Exit(runIssueTree(args[2:]))
	}
	if len(args) >= 2 && args[0] == "issue" && args[1] == "link" {
		os.Exit(runIssueLink(args[2:]))
	}
	// Show GHX help before gh help for issue command
	if len(args) >= 1 && args[0] == "issue" {
		showHelp := len(args) == 1 // no subcommand means help will be shown
		for _, a := range args[1:] {
			if a == "--help" || a == "-h" || a == "help" {
				showHelp = true
				break
			}
		}
		if showHelp {
			printIssueHelp()
		}
	}
	cmd := exec.Command("gh", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "ghx: failed to exec gh: %v\n", err)
		os.Exit(127)
	}
}

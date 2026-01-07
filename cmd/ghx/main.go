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

type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	URL    string `json:"url"`
	State  string `json:"state"`
	Labels []struct{ Name string `json:"name"` } `json:"labels"`
}

type Node struct {
	Issue    Issue
	Parent   int
	Children []*Node
}

func parseParent(body string) int {
	re := regexp.MustCompile(`(?i)^Parent:\s*#?(\d+)`)
	for _, line := range regexp.MustCompile("\r?\n").Split(body, -1) {
		m := re.FindStringSubmatch(line)
		if len(m) == 2 {
			return atoi(m[1])
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
	if len(sub) == 0 { return 0 }
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if s[i+j] != sub[j] { match = false; break }
		}
		if match { return i }
	}
	return -1
}

func isBug(is Issue) bool {
	// label named "bug" (case-insensitive) OR title starts with "Bug:" or "Bug " (case-insensitive)
	lower := func(s string) string { b := make([]byte, len(s)); for i:=0;i<len(s);i++{c:=s[i]; if c>='A'&&c<='Z'{b[i]=c+32}else{b[i]=c}}; return string(b) }
	for _, l := range is.Labels {
		if lower(l.Name) == "bug" { return true }
	}
	lt := lower(is.Title)
	return indexOf(lt, "bug:") == 0 || indexOf(lt, "bug ") == 0
}

func renderTree(roots []*Node) {
	sort.Slice(roots, func(i, j int) bool { return roots[i].Issue.Number < roots[j].Issue.Number })
	for i, r := range roots {
		renderNode(r, "", i == len(roots)-1)
	}
}

const (
	colorBranch      = "#5C6370" // One Dark comment gray
	colorOpenID      = "#98C379" // One Dark green
	colorClosedID    = "#C678DD" // One Dark purple
	colorTitlePrefix = "#61AFEF" // One Dark blue
	colorTitleSuffix = "#ABB2BF" // One Dark light gray
	colorBugDot      = "#E06C75" // One Dark red
)

var (
	branchStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(colorBranch))
	openIDStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorOpenID))
	closedIDStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorClosedID))
	prefixStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(colorTitlePrefix))
	suffixStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(colorTitleSuffix))
	bugDot         = lipgloss.NewStyle().Foreground(lipgloss.Color(colorBugDot)).Render("●")
)

func renderNode(n *Node, prefix string, isLast bool) {
	connector := "├── "
	nextPrefix := prefix + "│   "
	if isLast {
		connector = "└── "
		nextPrefix = prefix + "    "
	}
	idStyle := openIDStyle
	s := n.Issue.State
	closed := len(s) > 0 && (s[0] == 'c' || s[0] == 'C')
	if closed { idStyle = closedIDStyle }
	stateDot := "○"
	if closed { stateDot = "●" }
	bug := isBug(n.Issue)
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
	if bug { rendered = bugDot + " " + rendered }
	fmt.Println(branchStyle.Render(prefix+connector) + idStyle.Render(fmt.Sprintf("%s %d", stateDot, n.Issue.Number)) + " " + rendered)
	sort.Slice(n.Children, func(i, j int) bool { return n.Children[i].Issue.Number < n.Children[j].Issue.Number })
	for i, c := range n.Children {
		renderNode(c, nextPrefix, i == len(n.Children)-1)
	}
}

func runIssueTree(args []string) int {
	state := "open"
	hasOpen, hasClosed := false, false
	var rootQuery string
	var rootNum int
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--open" { hasOpen = true; continue }
		if a == "--closed" { hasClosed = true; continue }
		if a == "--root" {
			if i+1 < len(args) {
				val := args[i+1]
				if len(val) > 0 && val[0] == '#' { val = val[1:] }
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
	cmd := exec.Command("gh", "issue", "list", "--state", state, "--limit", "1000", "--json", "number,title,body,url,state,labels")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ghx: failed to list issues: %v\n", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 127
	}
	var issues []Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		fmt.Fprintf(os.Stderr, "ghx: failed to parse issues json: %v\n", err)
		return 1
	}
	nodes := make(map[int]*Node)
	for _, is := range issues {
		n := &Node{Issue: is, Parent: parseParent(is.Body)}
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
		lower := func(s string) string { b := make([]byte, len(s)); for i:=0;i<len(s);i++{c:=s[i]; if c>='A'&&c<='Z'{b[i]=c+32}else{b[i]=c}}; return string(b) }
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
	renderTree(start)
	return 0
}

func main() {
	args := os.Args[1:]
	if len(args) >= 2 && args[0] == "issue" && args[1] == "tree" {
		os.Exit(runIssueTree(args[2:]))
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

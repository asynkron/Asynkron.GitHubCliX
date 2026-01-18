package main

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestStripHash(t *testing.T) {
	tests := []struct {
		name  string
		color string
		want  string
	}{
		{
			name:  "Color with hash",
			color: "#FF0000",
			want:  "FF0000",
		},
		{
			name:  "Color without hash",
			color: "FF0000",
			want:  "FF0000",
		},
		{
			name:  "Empty string",
			color: "",
			want:  "",
		},
		{
			name:  "Just hash",
			color: "#",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHash(tt.color)
			if got != tt.want {
				t.Errorf("stripHash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{
			name: "String shorter than max",
			s:    "hello",
			max:  10,
			want: "hello",
		},
		{
			name: "String equal to max",
			s:    "hello",
			max:  5,
			want: "hello",
		},
		{
			name: "String longer than max",
			s:    "hello world",
			max:  8,
			want: "hello...",
		},
		{
			name: "Max is zero",
			s:    "hello",
			max:  0,
			want: "",
		},
		{
			name: "Max is 1",
			s:    "hello",
			max:  1,
			want: "h",
		},
		{
			name: "Max is 2",
			s:    "hello",
			max:  2,
			want: "he",
		},
		{
			name: "Max is 3",
			s:    "hello",
			max:  3,
			want: "hel",
		},
		{
			name: "Empty string",
			s:    "",
			max:  5,
			want: "",
		},
		{
			name: "Unicode characters",
			s:    "你好世界",
			max:  3,
			want: "你好世",
		},
		{
			name: "Unicode longer than max",
			s:    "你好世界朋友",
			max:  5,
			want: "你好...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.s, tt.max)
			if got != tt.want {
				t.Errorf("truncateString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringInSliceCaseInsensitive(t *testing.T) {
	tests := []struct {
		name string
		s    string
		list []string
		want bool
	}{
		{
			name: "Exact match",
			s:    "hello",
			list: []string{"hello", "world"},
			want: true,
		},
		{
			name: "Case insensitive match",
			s:    "HELLO",
			list: []string{"hello", "world"},
			want: true,
		},
		{
			name: "Mixed case match",
			s:    "HeLLo",
			list: []string{"hello", "world"},
			want: true,
		},
		{
			name: "Not in list",
			s:    "test",
			list: []string{"hello", "world"},
			want: false,
		},
		{
			name: "Empty list",
			s:    "hello",
			list: []string{},
			want: false,
		},
		{
			name: "Empty string in list",
			s:    "",
			list: []string{"", "hello"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringInSliceCaseInsensitive(tt.s, tt.list)
			if got != tt.want {
				t.Errorf("stringInSliceCaseInsensitive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIssueHasLabel(t *testing.T) {
	tests := []struct {
		name  string
		issue Issue
		label string
		want  bool
	}{
		{
			name: "Has label exact match",
			issue: Issue{
				Labels: []Label{
					{Name: "bug", Color: "red"},
					{Name: "feature", Color: "blue"},
				},
			},
			label: "bug",
			want:  true,
		},
		{
			name: "Has label case insensitive",
			issue: Issue{
				Labels: []Label{
					{Name: "bug", Color: "red"},
					{Name: "feature", Color: "blue"},
				},
			},
			label: "BUG",
			want:  true,
		},
		{
			name: "Does not have label",
			issue: Issue{
				Labels: []Label{
					{Name: "bug", Color: "red"},
					{Name: "feature", Color: "blue"},
				},
			},
			label: "enhancement",
			want:  false,
		},
		{
			name: "Empty labels",
			issue: Issue{
				Labels: []Label{},
			},
			label: "bug",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := issueHasLabel(tt.issue, tt.label)
			if got != tt.want {
				t.Errorf("issueHasLabel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLabelExists(t *testing.T) {
	tests := []struct {
		name   string
		labels []Label
		search string
		want   bool
	}{
		{
			name: "Label exists exact match",
			labels: []Label{
				{Name: "bug", Color: "red"},
				{Name: "feature", Color: "blue"},
			},
			search: "bug",
			want:   true,
		},
		{
			name: "Label exists case insensitive",
			labels: []Label{
				{Name: "bug", Color: "red"},
				{Name: "feature", Color: "blue"},
			},
			search: "BUG",
			want:   true,
		},
		{
			name: "Label does not exist",
			labels: []Label{
				{Name: "bug", Color: "red"},
				{Name: "feature", Color: "blue"},
			},
			search: "enhancement",
			want:   false,
		},
		{
			name:   "Empty labels",
			labels: []Label{},
			search: "bug",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := labelExists(tt.labels, tt.search)
			if got != tt.want {
				t.Errorf("labelExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyLabelChanges(t *testing.T) {
	tests := []struct {
		name     string
		labels   []Label
		add      string
		remove   []string
		addColor string
		want     int // expected number of labels after changes
	}{
		{
			name: "Add new label",
			labels: []Label{
				{Name: "bug", Color: "red"},
			},
			add:      "feature",
			remove:   []string{},
			addColor: "blue",
			want:     2,
		},
		{
			name: "Remove existing label",
			labels: []Label{
				{Name: "bug", Color: "red"},
				{Name: "feature", Color: "blue"},
			},
			add:      "",
			remove:   []string{"bug"},
			addColor: "",
			want:     1,
		},
		{
			name: "Add and remove",
			labels: []Label{
				{Name: "bug", Color: "red"},
				{Name: "feature", Color: "blue"},
			},
			add:      "enhancement",
			remove:   []string{"bug"},
			addColor: "green",
			want:     2,
		},
		{
			name: "Remove multiple labels",
			labels: []Label{
				{Name: "bug", Color: "red"},
				{Name: "feature", Color: "blue"},
				{Name: "enhancement", Color: "green"},
			},
			add:      "",
			remove:   []string{"bug", "feature"},
			addColor: "",
			want:     1,
		},
		{
			name:     "Add to empty",
			labels:   []Label{},
			add:      "bug",
			remove:   []string{},
			addColor: "red",
			want:     1,
		},
		{
			name: "Don't add duplicate",
			labels: []Label{
				{Name: "bug", Color: "red"},
			},
			add:      "bug",
			remove:   []string{},
			addColor: "red",
			want:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyLabelChanges(tt.labels, tt.add, tt.remove, tt.addColor)
			if len(got) != tt.want {
				t.Errorf("applyLabelChanges() returned %v labels, want %v", len(got), tt.want)
			}
		})
	}
}

func TestLaneForIssue(t *testing.T) {
	lanes := []boardLane{
		{Name: "TODO", Label: "TODO", Color: "blue"},
		{Name: "Doing", Label: "Doing", Color: "yellow"},
		{Name: "Done", Label: "Done", Color: "green"},
	}

	tests := []struct {
		name  string
		issue Issue
		want  int
	}{
		{
			name: "Issue in TODO",
			issue: Issue{
				Labels: []Label{{Name: "TODO", Color: "blue"}},
			},
			want: 0,
		},
		{
			name: "Issue in Doing",
			issue: Issue{
				Labels: []Label{{Name: "Doing", Color: "yellow"}},
			},
			want: 1,
		},
		{
			name: "Issue in Done",
			issue: Issue{
				Labels: []Label{{Name: "Done", Color: "green"}},
			},
			want: 2,
		},
		{
			name: "Issue with no matching label defaults to first lane",
			issue: Issue{
				Labels: []Label{{Name: "bug", Color: "red"}},
			},
			want: 0,
		},
		{
			name: "Issue with multiple labels uses first match",
			issue: Issue{
				Labels: []Label{
					{Name: "bug", Color: "red"},
					{Name: "Doing", Color: "yellow"},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := laneForIssue(tt.issue, lanes)
			if got != tt.want {
				t.Errorf("laneForIssue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindIssue(t *testing.T) {
	issues := []*Issue{
		{Number: 1, Title: "First"},
		{Number: 2, Title: "Second"},
		{Number: 3, Title: "Third"},
	}

	tests := []struct {
		name      string
		number    int
		wantIssue bool
		wantIndex int
	}{
		{
			name:      "Find first issue",
			number:    1,
			wantIssue: true,
			wantIndex: 0,
		},
		{
			name:      "Find middle issue",
			number:    2,
			wantIssue: true,
			wantIndex: 1,
		},
		{
			name:      "Find last issue",
			number:    3,
			wantIssue: true,
			wantIndex: 2,
		},
		{
			name:      "Issue not found",
			number:    999,
			wantIssue: false,
			wantIndex: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIssue, gotIndex := findIssue(issues, tt.number)
			if (gotIssue != nil) != tt.wantIssue {
				t.Errorf("findIssue() gotIssue = %v, want %v", gotIssue != nil, tt.wantIssue)
			}
			if gotIndex != tt.wantIndex {
				t.Errorf("findIssue() gotIndex = %v, want %v", gotIndex, tt.wantIndex)
			}
		})
	}
}

func TestIndexOfIssue(t *testing.T) {
	issues := []*Issue{
		{Number: 1, Title: "First"},
		{Number: 2, Title: "Second"},
		{Number: 3, Title: "Third"},
	}

	tests := []struct {
		name   string
		number int
		want   int
	}{
		{
			name:   "First issue",
			number: 1,
			want:   0,
		},
		{
			name:   "Middle issue",
			number: 2,
			want:   1,
		},
		{
			name:   "Last issue",
			number: 3,
			want:   2,
		},
		{
			name:   "Not found",
			number: 999,
			want:   -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := indexOfIssue(issues, tt.number)
			if got != tt.want {
				t.Errorf("indexOfIssue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveIssueAt(t *testing.T) {
	tests := []struct {
		name string
		idx  int
		want int // expected length after removal
	}{
		{
			name: "Remove first",
			idx:  0,
			want: 2,
		},
		{
			name: "Remove middle",
			idx:  1,
			want: 2,
		},
		{
			name: "Remove last",
			idx:  2,
			want: 2,
		},
		{
			name: "Invalid index negative",
			idx:  -1,
			want: 3,
		},
		{
			name: "Invalid index too large",
			idx:  99,
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := []*Issue{
				{Number: 1, Title: "First"},
				{Number: 2, Title: "Second"},
				{Number: 3, Title: "Third"},
			}
			got := removeIssueAt(issues, tt.idx)
			if len(got) != tt.want {
				t.Errorf("removeIssueAt() length = %v, want %v", len(got), tt.want)
			}
		})
	}
}

func TestSortLaneIssues(t *testing.T) {
	tests := []struct {
		name   string
		issues []*Issue
		want   []int // expected order of issue numbers
	}{
		{
			name: "Already sorted",
			issues: []*Issue{
				{Number: 1, Title: "First"},
				{Number: 2, Title: "Second"},
				{Number: 3, Title: "Third"},
			},
			want: []int{1, 2, 3},
		},
		{
			name: "Reverse order",
			issues: []*Issue{
				{Number: 3, Title: "Third"},
				{Number: 2, Title: "Second"},
				{Number: 1, Title: "First"},
			},
			want: []int{1, 2, 3},
		},
		{
			name: "Random order",
			issues: []*Issue{
				{Number: 2, Title: "Second"},
				{Number: 1, Title: "First"},
				{Number: 3, Title: "Third"},
			},
			want: []int{1, 2, 3},
		},
		{
			name:   "Empty list",
			issues: []*Issue{},
			want:   []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortLaneIssues(tt.issues)
			if len(tt.issues) != len(tt.want) {
				t.Errorf("sortLaneIssues() length = %v, want %v", len(tt.issues), len(tt.want))
				return
			}
			for i, wantNum := range tt.want {
				if tt.issues[i].Number != wantNum {
					t.Errorf("sortLaneIssues() issues[%d].Number = %v, want %v", i, tt.issues[i].Number, wantNum)
				}
			}
		})
	}
}

func TestRenderColumnSpacingMatchesHitTest(t *testing.T) {
	issues := []*Issue{
		{Number: 1, Title: "Issue 1"},
		{Number: 2, Title: "Issue 2"},
		{Number: 3, Title: "Issue 3"},
		{Number: 4, Title: "Issue 4"},
		{Number: 5, Title: "Issue 5"},
		{Number: 6, Title: "Issue 6"},
	}
	m := &boardModel{
		owner:       "o",
		repo:        "r",
		lanes:       []boardLane{{Name: "TODO", Label: "TODO", Color: "61AFEF", Issues: issues}},
		rowByCol:    []int{0},
		offsetByCol: []int{0},
		width:       80,
		height:      24,
	}
	m.layout = boardLayout{
		colWidth:    24,
		colGap:      0,
		boardLeft:   0,
		boardWidth:  24,
		boardTop:    0,
		boardHeight: 50,
		cardGap:     1,
	}
	m.layout.cardHeight = m.measureCardHeight()

	col := m.renderColumn(0)
	plain := ansi.Strip(col)
	lines := strings.Split(plain, "\n")
	if len(lines) != m.layout.boardHeight {
		t.Fatalf("expected %d lines, got %d", m.layout.boardHeight, len(lines))
	}

	cardBlock := m.layout.cardHeight + m.layout.cardGap
	for idx, issue := range issues {
		needle := fmt.Sprintf("#%d ", issue.Number)
		lineIndex := -1
		for i, line := range lines {
			if strings.Contains(line, needle) {
				lineIndex = i
				break
			}
		}
		if lineIndex < 0 {
			t.Fatalf("did not find issue %d in rendered column", issue.Number)
		}
		wantIndex := 1 + idx*cardBlock
		if lineIndex != wantIndex {
			t.Fatalf("issue %d expected at line %d, got %d", issue.Number, wantIndex, lineIndex)
		}

		col, row, ok := m.hitTest(0, lineIndex)
		if !ok {
			t.Fatalf("hitTest should succeed for y=%d", lineIndex)
		}
		if col != 0 {
			t.Fatalf("expected col 0 for y=%d, got %d", lineIndex, col)
		}
		if row != idx {
			t.Fatalf("expected row %d for y=%d, got %d", idx, lineIndex, row)
		}
	}

	gapY := 1 + m.layout.cardHeight
	colIdx, rowIdx, ok := m.hitTest(0, gapY)
	if !ok || colIdx != 0 || rowIdx != -1 {
		t.Fatalf("expected gap hit to return col=0 row=-1 ok=true, got col=%d row=%d ok=%v", colIdx, rowIdx, ok)
	}
}

func TestBoardBackgroundBlocksHaveDimensions(t *testing.T) {
	line := ansi.Strip(boardBackgroundLine(5))
	if len(line) != 5 {
		t.Fatalf("expected background line to be width 5, got %d", len(line))
	}
	if strings.TrimSpace(line) != "" {
		t.Fatalf("expected background line to be spaces, got %q", line)
	}

	block := ansi.Strip(boardBackgroundBlock(5, 3))
	lines := strings.Split(block, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) != 3 {
		t.Fatalf("expected background block height 3, got %d", len(lines))
	}
	for i, l := range lines {
		if len(l) != 5 {
			t.Fatalf("expected background block line %d width 5, got %d", i, len(l))
		}
		if strings.TrimSpace(l) != "" {
			t.Fatalf("expected background block line %d to be spaces, got %q", i, l)
		}
	}
}

func TestLaneAtXRespectsBoardLeftAndGap(t *testing.T) {
	m := &boardModel{
		lanes: make([]boardLane, 3),
		layout: boardLayout{
			colWidth:   10,
			colGap:     2,
			boardLeft:  3,
			boardWidth: 10*3 + 2*2,
		},
	}

	if got := m.laneAtX(0); got != -1 {
		t.Fatalf("expected x=0 to be outside board, got %d", got)
	}
	if got := m.laneAtX(2); got != -1 {
		t.Fatalf("expected x=2 to be outside board, got %d", got)
	}

	if got := m.laneAtX(3); got != 0 {
		t.Fatalf("expected x=3 to be col 0, got %d", got)
	}
	if got := m.laneAtX(12); got != 0 {
		t.Fatalf("expected x=12 to be col 0, got %d", got)
	}

	if got := m.laneAtX(13); got != -1 {
		t.Fatalf("expected x=13 to be gap, got %d", got)
	}
	if got := m.laneAtX(14); got != -1 {
		t.Fatalf("expected x=14 to be gap, got %d", got)
	}

	if got := m.laneAtX(15); got != 1 {
		t.Fatalf("expected x=15 to be col 1, got %d", got)
	}
	if got := m.laneAtX(24); got != 1 {
		t.Fatalf("expected x=24 to be col 1, got %d", got)
	}

	if got := m.laneAtX(27); got != 2 {
		t.Fatalf("expected x=27 to be col 2, got %d", got)
	}
	if got := m.laneAtX(36); got != 2 {
		t.Fatalf("expected x=36 to be col 2, got %d", got)
	}

	if got := m.laneAtX(37); got != -1 {
		t.Fatalf("expected x=37 to be outside board, got %d", got)
	}
}

func TestMoveModeDisablesVerticalNavigation(t *testing.T) {
	issues := []*Issue{
		{Number: 1, Title: "Issue 1"},
		{Number: 2, Title: "Issue 2"},
	}
	m := &boardModel{
		lanes:       []boardLane{{Name: "TODO", Label: "TODO", Color: "61AFEF", Issues: issues}},
		laneIndex:   0,
		rowByCol:    []int{0},
		offsetByCol: []int{0},
		layout: boardLayout{
			boardHeight: 12,
			cardHeight:  2,
			cardGap:     1,
		},
		moving:      true,
		movingIssue: 1,
	}

	m.updateBoardKeys(tea.KeyMsg{Type: tea.KeyDown})
	if m.rowByCol[0] != 0 {
		t.Fatalf("expected row to remain 0 in move mode, got %d", m.rowByCol[0])
	}

	m.moving = false
	m.updateBoardKeys(tea.KeyMsg{Type: tea.KeyDown})
	if m.rowByCol[0] != 1 {
		t.Fatalf("expected row to move to 1 when not moving, got %d", m.rowByCol[0])
	}
}

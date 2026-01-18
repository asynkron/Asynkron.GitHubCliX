package main

import (
	"testing"
)

func TestParseParent(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			name: "Parent with hash",
			body: "Parent: #123\nSome other content",
			want: 123,
		},
		{
			name: "Parent without hash",
			body: "Parent: 456\nSome other content",
			want: 456,
		},
		{
			name: "Parent issue with hash",
			body: "Parent issue: #789\nSome other content",
			want: 789,
		},
		{
			name: "Parent issue without hash",
			body: "Parent issue: 101\nSome other content",
			want: 101,
		},
		{
			name: "Case insensitive parent",
			body: "parent: #202\nSome other content",
			want: 202,
		},
		{
			name: "Case insensitive parent issue",
			body: "PARENT ISSUE: #303\nSome other content",
			want: 303,
		},
		{
			name: "No parent",
			body: "This is just a regular body\nwith no parent reference",
			want: 0,
		},
		{
			name: "Parent in middle of line (should not match)",
			body: "Some text Parent: #123\nMore content",
			want: 0,
		},
		{
			name: "Empty body",
			body: "",
			want: 0,
		},
		{
			name: "Parent with extra whitespace",
			body: "Parent:  #404\nContent",
			want: 404,
		},
		{
			name: "Parent issue with extra whitespace",
			body: "Parent   issue:   #505\nContent",
			want: 505,
		},
		{
			name: "Windows line endings",
			body: "Parent: #606\r\nSome other content",
			want: 606,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseParent(tt.body)
			if got != tt.want {
				t.Errorf("parseParent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAtoi(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{
			name: "Simple number",
			s:    "123",
			want: 123,
		},
		{
			name: "Zero",
			s:    "0",
			want: 0,
		},
		{
			name: "Large number",
			s:    "999999",
			want: 999999,
		},
		{
			name: "Number with non-digit suffix",
			s:    "123abc",
			want: 123,
		},
		{
			name: "Empty string",
			s:    "",
			want: 0,
		},
		{
			name: "Non-digit string",
			s:    "abc",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := atoi(tt.s)
			if got != tt.want {
				t.Errorf("atoi() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		haystack string
		needle   string
		want     bool
	}{
		{
			name:     "Exact match",
			haystack: "hello",
			needle:   "hello",
			want:     true,
		},
		{
			name:     "Substring at beginning",
			haystack: "hello world",
			needle:   "hello",
			want:     true,
		},
		{
			name:     "Substring at end",
			haystack: "hello world",
			needle:   "world",
			want:     true,
		},
		{
			name:     "Substring in middle",
			haystack: "hello world",
			needle:   "lo wo",
			want:     true,
		},
		{
			name:     "Not found",
			haystack: "hello world",
			needle:   "test",
			want:     false,
		},
		{
			name:     "Empty needle",
			haystack: "hello",
			needle:   "",
			want:     true,
		},
		{
			name:     "Empty haystack",
			haystack: "",
			needle:   "test",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.haystack, tt.needle)
			if got != tt.want {
				t.Errorf("contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLower(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "All uppercase",
			s:    "HELLO",
			want: "hello",
		},
		{
			name: "All lowercase",
			s:    "hello",
			want: "hello",
		},
		{
			name: "Mixed case",
			s:    "HeLLo WoRLd",
			want: "hello world",
		},
		{
			name: "With numbers",
			s:    "Test123",
			want: "test123",
		},
		{
			name: "Empty string",
			s:    "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lower(tt.s)
			if got != tt.want {
				t.Errorf("lower() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsBug(t *testing.T) {
	tests := []struct {
		name  string
		issue Issue
		want  bool
	}{
		{
			name: "Has bug label",
			issue: Issue{
				Title:  "Some issue",
				Labels: []Label{{Name: "bug", Color: "red"}},
			},
			want: true,
		},
		{
			name: "Has Bug label (different case)",
			issue: Issue{
				Title:  "Some issue",
				Labels: []Label{{Name: "Bug", Color: "red"}},
			},
			want: true,
		},
		{
			name: "Title starts with 'Bug:'",
			issue: Issue{
				Title:  "Bug: Fix the thing",
				Labels: []Label{},
			},
			want: true,
		},
		{
			name: "Title starts with 'bug:'",
			issue: Issue{
				Title:  "bug: fix the thing",
				Labels: []Label{},
			},
			want: true,
		},
		{
			name: "Title starts with 'Bug '",
			issue: Issue{
				Title:  "Bug in the code",
				Labels: []Label{},
			},
			want: true,
		},
		{
			name: "Not a bug",
			issue: Issue{
				Title:  "Feature request",
				Labels: []Label{{Name: "enhancement", Color: "blue"}},
			},
			want: false,
		},
		{
			name: "Bug in middle of title",
			issue: Issue{
				Title:  "Fix bug in code",
				Labels: []Label{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBug(tt.issue)
			if got != tt.want {
				t.Errorf("isBug() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsBlocked(t *testing.T) {
	tests := []struct {
		name  string
		issue Issue
		want  bool
	}{
		{
			name: "Has blocked label",
			issue: Issue{
				Title:  "Some issue",
				Labels: []Label{{Name: "blocked", Color: "red"}},
			},
			want: true,
		},
		{
			name: "Has Blocked label (different case)",
			issue: Issue{
				Title:  "Some issue",
				Labels: []Label{{Name: "Blocked", Color: "red"}},
			},
			want: true,
		},
		{
			name: "Title starts with 'Blocked:'",
			issue: Issue{
				Title:  "Blocked: Waiting for approval",
				Labels: []Label{},
			},
			want: true,
		},
		{
			name: "Title starts with 'blocked:'",
			issue: Issue{
				Title:  "blocked: waiting for approval",
				Labels: []Label{},
			},
			want: true,
		},
		{
			name: "Title starts with 'Blocked '",
			issue: Issue{
				Title:  "Blocked by other issue",
				Labels: []Label{},
			},
			want: true,
		},
		{
			name: "Not blocked",
			issue: Issue{
				Title:  "Feature request",
				Labels: []Label{{Name: "enhancement", Color: "blue"}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBlocked(tt.issue)
			if got != tt.want {
				t.Errorf("isBlocked() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNumDigits(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want int
	}{
		{
			name: "Zero",
			n:    0,
			want: 1,
		},
		{
			name: "Single digit",
			n:    5,
			want: 1,
		},
		{
			name: "Two digits",
			n:    42,
			want: 2,
		},
		{
			name: "Three digits",
			n:    123,
			want: 3,
		},
		{
			name: "Large number",
			n:    999999,
			want: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := numDigits(tt.n)
			if got != tt.want {
				t.Errorf("numDigits() = %v, want %v", got, tt.want)
			}
		})
	}
}

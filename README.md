# Asynkron.GitHubCliX
[![CI](https://github.com/asynkron/Asynkron.GitHubCliX/actions/workflows/ci.yml/badge.svg)](https://github.com/asynkron/Asynkron.GitHubCliX/actions/workflows/ci.yml)

A thin wrapper around GitHub CLI (`gh`) with extra visualizations.

Features
- Pass-through to `gh` for all commands
- `ghx issue tree` renders parent-child issue trees
- `ghx issue link` sets parent/child relationships between issues (GitHub sub-issues)
- `ghx board` opens a full-screen Kanban board for issues
- `ghx "<title>: <body> [flags]"` creates an issue from a single quoted string
- Flags: `--open`, `--closed`, `--root <title|#id|id>`, `--link`
- Bug and blocked labels rendered inline
- Styled output using charmbracelet/lipgloss with adaptive theme (automatically switches between light and dark mode based on terminal background)

Install
- From source: `go install ./cmd/ghx`

Usage
- `ghx issue tree`
- `ghx issue tree --open --closed`
- `ghx issue tree --root "Task 1"`
- `ghx issue tree --root 399`
- `ghx issue tree --link` (show issue URLs)
- `ghx issue link 123 --parent 456` (make `#123` a child of `#456`)
- `ghx issue link 456 --child 123` (make `#123` a child of `#456`)
- `ghx issue link 123 --unlink` (remove parent of `#123`)
- `ghx "Title: Body"` (create an issue; `:` is required and splits title/body)
- `ghx "Title: Body --label bug --assignee @me"` (create an issue and pass flags to `gh issue create`)
- `ghx board`

Notes
- Parent relationship from GitHub's native sub-issue feature
- Fallback: parses `Parent: #<number>` or `Parent issue: #<number>` from issue body
- `ghx issue link` accepts issue numbers as `123` or `#123` and manages GitHub sub-issues via `gh api graphql`
- Inline issue creation is only triggered when the entire issue spec is a single argument (quote it); the first `:` splits title/body, and anything after the first `--` is treated as flags and passed to `gh issue create`

Updated: 2026-01-07T17:28:23Z

## Screenshots (2026-01-07T17:28:23Z)

Command: ghx issue tree

![ghx issue tree](assets/images/ghx-issue-tree.png)

Command: ghx issue tree --open --closed --root 399

![ghx issue tree --open --closed --root 399](assets/images/ghx-issue-tree-root-399.png)

# Asynkron.GitHubCliX
[![CI](https://github.com/asynkron/Asynkron.GitHubCliX/actions/workflows/ci.yml/badge.svg)](https://github.com/asynkron/Asynkron.GitHubCliX/actions/workflows/ci.yml)

A thin wrapper around GitHub CLI (`gh`) with extra visualizations.

Features
- Pass-through to `gh` for all commands
- `ghx issue tree` renders parent-child issue trees
- `ghx board` opens a full-screen Kanban board for issues
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
- `ghx board`

Notes
- Parent relationship from GitHub's native sub-issue feature
- Fallback: parses `Parent: #<number>` or `Parent issue: #<number>` from issue body

Updated: 2026-01-07T17:28:23Z

## Screenshots (2026-01-07T17:28:23Z)

Command: ghx issue tree

![ghx issue tree](assets/images/ghx-issue-tree.png)

Command: ghx issue tree --open --closed --root 399

![ghx issue tree --open --closed --root 399](assets/images/ghx-issue-tree-root-399.png)

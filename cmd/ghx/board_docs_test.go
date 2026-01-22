package main

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	t.Cleanup(func() {
		_ = w.Close()
		_ = r.Close()
		os.Stdout = old
	})

	fn()

	_ = w.Close()
	os.Stdout = old

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("io.ReadAll: %v", err)
	}
	return string(out)
}

func readRepoFile(t *testing.T, repoRelativePath string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// This file lives in cmd/ghx; repo root is two levels up.
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))

	b, err := os.ReadFile(filepath.Join(repoRoot, repoRelativePath))
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", repoRelativePath, err)
	}
	return string(b)
}

func TestBoardHelpDocumentsFlags(t *testing.T) {
	out := captureStdout(t, func() {
		if code := runBoard([]string{"--help"}); code != 0 {
			t.Fatalf("runBoard(--help) = %d, want 0", code)
		}
	})

	for _, want := range []string{"--state", "--limit", "<open|closed|all>"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help output missing %q\n\n%s", want, out)
		}
	}
}

func TestReadmeDocumentsBoardFlags(t *testing.T) {
	readme := readRepoFile(t, "README.md")
	for _, want := range []string{"`ghx board --state closed`", "`ghx board --state all --limit 500`"} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README.md missing %q", want)
		}
	}
}


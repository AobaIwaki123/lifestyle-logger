package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunEventUsesExplicitDataDirectory(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := run([]string{
		"--data-dir", dataDir,
		"event",
		"--date", "2026-10-14",
		"--timestamp", "2026-10-14T10:00:00+09:00",
		"--category", "culture",
		"--title", "Generic event",
		"--details", "No private data in the repository.",
		"--metrics", `{"duration_minutes":40}`,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "evt-20261014-01") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
	path := filepath.Join(dataDir, "2026", "10", "2026-10-14.json")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(contents, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["date"] != "2026-10-14" {
		t.Fatalf("date = %v", decoded["date"])
	}
}

func TestRunRejectsInvalidDateAndMetrics(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"--data-dir", t.TempDir(), "event", "--date", "2026-99-99", "--title", "x", "--details", "y"},
		{"--data-dir", t.TempDir(), "event", "--title", "x", "--details", "y", "--metrics", "not-json"},
	} {
		if err := run(args, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Errorf("run(%v) unexpectedly succeeded", args)
		}
	}
}

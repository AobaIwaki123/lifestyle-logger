package logstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAddOperationsRoundTrip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store := New(root)

	path, eventID, err := store.AddEvent("2026-10-14", Event{
		Timestamp: "2026-10-14T10:00:00+09:00",
		Category:  "culture",
		Location:  "local library",
		Title:     "Morning reading",
		Details:   "Read for forty minutes.",
		Tags:      []string{"reading"},
	}, "A calm morning")
	if err != nil {
		t.Fatal(err)
	}
	if eventID != "evt-20261014-01" {
		t.Fatalf("event ID = %q, want evt-20261014-01", eventID)
	}

	_, decisionID, err := store.AddDecision("2026-10-14", Decision{
		Timestamp: "2026-10-14T11:00:00+09:00",
		Topic:     "Planning",
		Context:   "Several tasks compete for the same evening.",
		Decision:  "Assign one main theme to each evening.",
		Tags:      []string{"planning"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if decisionID != "dec-20261014-01" {
		t.Fatalf("decision ID = %q, want dec-20261014-01", decisionID)
	}

	_, topicID, err := store.AddDialogueTopic("2026-10-14", DialogueTopic{
		Topic:     "Conversation review",
		Takeaways: []string{"Separate observations from assumptions."},
		NotesSynced: []string{
			"notes/conversation-review.md",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if topicID != "dia-20261014-01" {
		t.Fatalf("topic ID = %q, want dia-20261014-01", topicID)
	}

	got, gotPath, err := store.Load("2026-10-14")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	if len(got.Events) != 1 || len(got.Decisions) != 1 || len(got.DialogueTopics) != 1 {
		t.Fatalf("unexpected counts: events=%d decisions=%d topics=%d", len(got.Events), len(got.Decisions), len(got.DialogueTopics))
	}
	if got.Summary != "A calm morning" {
		t.Fatalf("summary = %q", got.Summary)
	}

	contents, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(contents, &decoded); err != nil {
		t.Fatalf("written JSON is invalid: %v", err)
	}
}

func TestDailyPathRejectsInvalidDates(t *testing.T) {
	t.Parallel()
	store := New(t.TempDir())
	for _, date := range []string{"2026-99-99", "2026-1-01", "2026-01-01x", "../../etc/passwd"} {
		if _, err := store.DailyPath(date); err == nil {
			t.Errorf("DailyPath(%q) accepted invalid date", date)
		}
	}
}

func TestLoadRejectsMalformedExistingLog(t *testing.T) {
	t.Parallel()
	store := New(t.TempDir())
	path, err := store.DailyPath("2026-10-14")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"date":"2026-10-14","events":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Load("2026-10-14"); err == nil {
		t.Fatal("Load accepted malformed existing log")
	}
}

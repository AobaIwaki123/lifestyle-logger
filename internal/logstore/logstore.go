// Package logstore provides the private-data-independent JSON log store used by
// the lifestyle-logger command.
package logstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	dateLayout        = "2006-01-02"
	schemaReference   = "../../schema/lifestyle-log.schema.json"
	maxSummaryLineLen = 10000
)

// DailyLog is the JSON document stored for one calendar day.
type DailyLog struct {
	Schema         string          `json:"$schema"`
	Date           string          `json:"date"`
	Summary        string          `json:"summary"`
	Events         []Event         `json:"events"`
	Decisions      []Decision      `json:"decisions"`
	DialogueTopics []DialogueTopic `json:"dialogue_topics"`
}

// Event describes an external or physical activity.
type Event struct {
	ID        string         `json:"id"`
	Timestamp string         `json:"timestamp"`
	Category  string         `json:"category"`
	Location  string         `json:"location"`
	Title     string         `json:"title"`
	Details   string         `json:"details"`
	Tags      []string       `json:"tags"`
	Metrics   map[string]any `json:"metrics,omitempty"`
}

// Decision describes a decision made during the day.
type Decision struct {
	ID        string   `json:"id"`
	Timestamp string   `json:"timestamp"`
	Topic     string   `json:"topic"`
	Context   string   `json:"context"`
	Decision  string   `json:"decision"`
	Tags      []string `json:"tags"`
}

// DialogueTopic describes a conversation topic and its takeaways.
type DialogueTopic struct {
	ID          string   `json:"id"`
	Topic       string   `json:"topic"`
	Takeaways   []string `json:"takeaways"`
	NotesSynced []string `json:"notes_synced"`
}

// Store reads and writes daily log files below Root.
type Store struct {
	Root string
}

// New returns a store rooted at root.
func New(root string) Store {
	return Store{Root: filepath.Clean(root)}
}

// DailyPath returns the path for date after strictly validating the date.
func (s Store) DailyPath(date string) (string, error) {
	parsed, err := time.Parse(dateLayout, date)
	if err != nil || parsed.Format(dateLayout) != date {
		return "", fmt.Errorf("invalid date %q: expected YYYY-MM-DD", date)
	}
	if s.Root == "" || s.Root == "." {
		return "", errors.New("data directory must not be empty")
	}
	return filepath.Join(s.Root, date[:4], date[5:7], date+".json"), nil
}

// Load returns the existing daily log or a new empty log when the file is absent.
func (s Store) Load(date string) (DailyLog, string, error) {
	path, err := s.DailyPath(date)
	if err != nil {
		return DailyLog{}, "", err
	}

	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return NewDailyLog(date), path, nil
	}
	if err != nil {
		return DailyLog{}, path, fmt.Errorf("read %s: %w", path, err)
	}

	var daily DailyLog
	if err := json.Unmarshal(b, &daily); err != nil {
		return DailyLog{}, path, fmt.Errorf("parse %s: %w", path, err)
	}
	if daily.Schema == "" {
		daily.Schema = schemaReference
	}
	if err := daily.Validate(); err != nil {
		return DailyLog{}, path, fmt.Errorf("validate %s: %w", path, err)
	}
	return daily, path, nil
}

// NewDailyLog creates an empty document with the stable JSON shape.
func NewDailyLog(date string) DailyLog {
	return DailyLog{
		Schema:         schemaReference,
		Date:           date,
		Summary:        "",
		Events:         []Event{},
		Decisions:      []Decision{},
		DialogueTopics: []DialogueTopic{},
	}
}

// AddEvent appends event to date and returns the written path and ID.
func (s Store) AddEvent(date string, event Event, summary string) (string, string, error) {
	daily, path, err := s.Load(date)
	if err != nil {
		return "", "", err
	}
	event.ID = nextID("evt", date, eventIDs(daily.Events))
	daily.Events = append(daily.Events, event)
	if summary != "" {
		daily.Summary = summary
	}
	if err := s.save(path, daily); err != nil {
		return "", "", err
	}
	return path, event.ID, nil
}

// AddDecision appends decision to date and returns the written path and ID.
func (s Store) AddDecision(date string, decision Decision) (string, string, error) {
	daily, path, err := s.Load(date)
	if err != nil {
		return "", "", err
	}
	decision.ID = nextID("dec", date, decisionIDs(daily.Decisions))
	daily.Decisions = append(daily.Decisions, decision)
	if err := s.save(path, daily); err != nil {
		return "", "", err
	}
	return path, decision.ID, nil
}

// AddDialogueTopic appends topic to date and returns the written path and ID.
func (s Store) AddDialogueTopic(date string, topic DialogueTopic) (string, string, error) {
	daily, path, err := s.Load(date)
	if err != nil {
		return "", "", err
	}
	topic.ID = nextID("dia", date, topicIDs(daily.DialogueTopics))
	daily.DialogueTopics = append(daily.DialogueTopics, topic)
	if err := s.save(path, daily); err != nil {
		return "", "", err
	}
	return path, topic.ID, nil
}

// Validate checks the invariants that the CLI can enforce without third-party packages.
func (d DailyLog) Validate() error {
	parsed, err := time.Parse(dateLayout, d.Date)
	if err != nil || parsed.Format(dateLayout) != d.Date {
		return fmt.Errorf("invalid log date %q", d.Date)
	}
	if d.Schema == "" {
		return errors.New("missing $schema")
	}
	if strings.ContainsAny(d.Summary, "\r\n") || len(d.Summary) > maxSummaryLineLen {
		return errors.New("summary must be one line")
	}
	if d.Events == nil || d.Decisions == nil || d.DialogueTopics == nil {
		return errors.New("events, decisions, and dialogue_topics must be arrays")
	}

	for i, event := range d.Events {
		if err := validateEvent(event); err != nil {
			return fmt.Errorf("events[%d]: %w", i, err)
		}
	}
	for i, decision := range d.Decisions {
		if err := validateDecision(decision); err != nil {
			return fmt.Errorf("decisions[%d]: %w", i, err)
		}
	}
	for i, topic := range d.DialogueTopics {
		if err := validateTopic(topic); err != nil {
			return fmt.Errorf("dialogue_topics[%d]: %w", i, err)
		}
	}
	return nil
}

func validateEvent(event Event) error {
	if err := requireFields(map[string]string{
		"id": event.ID, "timestamp": event.Timestamp, "category": event.Category,
		"title": event.Title, "details": event.Details,
	}); err != nil {
		return err
	}
	if !validCategory(event.Category) {
		return fmt.Errorf("unsupported category %q", event.Category)
	}
	if _, err := time.Parse(time.RFC3339, event.Timestamp); err != nil {
		return fmt.Errorf("invalid timestamp %q", event.Timestamp)
	}
	return nil
}

func validateDecision(decision Decision) error {
	if err := requireFields(map[string]string{
		"id": decision.ID, "timestamp": decision.Timestamp, "topic": decision.Topic,
		"context": decision.Context, "decision": decision.Decision,
	}); err != nil {
		return err
	}
	if _, err := time.Parse(time.RFC3339, decision.Timestamp); err != nil {
		return fmt.Errorf("invalid timestamp %q", decision.Timestamp)
	}
	return nil
}

func validateTopic(topic DialogueTopic) error {
	if err := requireFields(map[string]string{"id": topic.ID, "topic": topic.Topic}); err != nil {
		return err
	}
	if len(topic.Takeaways) == 0 {
		return errors.New("takeaways must not be empty")
	}
	return nil
}

func requireFields(fields map[string]string) error {
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must not be empty", name)
		}
	}
	return nil
}

func validCategory(category string) bool {
	switch category {
	case "fieldwork", "exercise", "culture", "communication", "rest", "work", "other":
		return true
	default:
		return false
	}
}

func (s Store) save(path string, daily DailyLog) error {
	if err := daily.Validate(); err != nil {
		return fmt.Errorf("validate log before write: %w", err)
	}
	b, err := json.MarshalIndent(daily, "", "  ")
	if err != nil {
		return fmt.Errorf("encode log: %w", err)
	}
	b = append(b, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".lifestyle-log-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary log: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("protect temporary log: %w", err)
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary log: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temporary log: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary log: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func eventIDs(events []Event) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	return ids
}

func decisionIDs(decisions []Decision) []string {
	ids := make([]string, 0, len(decisions))
	for _, decision := range decisions {
		ids = append(ids, decision.ID)
	}
	return ids
}

func topicIDs(topics []DialogueTopic) []string {
	ids := make([]string, 0, len(topics))
	for _, topic := range topics {
		ids = append(ids, topic.ID)
	}
	return ids
}

func nextID(prefix, date string, existing []string) string {
	stem := prefix + "-" + strings.ReplaceAll(date, "-", "") + "-"
	max := 0
	for _, id := range existing {
		if !strings.HasPrefix(id, stem) {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(id, stem))
		if err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s%02d", stem, max+1)
}

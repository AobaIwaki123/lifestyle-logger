// Command lifestyle-logger appends generic daily activity and reflection logs.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AobaIwaki123/lifestyle-logger/internal/logstore"
)

const (
	dateLayout = "2006-01-02"
	envDataDir = "LIFESTYLE_LOG_DIR"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	defaultDir, err := defaultDataDir()
	if err != nil {
		return err
	}
	if envDir := os.Getenv(envDataDir); envDir != "" {
		defaultDir = envDir
	}

	root := flag.NewFlagSet("lifestyle-logger", flag.ContinueOnError)
	root.SetOutput(stderr)
	dataDir := root.String("data-dir", defaultDir, "root directory for daily JSON logs")
	verbose := root.Bool("verbose", false, "enable diagnostic logging")
	root.Usage = func() {
		fmt.Fprintln(stderr, "Usage: lifestyle-logger [global options] <event|decision|topic> [options]")
		root.PrintDefaults()
		fmt.Fprintln(stderr, "\nEnvironment:")
		fmt.Fprintf(stderr, "  %s  default data directory override\n", envDataDir)
	}
	if err := root.Parse(args); err != nil {
		return err
	}
	remaining := root.Args()
	if len(remaining) == 0 {
		root.Usage()
		return errors.New("a subcommand is required")
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if *verbose {
		logger = slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	resolvedDir, err := expandHome(*dataDir)
	if err != nil {
		return err
	}
	store := logstore.New(resolvedDir)

	switch remaining[0] {
	case "event":
		return runEvent(store, remaining[1:], stdout, stderr, logger)
	case "decision":
		return runDecision(store, remaining[1:], stdout, stderr, logger)
	case "topic":
		return runTopic(store, remaining[1:], stdout, stderr, logger)
	case "help":
		root.Usage()
		return nil
	default:
		root.Usage()
		return fmt.Errorf("unknown subcommand %q", remaining[0])
	}
}

func runEvent(store logstore.Store, args []string, stdout, stderr io.Writer, logger *slog.Logger) error {
	fs := flag.NewFlagSet("event", flag.ContinueOnError)
	fs.SetOutput(stderr)
	date := fs.String("date", "", "date in YYYY-MM-DD format; defaults to today")
	timestamp := fs.String("timestamp", "", "event timestamp in RFC3339 format")
	category := fs.String("category", "other", "fieldwork, exercise, culture, communication, rest, work, or other")
	location := fs.String("location", "", "physical or virtual location")
	title := fs.String("title", "", "short event title")
	details := fs.String("details", "", "event details")
	tags := fs.String("tags", "", "comma-separated tags")
	metrics := fs.String("metrics", "", "JSON object with optional metrics")
	summary := fs.String("summary", "", "one-line summary for the day")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(*title) == "" || strings.TrimSpace(*details) == "" {
		return errors.New("--title and --details are required")
	}
	if strings.ContainsAny(*summary, "\r\n") {
		return errors.New("--summary must be one line")
	}
	dateValue, timestampValue, err := resolveDateAndTimestamp(*date, *timestamp)
	if err != nil {
		return err
	}
	parsedTags := splitList(*tags, ",")
	parsedMetrics, err := parseMetrics(*metrics)
	if err != nil {
		return fmt.Errorf("--metrics: %w", err)
	}

	path, id, err := store.AddEvent(dateValue, logstore.Event{
		Timestamp: timestampValue,
		Category:  *category,
		Location:  *location,
		Title:     *title,
		Details:   *details,
		Tags:      parsedTags,
		Metrics:   parsedMetrics,
	}, *summary)
	if err != nil {
		return err
	}
	logger.Debug("event appended", "id", id, "date", dateValue, "path", path)
	fmt.Fprintf(stdout, "Added event [%s] to %s\n", id, path)
	return nil
}

func runDecision(store logstore.Store, args []string, stdout, stderr io.Writer, logger *slog.Logger) error {
	fs := flag.NewFlagSet("decision", flag.ContinueOnError)
	fs.SetOutput(stderr)
	date := fs.String("date", "", "date in YYYY-MM-DD format; defaults to today")
	timestamp := fs.String("timestamp", "", "decision timestamp in RFC3339 format")
	topic := fs.String("topic", "", "decision topic")
	context := fs.String("context", "", "background context")
	decision := fs.String("decision", "", "decision text")
	tags := fs.String("tags", "", "comma-separated tags")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(*topic) == "" || strings.TrimSpace(*context) == "" || strings.TrimSpace(*decision) == "" {
		return errors.New("--topic, --context, and --decision are required")
	}
	dateValue, timestampValue, err := resolveDateAndTimestamp(*date, *timestamp)
	if err != nil {
		return err
	}

	path, id, err := store.AddDecision(dateValue, logstore.Decision{
		Timestamp: timestampValue,
		Topic:     *topic,
		Context:   *context,
		Decision:  *decision,
		Tags:      splitList(*tags, ","),
	})
	if err != nil {
		return err
	}
	logger.Debug("decision appended", "id", id, "date", dateValue, "path", path)
	fmt.Fprintf(stdout, "Added decision [%s] to %s\n", id, path)
	return nil
}

func runTopic(store logstore.Store, args []string, stdout, stderr io.Writer, logger *slog.Logger) error {
	fs := flag.NewFlagSet("topic", flag.ContinueOnError)
	fs.SetOutput(stderr)
	date := fs.String("date", "", "date in YYYY-MM-DD format; defaults to today")
	topic := fs.String("topic", "", "dialogue topic")
	takeaways := fs.String("takeaways", "", "semicolon-separated takeaways")
	notes := fs.String("notes", "", "comma-separated note paths")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	if strings.TrimSpace(*topic) == "" {
		return errors.New("--topic is required")
	}
	parsedTakeaways := splitList(*takeaways, ";")
	if len(parsedTakeaways) == 0 {
		return errors.New("--takeaways must contain at least one item")
	}
	dateValue, _, err := resolveDateAndTimestamp(*date, "")
	if err != nil {
		return err
	}

	path, id, err := store.AddDialogueTopic(dateValue, logstore.DialogueTopic{
		Topic:       *topic,
		Takeaways:   parsedTakeaways,
		NotesSynced: splitList(*notes, ","),
	})
	if err != nil {
		return err
	}
	logger.Debug("topic appended", "id", id, "date", dateValue, "path", path)
	fmt.Fprintf(stdout, "Added topic [%s] to %s\n", id, path)
	return nil
}

func resolveDateAndTimestamp(dateValue, timestampValue string) (string, string, error) {
	now := time.Now()
	if dateValue == "" {
		dateValue = now.Format(dateLayout)
	} else {
		parsed, err := time.ParseInLocation(dateLayout, dateValue, time.Local)
		if err != nil || parsed.Format(dateLayout) != dateValue {
			return "", "", fmt.Errorf("invalid date %q: expected YYYY-MM-DD", dateValue)
		}
	}
	if timestampValue == "" {
		timestampValue = now.Format(time.RFC3339)
	} else if _, err := time.Parse(time.RFC3339, timestampValue); err != nil {
		return "", "", fmt.Errorf("invalid timestamp %q: expected RFC3339", timestampValue)
	}
	return dateValue, timestampValue, nil
}

func parseMetrics(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var metrics map[string]any
	if err := json.Unmarshal([]byte(raw), &metrics); err != nil {
		return nil, errors.New("must be a JSON object")
	}
	if metrics == nil {
		return nil, errors.New("must be a JSON object")
	}
	return metrics, nil
}

func splitList(raw, separator string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	parts := strings.Split(raw, separator)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func defaultDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "lifestyle-logs"), nil
}

func expandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find home directory: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return filepath.Clean(path), nil
}

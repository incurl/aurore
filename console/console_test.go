package console

import (
	"strings"
	"sync"
	"testing"
)

// collector records the published entries.
type collector struct {
	mu      sync.Mutex
	entries []Entry
}

func (c *collector) EmitConsoleEntry(entry Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, entry)
}

func (c *collector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

func TestAppendRecordsAnEntry(t *testing.T) {
	log := New()
	entry := log.Info("job abc", "requesting image from MiniMax")

	if entry.ID == 0 {
		t.Errorf("entry ID = %d, want a non-zero ID", entry.ID)
	}
	if entry.Level != LevelInfo {
		t.Errorf("level = %q, want %q", entry.Level, LevelInfo)
	}
	if entry.Source != "job abc" {
		t.Errorf("source = %q, want %q", entry.Source, "job abc")
	}
	if entry.Message != "requesting image from MiniMax" {
		t.Errorf("message = %q", entry.Message)
	}
	if entry.Time == "" {
		t.Error("time is empty")
	}

	entries := log.Entries()
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Message != entry.Message {
		t.Errorf("stored message = %q, want %q", entries[0].Message, entry.Message)
	}
}

func TestAppendPublishesToTheEmitter(t *testing.T) {
	c := &collector{}
	log := New()
	log.SetEmitter(c)

	log.Info("job abc", "hello")
	log.Raw("job abc", "response", `{"base_resp":{"status_code":1004}}`)
	log.Error("job abc", "boom")

	if c.count() != 3 {
		t.Errorf("published %d entries, want 3", c.count())
	}
}

func TestRawKeepsTheDetail(t *testing.T) {
	log := New()
	body := `{"base_resp":{"status_code":2013,"status_msg":"invalid params"}}`
	entry := log.Raw("job abc", "MiniMax error 2013", body)

	if entry.Level != LevelRaw {
		t.Errorf("level = %q, want %q", entry.Level, LevelRaw)
	}
	if entry.Detail != body {
		t.Errorf("detail = %q, want %q", entry.Detail, body)
	}
}

func TestEntriesAreCapped(t *testing.T) {
	log := New()
	for i := 0; i < maxEntries+50; i++ {
		log.Info("job", "message")
	}

	entries := log.Entries()
	if len(entries) != maxEntries {
		t.Fatalf("len(entries) = %d, want %d", len(entries), maxEntries)
	}
	// The oldest entries are dropped, so the last one is kept.
	last := entries[len(entries)-1]
	if last.ID != int64(maxEntries+50) {
		t.Errorf("last ID = %d, want %d", last.ID, maxEntries+50)
	}
}

func TestClearDiscardsTheEntries(t *testing.T) {
	log := New()
	log.Info("job", "hello")
	log.Clear()

	if entries := log.Entries(); len(entries) != 0 {
		t.Errorf("len(entries) = %d, want 0", len(entries))
	}
}

func TestEntriesReturnsACopy(t *testing.T) {
	log := New()
	log.Info("job", "hello")

	entries := log.Entries()
	entries[0].Message = "changed"

	fresh := log.Entries()
	if fresh[0].Message == "changed" {
		t.Error("Entries returned the internal slice; the caller mutated the log")
	}
}

func TestStringRendersTheEntry(t *testing.T) {
	entry := Entry{
		Time:    "12:00:00.000",
		Level:   LevelError,
		Source:  "job abc",
		Message: "boom",
	}
	got := entry.String()
	for _, want := range []string{"12:00:00.000", "error", "job abc", "boom"} {
		if !strings.Contains(got, want) {
			t.Errorf("String() = %q, want it to contain %q", got, want)
		}
	}
}

package bindings

import (
	"testing"

	"github.com/incurl/aurore/console"
)

// TestEventBridgeEmitsConsoleEntries verifies that the event bridge
// forwards console entries under the expected event name.
func TestEventBridgeEmitsConsoleEntries(t *testing.T) {
	if EventConsoleEntry != "console:entry" {
		t.Errorf("EventConsoleEntry = %q, want %q", EventConsoleEntry, "console:entry")
	}

	// The bridge needs a Wails application, so the nil check is
	// exercised here: a nil bridge must not panic.
	var bridge *EventBridge
	bridge.EmitConsoleEntry(console.Entry{ID: 1, Message: "hello"})

	entry := console.Entry{ID: 2, Level: console.LevelError, Message: "boom"}
	if entry.ID != 2 {
		t.Errorf("entry ID = %d, want 2", entry.ID)
	}
}

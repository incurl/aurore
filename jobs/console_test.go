package jobs

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/incurl/aurore/console"
	"github.com/incurl/aurore/minimax"
	aurorepb "github.com/incurl/aurore/pocketbase"
)

// TestJobFailureIsRecordedInTheConsole verifies that a MiniMax failure
// reaches the debug console, together with the raw response body.
func TestJobFailureIsRecordedInTheConsole(t *testing.T) {
	app := newTestApp(t)
	srv := fakeMiniMax(t, minimax.StatusInvalidParams, 0)
	defer srv.Close()

	log := console.New()
	manager := NewManager(app,
		WithLog(log),
		WithClientFactory(func(ctx context.Context) (*minimax.Client, error) {
			return minimax.NewClient("key",
				minimax.WithBaseURL(srv.URL),
				minimax.WithHTTPClient(srv.Client()),
				minimax.WithRateLimiter(minimax.NewRateLimiter(1000, time.Millisecond)),
			), nil
		}),
	)

	job, err := manager.Submit(context.Background(), Request{
		Prompt: "a red bicycle",
		Params: Params{Model: "image-01", BatchSize: 1},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForStatus(t, manager, job.ID, aurorepb.StatusFailed)

	entries := log.Entries()
	if len(entries) == 0 {
		t.Fatal("the console recorded no entries")
	}

	// The failure must be there, with the MiniMax status code.
	var failure *console.Entry
	for i := range entries {
		if entries[i].Level == console.LevelError || entries[i].Level == console.LevelRaw {
			failure = &entries[i]
			break
		}
	}
	if failure == nil {
		t.Fatalf("the console recorded no error entry: %+v", entries)
	}
	if !strings.Contains(failure.Message, "2013") {
		t.Errorf("the error message %q does not mention the status code 2013", failure.Message)
	}
	if !strings.Contains(failure.Message, "stubbed failure") {
		t.Errorf("the error message %q does not mention the API status message", failure.Message)
	}
}

// TestJobSuccessIsRecordedInTheConsole verifies that a successful request
// is logged too, so that the console shows the API traffic and not only
// the failures.
func TestJobSuccessIsRecordedInTheConsole(t *testing.T) {
	app := newTestApp(t)
	srv := fakeMiniMax(t, minimax.StatusOK, 1)
	defer srv.Close()

	log := console.New()
	manager := NewManager(app,
		WithLog(log),
		WithClientFactory(func(ctx context.Context) (*minimax.Client, error) {
			return minimax.NewClient("key",
				minimax.WithBaseURL(srv.URL),
				minimax.WithHTTPClient(srv.Client()),
				minimax.WithRateLimiter(minimax.NewRateLimiter(1000, time.Millisecond)),
			), nil
		}),
	)

	job, err := manager.Submit(context.Background(), Request{
		Prompt: "a red bicycle",
		Params: Params{Model: "image-01", BatchSize: 1},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	waitForStatus(t, manager, job.ID, aurorepb.StatusCompleted)

	entries := log.Entries()
	if len(entries) == 0 {
		t.Fatal("the console recorded no entries")
	}

	var sawRequest bool
	for _, entry := range entries {
		if strings.Contains(entry.Message, "requesting image from MiniMax") {
			sawRequest = true
		}
	}
	if !sawRequest {
		t.Errorf("the console did not record the request: %+v", entries)
	}
}

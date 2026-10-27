package bindings

import (
	"testing"

	"github.com/incurl/aurore/console"
	"github.com/incurl/aurore/jobs"
	"github.com/incurl/aurore/pocketbase"
)

// newTestService builds an AppService backed by a temporary database.
func newTestService(t *testing.T) (*AppService, *pocketbase.SettingsStore) {
	t.Helper()

	pb := pocketbase.NewService(t.TempDir())
	if err := pb.Bootstrap(); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	cipher, err := pocketbase.NewCipher(pb.DataDir())
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}

	settings := pocketbase.NewSettingsStore(pb.App(), cipher)
	manager := jobs.NewManager(pb.App())
	return NewAppService(pb, settings, manager, console.New()), settings
}

// TestSaveCredentialsNormalizesTheAPIKey verifies that a key pasted with
// stray characters is stored clean, and that a blank key leaves the
// stored one alone.
func TestSaveCredentialsNormalizesTheAPIKey(t *testing.T) {
	svc, settings := newTestService(t)

	// A key pasted with quotes, whitespace and the whole header value.
	if err := svc.SaveCredentials("  \"Bearer abc123\"\n", "", ""); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	key, found, err := settings.GetAPIKey()
	if err != nil {
		t.Fatalf("GetAPIKey: %v", err)
	}
	if !found {
		t.Fatal("the key was not stored")
	}
	if key != "abc123" {
		t.Errorf("stored key = %q, want %q", key, "abc123")
	}

	// A key that is only whitespace must not wipe the working one.
	if err := svc.SaveCredentials("   \n\t", "", ""); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	key, found, err = settings.GetAPIKey()
	if err != nil {
		t.Fatalf("GetAPIKey: %v", err)
	}
	if !found {
		t.Fatal("the key was removed by a blank save")
	}
	if key != "abc123" {
		t.Errorf("stored key = %q after a blank save, want %q", key, "abc123")
	}
}

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

// TestCapabilitiesGateTheH3Controls verifies the description the UI uses
// to decide which controls to render.
//
// It is the contract behind the capability gate: a control the model
// cannot honour must be reported as unsupported, so the UI never offers
// a setting that would be silently dropped.
func TestCapabilitiesGateTheH3Controls(t *testing.T) {
	svc, _ := newTestService(t)

	h3, err := svc.GetCapabilities("MiniMax-H3-Image")
	if err != nil {
		t.Fatalf("GetCapabilities: %v", err)
	}

	if h3.SupportsDimensions {
		t.Error("H3 does not accept a raw width and height")
	}
	if !h3.SupportsAspectRatio {
		t.Error("H3 does accept an aspect ratio")
	}
	if h3.SupportsSeed {
		// If this regresses, the UI would offer a seed that never reaches
		// the model.
		t.Error("H3 has no seed field")
	}
	if h3.SupportsWatermark {
		t.Error("H3 has no watermark field")
	}
	if h3.SupportsStyle {
		t.Error("H3 has no style presets")
	}
	if len(h3.Areas) != 3 {
		t.Errorf("areas = %v, want three levels", h3.Areas)
	}
	if h3.MaxBatchSize != 10 {
		t.Errorf("max batch = %d, want 10", h3.MaxBatchSize)
	}
	if h3.MaxPromptLength != 32000 {
		t.Errorf("max prompt = %d, want 32000", h3.MaxPromptLength)
	}

	// The Image-01 family keeps its own description.
	image01, err := svc.GetCapabilities("image-01")
	if err != nil {
		t.Fatalf("GetCapabilities: %v", err)
	}
	if !image01.SupportsDimensions {
		t.Error("image-01 does accept a width and height")
	}
	if !image01.SupportsSeed {
		t.Error("image-01 does accept a seed")
	}
	if len(image01.Areas) != 0 {
		t.Errorf("areas = %v, want none for image-01", image01.Areas)
	}
	if image01.MaxBatchSize != 9 {
		t.Errorf("max batch = %d, want 9", image01.MaxBatchSize)
	}
}

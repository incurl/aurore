package pocketbase

import "testing"

// TestUIZoomPersistsAcrossInstances verifies that the zoom survives a
// restart of the settings store.
func TestUIZoomPersistsAcrossInstances(t *testing.T) {
	svc := newTestService(t)
	cipher, err := NewCipher(svc.DataDir())
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}

	store := NewSettingsStore(svc.App(), cipher)
	if err := store.SetUI(UI{Zoom: 150, Theme: "dark"}); err != nil {
		t.Fatalf("SetUI: %v", err)
	}

	// A new store, as if the application had restarted.
	store2 := NewSettingsStore(svc.App(), cipher)
	ui, err := store2.GetUI()
	if err != nil {
		t.Fatalf("GetUI: %v", err)
	}
	if ui.Zoom != 150 {
		t.Errorf("zoom after restart = %d, want 150", ui.Zoom)
	}
}

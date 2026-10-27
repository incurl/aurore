package pocketbase

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// newTestService bootstraps an isolated PocketBase instance.
func newTestService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	svc := NewService(dir)
	if err := svc.Bootstrap(); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	return svc
}

// TestBootstrapCreatesSchema verifies that the migrations create the five
// Aurore collections with their expected fields.
func TestBootstrapCreatesSchema(t *testing.T) {
	svc := newTestService(t)

	expected := map[string][]string{
		CollectionImageJobs: {
			"prompt", "negative_prompt", "minimax_task_id", "status",
			"error_code", "error_message", "params", "seed_requested",
			"seed_effective", "image_count", "created", "updated",
		},
		CollectionGeneratedImages: {
			"job", "file", "thumbnail", "width", "height", "bytes",
			"seed", "style_tag", "prompt_snapshot", "file_name", "created",
		},
		CollectionPromptPresets: {
			"name", "prompt", "negative_prompt", "params", "is_default",
			"created", "updated",
		},
		CollectionAppSettings: {
			"key", "value", "encrypted", "updated",
		},
		CollectionUploadedAssets: {
			"file", "kind", "job", "mime", "bytes", "width", "height",
			"created",
		},
	}

	for name, fields := range expected {
		col, err := svc.Collection(name)
		if err != nil {
			t.Errorf("collection %q: %v", name, err)
			continue
		}
		for _, field := range fields {
			if col.Fields.GetByName(field) == nil {
				t.Errorf("collection %q is missing field %q", name, field)
			}
		}
	}
}

// TestBootstrapIsIdempotent verifies that re-bootstrapping an existing
// data directory does not fail or duplicate the schema.
func TestBootstrapIsIdempotent(t *testing.T) {
	dir := t.TempDir()

	for i := 0; i < 2; i++ {
		svc := NewService(dir)
		if err := svc.Bootstrap(); err != nil {
			t.Fatalf("Bootstrap #%d: %v", i, err)
		}
		col, err := svc.Collection(CollectionImageJobs)
		if err != nil {
			t.Fatalf("collection #%d: %v", i, err)
		}
		if col.Fields.GetByName("prompt") == nil {
			t.Errorf("bootstrap #%d: missing prompt field", i)
		}
	}
}

// TestSettingsAreEncrypted verifies that the API key is stored encrypted
// and never appears in plaintext in the database file.
func TestSettingsAreEncrypted(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(dir)
	if err := svc.Bootstrap(); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	cipher, err := NewCipher(svc.DataDir())
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	store := NewSettingsStore(svc.App(), cipher)

	const secret = "sk-super-secret-value"
	if err := store.SetAPIKey(secret); err != nil {
		t.Fatalf("SetAPIKey: %v", err)
	}

	// The stored value must round-trip.
	key, found, err := store.GetAPIKey()
	if err != nil {
		t.Fatalf("GetAPIKey: %v", err)
	}
	if !found {
		t.Fatal("the API key was not found")
	}
	if key != secret {
		t.Errorf("key = %q, want %q", key, secret)
	}

	// The raw record must not contain the plaintext.
	record, err := svc.App().FindFirstRecordByFilter(
		CollectionAppSettings,
		"key = {:key}",
		map[string]any{"key": SettingMiniMaxAPIKey},
	)
	if err != nil {
		t.Fatalf("find setting: %v", err)
	}
	if !record.GetBool("encrypted") {
		t.Error("the setting must be flagged as encrypted")
	}
	if raw := record.GetString("value"); raw == secret {
		t.Error("the stored value must not be the plaintext key")
	}

	// The plaintext must not appear anywhere in the database file.
	dbPath := filepath.Join(svc.DataDir(), "data.db")
	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read db: %v", err)
	}
	if string(raw) == "" {
		t.Fatal("the database file is empty")
	}
	if containsBytes(raw, []byte(secret)) {
		t.Error("the plaintext API key is present in the database file")
	}
}

// TestCipherRoundTrip verifies the AES-256-GCM envelope.
func TestCipherRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cipher, err := NewCipher(dir)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}

	envelope, err := cipher.EncryptJSON(map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("EncryptJSON: %v", err)
	}
	if envelope == "" {
		t.Fatal("the envelope is empty")
	}

	var out map[string]string
	if err := cipher.DecryptJSON(envelope, &out); err != nil {
		t.Fatalf("DecryptJSON: %v", err)
	}
	if out["k"] != "v" {
		t.Errorf("out = %v", out)
	}

	// A different key must not decrypt the envelope.
	other, err := NewCipher(t.TempDir())
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	if err := other.DecryptJSON(envelope, &out); err == nil {
		t.Error("a foreign key must not decrypt the envelope")
	}
}

// TestCipherKeyIsStable verifies that the key file is reused across
// restarts.
func TestCipherKeyIsStable(t *testing.T) {
	dir := t.TempDir()

	first, err := NewCipher(dir)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	envelope, err := first.Encrypt([]byte("payload"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	second, err := NewCipher(dir)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	plain, err := second.Decrypt(envelope)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(plain) != "payload" {
		t.Errorf("plain = %q", plain)
	}

	info, err := os.Stat(filepath.Join(dir, keyFileName))
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("key file permissions = %o, want 600", perm)
	}
}

// TestDefaultsRoundTrip verifies the non-secret settings.
func TestDefaultsRoundTrip(t *testing.T) {
	svc := newTestService(t)
	cipher, err := NewCipher(svc.DataDir())
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	store := NewSettingsStore(svc.App(), cipher)

	defaults := DefaultDefaults()
	defaults.BatchSize = 4
	defaults.Model = "image-01-live"
	if err := store.SetDefaults(defaults); err != nil {
		t.Fatalf("SetDefaults: %v", err)
	}

	got, err := store.GetDefaults()
	if err != nil {
		t.Fatalf("GetDefaults: %v", err)
	}
	if got.BatchSize != 4 || got.Model != "image-01-live" {
		t.Errorf("defaults = %+v", got)
	}

	// The base URL and model fall back to their documented defaults.
	baseURL, err := store.GetBaseURL()
	if err != nil {
		t.Fatalf("GetBaseURL: %v", err)
	}
	if baseURL != DefaultBaseURL {
		t.Errorf("base URL = %q, want %q", baseURL, DefaultBaseURL)
	}
	model, err := store.GetModel()
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if model != "image-01" {
		t.Errorf("model = %q, want image-01", model)
	}
}

// TestHandlerServesFileRoute verifies that the PocketBase handler exposes
// the native file route used to serve generated images.
func TestHandlerServesFileRoute(t *testing.T) {
	svc := newTestService(t)

	handler, err := svc.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	if handler == nil {
		t.Fatal("the handler is nil")
	}

	// The collections API must be reachable through the handler.
	req := httptest.NewRequest(http.MethodGet, "/api/collections/"+CollectionImageJobs, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == 0 {
		t.Fatal("the handler did not respond")
	}
	// A superuser-less request is rejected by the closed rules, but the
	// route must exist and return a JSON body.
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Errorf("the response is not JSON: %s", rec.Body.String())
	}
}

// containsBytes reports whether haystack contains needle.
func containsBytes(haystack, needle []byte) bool {
	return len(needle) > 0 && bytes.Contains(haystack, needle)
}

// TestUIRoundTrip verifies that the interface preferences are stored and
// read back.
func TestUIRoundTrip(t *testing.T) {
	svc := newTestService(t)
	cipher, err := NewCipher(svc.DataDir())
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	store := NewSettingsStore(svc.App(), cipher)

	// The factory defaults are returned when nothing was saved.
	ui, err := store.GetUI()
	if err != nil {
		t.Fatalf("GetUI: %v", err)
	}
	if ui.Zoom != 100 {
		t.Errorf("default zoom = %d, want 100", ui.Zoom)
	}
	if ui.Theme != "dark" {
		t.Errorf("default theme = %q, want dark", ui.Theme)
	}

	// A saved zoom is read back.
	if err := store.SetUI(UI{Zoom: 150, Theme: "dark"}); err != nil {
		t.Fatalf("SetUI: %v", err)
	}
	ui, err = store.GetUI()
	if err != nil {
		t.Fatalf("GetUI: %v", err)
	}
	if ui.Zoom != 150 {
		t.Errorf("zoom = %d, want 150", ui.Zoom)
	}
}

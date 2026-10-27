package pocketbase

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	// Importing the migrations package registers the Aurore schema
	// migrations with PocketBase's own migration list.
	_ "github.com/incurl/aurore/pocketbase/migrations"
)

// Service owns the embedded PocketBase application.
type Service struct {
	app *pocketbase.PocketBase
	// dataDir is the directory holding the SQLite database and uploaded
	// files.
	dataDir string
}

// NewService creates the PocketBase service rooted at dataDir.
//
// The database and the uploaded files live side by side in dataDir, which
// defaults to "pb_data" next to the executable.
func NewService(dataDir string) *Service {
	if dataDir == "" {
		dataDir = DefaultDataDir()
	}
	return &Service{dataDir: dataDir}
}

// DefaultDataDir returns the default location of the Aurore data
// directory.
func DefaultDataDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "pb_data")
	}
	return "pb_data"
}

// App returns the underlying PocketBase application.
func (s *Service) App() *pocketbase.PocketBase { return s.app }

// DataDir returns the directory holding the database and files.
func (s *Service) DataDir() string { return s.dataDir }

// Bootstrap initialises PocketBase and applies the Aurore migrations.
//
// PocketBase is used purely as a library: Start() is never called, so no
// HTTP server is bound and no separate process is required.
func (s *Service) Bootstrap() error {
	if err := os.MkdirAll(s.dataDir, 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	s.app = pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir: s.dataDir,
	})

	if err := s.app.Bootstrap(); err != nil {
		return fmt.Errorf("bootstrap pocketbase: %w", err)
	}

	// Apply the Go migrations that create the Aurore schema.
	if err := s.app.RunAppMigrations(); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

// Handler returns the PocketBase HTTP handler.
//
// The returned handler serves PocketBase's REST API and, crucially, its
// native file route (/api/files/<collection>/<record>/<filename>). Aurore
// mounts it on the Wails asset server so generated images are served from
// the same origin as the UI, without opening a second port.
func (s *Service) Handler() (http.Handler, error) {
	if s.app == nil {
		return nil, fmt.Errorf("pocketbase: service has not been bootstrapped")
	}
	router, err := apis.NewRouter(s.app)
	if err != nil {
		return nil, fmt.Errorf("create pocketbase router: %w", err)
	}
	return router.BuildMux()
}

// Collection returns a collection by name.
func (s *Service) Collection(name string) (*core.Collection, error) {
	if s.app == nil {
		return nil, fmt.Errorf("pocketbase: service has not been bootstrapped")
	}
	col, err := s.app.FindCollectionByNameOrId(name)
	if err != nil {
		return nil, fmt.Errorf("find collection %q: %w", name, err)
	}
	return col, nil
}

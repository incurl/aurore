// Aurore is a self-contained desktop image generation application.
//
// The UI shell is derived from the InvokeAI web frontend (Apache-2.0),
// but InvokeAI is used purely as a UI component library: no part of its
// native diffusion backend, model runtime or workflow engine is
// included. Every generation request is sent to the remote MiniMax
// Image-01 API.
//
// Persistence, file storage, migrations and access rules are provided by
// PocketBase, embedded as a Go library. PocketBase is never run as a
// standalone process: its REST and file API is mounted on the Wails
// asset server so images are served from the UI's own origin.
package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/incurl/aurore/bindings"
	"github.com/incurl/aurore/console"
	"github.com/incurl/aurore/jobs"
	"github.com/incurl/aurore/minimax"
	"github.com/incurl/aurore/pocketbase"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails embeds the built frontend. Any file in frontend/dist is served to
// the webview.
//
//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Register the job update event so the binding generator produces a
	// strongly typed TS API for it.
	application.RegisterEvent[jobs.Job]("job:updated")

	// The debug console entry event, likewise.
	application.RegisterEvent[console.Entry]("console:entry")
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run bootstraps the backend and starts the Wails application.
func run() error {
	// --- PocketBase ------------------------------------------------
	pbService := pocketbase.NewService(dataDir())
	if err := pbService.Bootstrap(); err != nil {
		return fmt.Errorf("bootstrap pocketbase: %w", err)
	}

	cipher, err := pocketbase.NewCipher(pbService.DataDir())
	if err != nil {
		return fmt.Errorf("initialise settings encryption: %w", err)
	}
	settings := pocketbase.NewSettingsStore(pbService.App(), cipher)

	// The event bridge pushes job updates to the frontend. It is created
	// once the Wails application exists, so it is declared here and
	// assigned later.
	var bridge *bindings.EventBridge

	// The debug console records the API traffic for the panel at the
	// bottom of the window.
	consoleLog := console.New()

	// The job manager builds a MiniMax client from the currently
	// configured credentials for every job.
	jobManager := jobs.NewManager(
		pbService.App(),
		jobs.WithEmitter(bridge),
		jobs.WithLog(consoleLog),
		jobs.WithClientFactory(func(ctx context.Context) (*minimax.Client, error) {
			apiKey, found, err := settings.GetAPIKey()
			if err != nil {
				return nil, err
			}
			if !found || apiKey == "" {
				return nil, fmt.Errorf("no MiniMax API key is configured. Add one in Settings.")
			}
			baseURL, err := settings.GetBaseURL()
			if err != nil {
				return nil, err
			}
			// Recorded so a rejection can be read against what was
			// actually sent. The key is never logged, only its
			// fingerprint.
			consoleLog.Info("minimax", fmt.Sprintf(
				"calling %s with a key of %s",
				baseURL, minimax.FingerprintAPIKey(apiKey)))
			return minimax.NewClient(apiKey, minimax.WithBaseURL(baseURL)), nil
		}),
	)

	appService := bindings.NewAppService(pbService, settings, jobManager, consoleLog)

	// Mount PocketBase's own REST and file API alongside the static
	// frontend. This keeps everything on a single origin: no extra
	// port, no separate PocketBase process and no CORS handling.
	pbHandler, handlerErr := pbService.Handler()
	if handlerErr != nil {
		return fmt.Errorf("create pocketbase handler: %w", handlerErr)
	}

	// --- Wails application -----------------------------------------
	app := application.New(application.Options{
		Name:        "Aurore",
		Description: "AI image generation powered by MiniMax Image-01",
		Services: []application.Service{
			application.NewService(appService),
		},
		Assets: application.AssetOptions{
			Handler: combineHandlers(pbHandler, application.AssetFileServerFS(assets)),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// The event bridge pushes job updates to the frontend.
	bridge = bindings.NewEventBridge(app)
	jobManager.SetEmitter(bridge)
	consoleLog.SetEmitter(bridge)

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Aurore",
		Width:  1440,
		Height: 900,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	return app.Run()
}

// combineHandlers routes /api/* to the PocketBase handler and everything
// else to the static frontend.
func combineHandlers(apiHandler, staticHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiHandler.ServeHTTP(w, r)
			return
		}
		staticHandler.ServeHTTP(w, r)
	})
}

// dataDir returns the directory holding the database and uploaded files.
//
// The AURORE_DATA_DIR environment variable overrides the default, which
// lets the app run against an isolated database during development and
// testing.
func dataDir() string {
	if dir := os.Getenv("AURORE_DATA_DIR"); dir != "" {
		return dir
	}
	return pocketbase.DefaultDataDir()
}

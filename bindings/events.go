package bindings

import (
	"github.com/incurl/aurore/jobs"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// EventJobUpdated is emitted whenever a job changes state. The payload is
// a jobs.Job.
const EventJobUpdated = "job:updated"

// EventBridge pushes job updates to the frontend as Wails events.
//
// The frontend also polls via React Query, so a missed event only delays
// a refresh rather than losing an update.
type EventBridge struct {
	app *application.App
}

// NewEventBridge creates an event bridge.
func NewEventBridge(app *application.App) *EventBridge {
	return &EventBridge{app: app}
}

// EmitJobUpdated implements jobs.EventEmitter.
func (b *EventBridge) EmitJobUpdated(job jobs.Job) {
	if b == nil || b.app == nil {
		return
	}
	b.app.Event.Emit(EventJobUpdated, job)
}

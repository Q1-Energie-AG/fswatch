// +build !plan9

// Package fswatch provides a platform-independent filewatcher
// which debounces events to avoid using files before they are entirly
// written to disk
package fswatch

import (
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const channelBufferSize = 10

// Watcher is a debounced filewatcher
// If a CREATE / WRITE happens it waits for {debounceDuration}
// to publish the event and resets the {debounceDuration} when
// a new WRITE event is created for the file.
type Watcher struct {
	// IgnoreTemporaryFiles indicates if files which
	// are CREATEd and DELETEd during the debounce duration are ignored
	// The default value is true.
	// If this is false, only the DELETE event is emitted right after it occurs.
	IgnoreTemporaryFiles bool

	isClosed bool
	closeMu  sync.Mutex
	closeCh  chan struct{}

	watcher          *fsnotify.Watcher
	debounceDuration time.Duration

	debounceMap   map[string]chan fsnotify.Event
	debounceMapMu sync.Mutex

	// Events is the channel on which all events are published
	Events chan fsnotify.Event

	// Errors is the channel on which all errors are published
	Errors chan error
}

// NewWatcher creates a new watcher with the specified debounceDuration.
func NewWatcher(debounceDuration time.Duration) (*Watcher, error) {
	// Create underlying fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	debouncedWatcher := &Watcher{
		watcher:          watcher,
		debounceDuration: debounceDuration,
		debounceMap:      make(map[string]chan fsnotify.Event),

		closeCh: make(chan struct{}),
		Events:  make(chan fsnotify.Event),
		Errors:  make(chan error),

		IgnoreTemporaryFiles: true,
	}

	// Start debounce loop
	go debouncedWatcher.debounceLoop()

	return debouncedWatcher, nil
}

// Add adds a new path to the watcher.
func (w *Watcher) Add(name string) error {
	return w.watcher.Add(name)
}

// Close closes the watcher.
func (w *Watcher) Close() error {
	// Close channel to quit debounce loop
	w.closeMu.Lock()
	if !w.isClosed {
		close(w.closeCh)
		w.isClosed = true
	}
	w.closeMu.Unlock()

	return w.watcher.Close()
}

// Remove removes a path from the watcher.
func (w *Watcher) Remove(name string) error {
	return w.watcher.Remove(name)
}

// debounceLoop receives all fsnotify events
// and handles them accordingly.
func (w *Watcher) debounceLoop() {
	for {
		select {
		case <-w.closeCh:
			// Watcher was closed
			return
		case event := <-w.watcher.Events:
			// A new filesystem event was received
			w.handleEvent(event)
		case err := <-w.watcher.Errors:
			// A fsnotfiy error was received
			w.Errors <- err
		}
	}
}

// handleEvent handles incoming fsnotify events.
func (w *Watcher) handleEvent(event fsnotify.Event) {
	// Handle write or create event
	if isWrite(event) || isCreate(event) {
		// Check if file is already being debounced
		w.debounceMapMu.Lock()
		ch, ok := w.debounceMap[event.Name]

		if !ok {
			// If not, add it to the debounce map
			ch := make(chan fsnotify.Event, channelBufferSize)
			w.debounceMap[event.Name] = ch

			// Start the debounce handler
			go w.debounceFile(event, ch)
		} else {
			// Publish the event to the channel of the
			// debounce handler
			ch <- event
		}

		w.debounceMapMu.Unlock()
	} else {
		w.debounceMapMu.Lock()
		ch, ok := w.debounceMap[event.Name]
		w.debounceMapMu.Unlock()
		if ok {
			// Debounce the event
			ch <- event
		} else {
			// Publish the event
			w.Events <- event
		}
	}
}

func (w *Watcher) debounceFile(event fsnotify.Event, ch chan fsnotify.Event) {
	for {
		select {
		case newEvent := <-ch:
			if newEvent.Op&fsnotify.Remove == fsnotify.Remove {
				// Remove this from the map
				w.debounceMapMu.Lock()
				delete(w.debounceMap, event.Name)
				w.debounceMapMu.Unlock()

				if !w.IgnoreTemporaryFiles {
					// if temporary files are not ignored
					// publish the delete event of the file
					w.Events <- newEvent
				}

				return
			} else if newEvent.Op&fsnotify.Rename == fsnotify.Rename {
				w.debounceMapMu.Lock()
				// Remove old debounce map entry because rename triggers a
				// new CREATE event for the renamed file
				delete(w.debounceMap, event.Name)
				w.debounceMapMu.Unlock()

				return
			}

			continue

		case <-time.After(w.debounceDuration):
			// Remove this from the map
			w.debounceMapMu.Lock()
			delete(w.debounceMap, event.Name)
			w.debounceMapMu.Unlock()
			// Emit event
			w.Events <- event

			return
		}
	}
}

func isWrite(event fsnotify.Event) bool {
	return event.Op&fsnotify.Write == fsnotify.Write
}

func isCreate(event fsnotify.Event) bool {
	return event.Op&fsnotify.Create == fsnotify.Create
}

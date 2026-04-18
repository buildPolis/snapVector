//go:build darwin

package gui

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"snapvector/capture"
)

func TestDispatchGlobalHotkeysEmitsEvent(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()

	listener := &GlobalHotkeyListener{
		actionCh: make(chan string, 4),
		stopCh:   make(chan struct{}),
		idMap: map[int]string{
			0: "capture.region",
			1: "capture.fullscreen",
			2: "capture.allDisplays",
		},
	}
	listener.actionCh <- "capture.region"
	listener.actionCh <- "capture.fullscreen"
	listener.actionCh <- "capture.allDisplays"
	close(listener.actionCh)
	app.globalHotkeyListener = listener

	// Fail loudly if the dispatcher regresses to calling the capture pipeline
	// directly: in the bug we're fixing, it captured on the Go side and
	// discarded the PNG instead of handing it to the frontend.
	app.newCapturer = func() capture.Capturer {
		t.Fatalf("global hotkey dispatch must not invoke the platform capturer directly")
		return nil
	}

	type event struct {
		name    string
		payload []any
	}
	var (
		mu       sync.Mutex
		received []event
	)
	app.emitEvent = func(_ context.Context, name string, data ...any) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, event{name: name, payload: append([]any(nil), data...)})
	}

	done := make(chan struct{})
	go func() {
		app.dispatchGlobalHotkeys()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatchGlobalHotkeys did not return after action channel closed")
	}

	mu.Lock()
	defer mu.Unlock()

	want := []event{
		{name: "snapvector:hotkey", payload: []any{"capture.region"}},
		{name: "snapvector:hotkey", payload: []any{"capture.fullscreen"}},
		{name: "snapvector:hotkey", payload: []any{"capture.allDisplays"}},
	}
	if !reflect.DeepEqual(received, want) {
		t.Fatalf("events = %+v, want %+v", received, want)
	}
}

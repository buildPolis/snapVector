//go:build linux

package gui

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// GlobalHotkeyActions lists actions eligible for system-wide hotkey registration.
// Only capture actions make sense as global shortcuts (single-key tool
// shortcuts would steal keystrokes from other applications).
var GlobalHotkeyActions = []string{
	"capture.fullscreen",
	"capture.region",
	"capture.allDisplays",
}

// GlobalHotkeyListener manages D-Bus GlobalShortcuts portal bindings.
type GlobalHotkeyListener struct {
	conn       *dbus.Conn
	sessionID  dbus.ObjectPath
	actionCh   chan string
	stopCh     chan struct{}
	closeOnce  sync.Once
}

// NewGlobalHotkeyListener creates a listener. Call Start() to begin.
func NewGlobalHotkeyListener() *GlobalHotkeyListener {
	return &GlobalHotkeyListener{
		actionCh: make(chan string, 8),
		stopCh:   make(chan struct{}),
	}
}

// Actions returns a channel that emits action names when global hotkeys fire.
func (g *GlobalHotkeyListener) Actions() <-chan string {
	return g.actionCh
}

// Start connects to the D-Bus session bus, creates a GlobalShortcuts session,
// binds the configured shortcuts, and listens for Activated signals.
// It returns immediately; signals are delivered on Actions().
func (g *GlobalHotkeyListener) Start(bindings []Hotkey) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("connect session bus: %w", err)
	}
	g.conn = conn

	// Filter to global-eligible bindings that have a combo with modifier keys.
	eligible := filterGlobalBindings(bindings)
	if len(eligible) == 0 {
		g.conn.Close()
		return fmt.Errorf("no eligible global hotkey bindings found")
	}

	sessionPath, err := g.createSession()
	if err != nil {
		g.conn.Close()
		return fmt.Errorf("create GlobalShortcuts session: %w", err)
	}
	g.sessionID = sessionPath

	if err := g.bindShortcuts(eligible); err != nil {
		g.conn.Close()
		return fmt.Errorf("bind shortcuts: %w", err)
	}

	go g.listenSignals()
	return nil
}

// Stop closes the D-Bus connection and stops the listener goroutine.
func (g *GlobalHotkeyListener) Stop() {
	g.closeOnce.Do(func() {
		close(g.stopCh)
		if g.conn != nil {
			g.conn.Close()
		}
	})
}

func (g *GlobalHotkeyListener) createSession() (dbus.ObjectPath, error) {
	token := fmt.Sprintf("snapvector_%d", time.Now().UnixNano())
	senderName := strings.Replace(g.conn.Names()[0], ".", "_", -1)
	senderName = strings.TrimPrefix(senderName, ":")
	responsePath := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + senderName + "/" + token)

	sigCh := make(chan *dbus.Signal, 1)
	g.conn.Signal(sigCh)
	matchRule := fmt.Sprintf("type='signal',interface='org.freedesktop.portal.Request',member='Response',path='%s'", responsePath)
	g.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, matchRule)
	defer g.conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, matchRule)

	obj := g.conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	opts := map[string]dbus.Variant{
		"handle_token":  dbus.MakeVariant(token),
		"session_handle_token": dbus.MakeVariant("snapvector_session"),
	}

	call := obj.Call("org.freedesktop.portal.GlobalShortcuts.CreateSession", 0, opts)
	if call.Err != nil {
		return "", call.Err
	}

	select {
	case sig := <-sigCh:
		if len(sig.Body) < 2 {
			return "", fmt.Errorf("unexpected CreateSession response")
		}
		resp, _ := sig.Body[0].(uint32)
		if resp != 0 {
			return "", fmt.Errorf("CreateSession failed with response %d", resp)
		}
		results, _ := sig.Body[1].(map[string]dbus.Variant)
		if sh, ok := results["session_handle"]; ok {
			if path, ok := sh.Value().(string); ok {
				return dbus.ObjectPath(path), nil
			}
		}
		// Use the conventional session path.
		return dbus.ObjectPath("/org/freedesktop/portal/desktop/session/" + senderName + "/snapvector_session"), nil
	case <-time.After(10 * time.Second):
		return "", fmt.Errorf("CreateSession timed out")
	}
}

func (g *GlobalHotkeyListener) bindShortcuts(bindings []Hotkey) error {
	token := fmt.Sprintf("snapvector_bind_%d", time.Now().UnixNano())
	senderName := strings.Replace(g.conn.Names()[0], ".", "_", -1)
	senderName = strings.TrimPrefix(senderName, ":")
	responsePath := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + senderName + "/" + token)

	sigCh := make(chan *dbus.Signal, 1)
	g.conn.Signal(sigCh)
	matchRule := fmt.Sprintf("type='signal',interface='org.freedesktop.portal.Request',member='Response',path='%s'", responsePath)
	g.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, matchRule)
	defer g.conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, matchRule)

	// Build the shortcuts array: each element is (id, {description, preferred_trigger})
	type shortcutEntry struct {
		ID   string
		Opts map[string]dbus.Variant
	}
	shortcuts := make([]shortcutEntry, 0, len(bindings))
	for _, b := range bindings {
		trigger := comboToPortalTrigger(b.Combo)
		shortcuts = append(shortcuts, shortcutEntry{
			ID: b.Action,
			Opts: map[string]dbus.Variant{
				"description":       dbus.MakeVariant(b.Action),
				"preferred_trigger": dbus.MakeVariant(trigger),
			},
		})
	}

	obj := g.conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	opts := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(token),
	}

	call := obj.Call("org.freedesktop.portal.GlobalShortcuts.BindShortcuts",
		0, g.sessionID, shortcuts, "", opts)
	if call.Err != nil {
		return call.Err
	}

	select {
	case sig := <-sigCh:
		if len(sig.Body) >= 1 {
			if resp, ok := sig.Body[0].(uint32); ok && resp != 0 {
				return fmt.Errorf("BindShortcuts failed with response %d", resp)
			}
		}
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("BindShortcuts timed out")
	}
}

func (g *GlobalHotkeyListener) listenSignals() {
	sigCh := make(chan *dbus.Signal, 16)
	g.conn.Signal(sigCh)
	g.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0,
		"type='signal',interface='org.freedesktop.portal.GlobalShortcuts',member='Activated'")

	for {
		select {
		case sig := <-sigCh:
			if sig == nil {
				return
			}
			if sig.Name != "org.freedesktop.portal.GlobalShortcuts.Activated" {
				continue
			}
			if len(sig.Body) < 2 {
				continue
			}
			shortcutID, ok := sig.Body[1].(string)
			if !ok {
				continue
			}
			log.Printf("snapvector global hotkey activated: %s", shortcutID)
			select {
			case g.actionCh <- shortcutID:
			default:
				// Channel full — drop to avoid blocking.
			}
		case <-g.stopCh:
			return
		}
	}
}

// filterGlobalBindings returns only bindings whose action is in GlobalHotkeyActions
// and whose combo includes at least one modifier key (safety: never grab bare keys globally).
func filterGlobalBindings(bindings []Hotkey) []Hotkey {
	eligible := map[string]bool{}
	for _, a := range GlobalHotkeyActions {
		eligible[a] = true
	}
	var out []Hotkey
	for _, b := range bindings {
		if !eligible[b.Action] {
			continue
		}
		if b.Combo == "" {
			continue
		}
		if !hasModifier(b.Combo) {
			log.Printf("snapvector: skipping global hotkey %q for %s (no modifier key — unsafe for global)", b.Combo, b.Action)
			continue
		}
		out = append(out, b)
	}
	return out
}

// hasModifier checks that a combo string contains at least one modifier key.
func hasModifier(combo string) bool {
	parts := strings.Split(combo, "+")
	for _, p := range parts {
		if isModifier(p) {
			return true
		}
	}
	return false
}

// comboToPortalTrigger converts our internal combo format (mod+shift+q)
// to the XDG GlobalShortcuts preferred_trigger format (e.g. "<Control><Shift>q").
func comboToPortalTrigger(combo string) string {
	parts := strings.Split(combo, "+")
	var b strings.Builder
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "mod", "ctrl":
			b.WriteString("<Control>")
		case "alt":
			b.WriteString("<Alt>")
		case "shift":
			b.WriteString("<Shift>")
		}
	}
	main := parts[len(parts)-1]
	b.WriteString(main)
	return b.String()
}

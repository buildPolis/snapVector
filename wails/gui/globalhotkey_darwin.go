//go:build darwin

// macOS global hotkey listener via Carbon RegisterEventHotKey.
//
// Carbon's hotkey API requires no TCC permission (unlike NSEvent's global
// monitor which needs Accessibility). The API is deprecated on paper but
// Apple's own apps still use it and the framework ships in every macOS release.

package gui

/*
#cgo LDFLAGS: -framework Carbon
#cgo CFLAGS: -Wno-deprecated-declarations

#include <Carbon/Carbon.h>
#include <stdint.h>

#define SV_MAX_HOTKEYS 16

static EventHotKeyRef sv_hotkey_refs[SV_MAX_HOTKEYS];
static EventHandlerRef sv_handler_ref = NULL;

// Ring buffer for fired hotkey IDs.
static volatile int32_t sv_fired_ids[64];
static volatile int32_t sv_fired_write = 0;
static volatile int32_t sv_fired_read = 0;

static OSStatus sv_hotkey_handler(EventHandlerCallRef next, EventRef event, void* userData) {
    (void)next; (void)userData;
    EventHotKeyID hkID;
    if (GetEventParameter(event, kEventParamDirectObject, typeEventHotKeyID,
                          NULL, sizeof(hkID), NULL, &hkID) != noErr) {
        return noErr;
    }
    int32_t w = sv_fired_write;
    int32_t next_w = (w + 1) % 64;
    if (next_w != sv_fired_read) {
        sv_fired_ids[w] = (int32_t)hkID.id;
        sv_fired_write = next_w;
    }
    return noErr;
}

static int sv_install_handler(void) {
    if (sv_handler_ref != NULL) return 0;
    EventTypeSpec spec = { kEventClassKeyboard, kEventHotKeyPressed };
    OSStatus st = InstallEventHandler(
        GetEventDispatcherTarget(), sv_hotkey_handler, 1, &spec, NULL, &sv_handler_ref);
    return st == noErr ? 0 : -1;
}

static int sv_register_hotkey(int id, uint32_t keyCode, uint32_t modifiers) {
    if (id < 0 || id >= SV_MAX_HOTKEYS) return -1;
    if (sv_handler_ref == NULL && sv_install_handler() != 0) return -2;
    EventHotKeyID hkID = { 'SVec', (UInt32)id };
    OSStatus st = RegisterEventHotKey(keyCode, modifiers, hkID,
        GetEventDispatcherTarget(), 0, &sv_hotkey_refs[id]);
    return st == noErr ? 0 : -1;
}

static void sv_unregister_hotkey(int id) {
    if (id < 0 || id >= SV_MAX_HOTKEYS) return;
    if (sv_hotkey_refs[id] != NULL) {
        UnregisterEventHotKey(sv_hotkey_refs[id]);
        sv_hotkey_refs[id] = NULL;
    }
}

static void sv_unregister_all(void) {
    for (int i = 0; i < SV_MAX_HOTKEYS; i++) sv_unregister_hotkey(i);
}

// Returns fired hotkey ID or -1 if none.
static int32_t sv_poll_hotkey(void) {
    if (sv_fired_read == sv_fired_write) return -1;
    int32_t id = sv_fired_ids[sv_fired_read];
    sv_fired_read = (sv_fired_read + 1) % 64;
    return id;
}
*/
import "C"

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// macOS Carbon modifier flags.
const (
	carbonCmdKey   = 0x0100
	carbonShiftKey = 0x0200
	carbonOptKey   = 0x0800
	carbonCtrlKey  = 0x1000
)

var macKeyCodeTable = map[string]uint32{
	"a": 0, "s": 1, "d": 2, "f": 3, "h": 4, "g": 5, "z": 6, "x": 7,
	"c": 8, "v": 9, "b": 11, "q": 12, "w": 13, "e": 14, "r": 15,
	"y": 16, "t": 17, "1": 18, "2": 19, "3": 20, "4": 21, "6": 22,
	"5": 23, "=": 24, "9": 25, "7": 26, "-": 27, "8": 28, "0": 29,
	"]": 30, "o": 31, "u": 32, "[": 33, "i": 34, "p": 35, "l": 37,
	"j": 38, "'": 39, "k": 40, ";": 41, "\\": 42, ",": 43, "/": 44,
	"n": 45, "m": 46, ".": 47, "`": 50, " ": 49,
	"f1": 122, "f2": 120, "f3": 99, "f4": 118, "f5": 96, "f6": 97,
	"f7": 98, "f8": 100, "f9": 101, "f10": 109, "f11": 103, "f12": 111,
}

type GlobalHotkeyListener struct {
	actionCh  chan string
	stopCh    chan struct{}
	closeOnce sync.Once
	idMap     map[int]string
}

func newDarwinGlobalHotkeyListener(bindings []Hotkey) (*GlobalHotkeyListener, error) {
	eligible := filterGlobalBindings(bindings)
	l := &GlobalHotkeyListener{
		actionCh: make(chan string, 8),
		stopCh:   make(chan struct{}),
		idMap:    make(map[int]string),
	}
	for i, b := range eligible {
		if i >= 16 {
			break
		}
		mods, mainKey := parseCombo(b.Combo)
		kc, ok := macKeyCodeTable[mainKey]
		if !ok {
			log.Printf("snapvector: unknown macOS keycode for %q in combo %q, skipping", mainKey, b.Combo)
			continue
		}
		var cm C.uint32_t
		for _, m := range mods {
			switch m {
			case "mod":
				cm |= carbonCmdKey
			case "ctrl":
				cm |= carbonCtrlKey
			case "alt":
				cm |= carbonOptKey
			case "shift":
				cm |= carbonShiftKey
			}
		}
		rc := C.sv_register_hotkey(C.int(i), C.uint32_t(kc), cm)
		if rc != 0 {
			log.Printf("snapvector: failed to register global hotkey %q for %s (rc=%d)", b.Combo, b.Action, rc)
			continue
		}
		l.idMap[i] = b.Action
		log.Printf("snapvector: registered global hotkey %q → %s (id=%d)", b.Combo, b.Action, i)
	}
	if len(l.idMap) == 0 {
		return nil, fmt.Errorf("no global hotkeys registered")
	}
	return l, nil
}

func (l *GlobalHotkeyListener) Start() {
	go l.pollLoop()
}

func (l *GlobalHotkeyListener) Actions() <-chan string {
	return l.actionCh
}

func (l *GlobalHotkeyListener) Stop() {
	l.closeOnce.Do(func() {
		close(l.stopCh)
		C.sv_unregister_all()
	})
}

func (l *GlobalHotkeyListener) pollLoop() {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			for {
				id := C.sv_poll_hotkey()
				if id < 0 {
					break
				}
				action, ok := l.idMap[int(id)]
				if !ok {
					continue
				}
				select {
				case l.actionCh <- action:
				default:
				}
			}
		}
	}
}

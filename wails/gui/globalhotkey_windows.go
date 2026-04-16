//go:build windows

package gui

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	modAlt      = 0x0001
	modControl  = 0x0002
	modShift    = 0x0004
	modNoRepeat = 0x4000
	wmHotkey    = 0x0312
	wmQuit      = 0x0012
)

var (
	user32            = windows.NewLazyDLL("user32.dll")
	pRegisterHotKey   = user32.NewProc("RegisterHotKey")
	pUnregisterHotKey = user32.NewProc("UnregisterHotKey")
	pGetMessage       = user32.NewProc("GetMessageW")
	pPostThreadMsg    = user32.NewProc("PostThreadMessageW")
)

var winVKTable = map[string]uint32{
	"a": 0x41, "b": 0x42, "c": 0x43, "d": 0x44, "e": 0x45, "f": 0x46,
	"g": 0x47, "h": 0x48, "i": 0x49, "j": 0x4A, "k": 0x4B, "l": 0x4C,
	"m": 0x4D, "n": 0x4E, "o": 0x4F, "p": 0x50, "q": 0x51, "r": 0x52,
	"s": 0x53, "t": 0x54, "u": 0x55, "v": 0x56, "w": 0x57, "x": 0x58,
	"y": 0x59, "z": 0x5A,
	"0": 0x30, "1": 0x31, "2": 0x32, "3": 0x33, "4": 0x34,
	"5": 0x35, "6": 0x36, "7": 0x37, "8": 0x38, "9": 0x39,
	"f1": 0x70, "f2": 0x71, "f3": 0x72, "f4": 0x73, "f5": 0x74, "f6": 0x75,
	"f7": 0x76, "f8": 0x77, "f9": 0x78, "f10": 0x79, "f11": 0x7A, "f12": 0x7B,
	"-": 0xBD, "=": 0xBB, "[": 0xDB, "]": 0xDD, "\\": 0xDC,
	";": 0xBA, "'": 0xDE, ",": 0xBC, ".": 0xBE, "/": 0xBF, "`": 0xC0,
	" ": 0x20,
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

type GlobalHotkeyListener struct {
	actionCh  chan string
	stopCh    chan struct{}
	closeOnce sync.Once
	idMap     map[int]string
	threadID  uint32
	count     int
}

func newWindowsGlobalHotkeyListener(bindings []Hotkey) (*GlobalHotkeyListener, error) {
	eligible := filterGlobalBindings(bindings)
	l := &GlobalHotkeyListener{
		actionCh: make(chan string, 8),
		stopCh:   make(chan struct{}),
		idMap:    make(map[int]string),
	}
	for i, b := range eligible {
		mods, mainKey := parseCombo(b.Combo)
		vk, ok := winVKTable[mainKey]
		if !ok {
			log.Printf("snapvector: unknown Windows VK for %q in combo %q, skipping", mainKey, b.Combo)
			continue
		}
		var wm uint32 = modNoRepeat
		for _, m := range mods {
			switch m {
			case "mod", "ctrl":
				wm |= modControl
			case "alt":
				wm |= modAlt
			case "shift":
				wm |= modShift
			}
		}
		l.idMap[i] = b.Action
		l.count++
		_ = vk
		_ = wm
	}
	if l.count == 0 {
		return nil, fmt.Errorf("no global hotkeys to register")
	}

	eligible2 := filterGlobalBindings(bindings)
	l.startLoop(eligible2)
	return l, nil
}

func (l *GlobalHotkeyListener) startLoop(bindings []Hotkey) {
	ready := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		l.threadID = windows.GetCurrentThreadId()

		for i, b := range bindings {
			mods, mainKey := parseCombo(b.Combo)
			vk, ok := winVKTable[mainKey]
			if !ok {
				continue
			}
			var wm uintptr = modNoRepeat
			for _, m := range mods {
				switch m {
				case "mod", "ctrl":
					wm |= modControl
				case "alt":
					wm |= modAlt
				case "shift":
					wm |= modShift
				}
			}
			r, _, err := pRegisterHotKey.Call(0, uintptr(i), wm, uintptr(vk))
			if r == 0 {
				log.Printf("snapvector: RegisterHotKey failed for %q (%s): %v", b.Combo, b.Action, err)
			} else {
				log.Printf("snapvector: registered global hotkey %q → %s (id=%d)", b.Combo, b.Action, i)
			}
		}

		close(ready)

		var m msg
		for {
			r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if r == 0 || int32(r) == -1 {
				break
			}
			if m.message == wmHotkey {
				id := int(m.wParam)
				if action, ok := l.idMap[id]; ok {
					select {
					case l.actionCh <- action:
					default:
					}
				}
			}
		}

		for i := 0; i < len(bindings); i++ {
			pUnregisterHotKey.Call(0, uintptr(i))
		}
	}()
	<-ready
}

func (l *GlobalHotkeyListener) Start() {}

func (l *GlobalHotkeyListener) Actions() <-chan string {
	return l.actionCh
}

func (l *GlobalHotkeyListener) Stop() {
	l.closeOnce.Do(func() {
		close(l.stopCh)
		if l.threadID != 0 {
			pPostThreadMsg.Call(uintptr(l.threadID), wmQuit, 0, 0)
		}
	})
}

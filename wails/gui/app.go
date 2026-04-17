package gui

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"snapvector/annotation"
	"snapvector/capture"
	"snapvector/clipboarddoc"
	"snapvector/exportdoc"
	"snapvector/svgdoc"
)

type App struct {
	ctx                  context.Context
	newCapturer          func() capture.Capturer
	convertExporter      func(context.Context, string, string) ([]byte, string, error)
	writeClipboard       func(context.Context, []byte, string) error
	openFileDialog       func(context.Context, wailsruntime.OpenDialogOptions) (string, error)
	openDirectoryDialog  func(context.Context, wailsruntime.OpenDialogOptions) (string, error)
	saveFileDialog       func(context.Context, wailsruntime.SaveDialogOptions) (string, error)
	readFile             func(string) ([]byte, error)
	writeFile            func(string, []byte, os.FileMode) error
	hideWindow           func(context.Context)
	showWindow           func(context.Context)
	preCaptureDelay      time.Duration
	postCaptureHold      time.Duration
	preferencesStore     *PreferencesStore
	hotkeyStore          *HotkeyStore
	globalHotkeyListener globalHotkeyListenerHandle
}

type CaptureResult struct {
	Format        string         `json:"format"`
	MimeType      string         `json:"mimeType"`
	Base64        string         `json:"base64"`
	Display       map[string]any `json:"display,omitempty"`
	CaptureRegion map[string]any `json:"captureRegion,omitempty"`
}

type ExportResult struct {
	Format            string         `json:"format"`
	MimeType          string         `json:"mimeType"`
	Base64            string         `json:"base64,omitempty"`
	SVG               string         `json:"svg,omitempty"`
	AnnotationCount   int            `json:"annotationCount"`
	Canvas            map[string]any `json:"canvas"`
	CaptureRegion     map[string]any `json:"captureRegion,omitempty"`
	CopiedToClipboard bool           `json:"copiedToClipboard,omitempty"`
}

type DocumentOpenResult struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Contents string `json:"contents"`
}

type DocumentSaveResult struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

func NewApp() *App {
	return &App{
		newCapturer:         capture.NewPlatformCapturer,
		convertExporter:     exportdoc.Convert,
		writeClipboard:      clipboarddoc.Write,
		openFileDialog:      wailsruntime.OpenFileDialog,
		openDirectoryDialog: wailsruntime.OpenDirectoryDialog,
		saveFileDialog:      wailsruntime.SaveFileDialog,
		readFile:            os.ReadFile,
		writeFile:           os.WriteFile,
		hideWindow:          wailsruntime.WindowHide,
		showWindow:          wailsruntime.WindowShow,
		preCaptureDelay:     250 * time.Millisecond,
		postCaptureHold:     120 * time.Millisecond,
		preferencesStore:    NewPreferencesStore(),
		hotkeyStore:         NewHotkeyStore(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	wailsruntime.WindowCenter(ctx)
	a.startGlobalHotkeys()
}

func (a *App) shutdown(context.Context) {
	a.stopGlobalHotkeys()
}

func (a *App) CaptureScreen() (*CaptureResult, error) {
	return a.captureWith(a.newCapturer().CaptureFullScreen)
}

func (a *App) CaptureRegion() (*CaptureResult, error) {
	return a.captureWith(a.newCapturer().CaptureInteractiveRegion)
}

func (a *App) CaptureAllDisplays() (*CaptureResult, error) {
	return a.captureWith(a.newCapturer().CaptureAllDisplays)
}

// captureWith drops the Wails window out of the way so the OS capture UI
// (e.g. screencapture -i -s crosshair) can own focus and so the app itself
// isn't photographed, then runs the actual capture.
func (a *App) captureWith(run func(context.Context) (capture.PNG, capture.Meta, error)) (*CaptureResult, error) {
	ctx, cancel := a.captureContext()
	defer cancel()

	if a.ctx != nil && a.hideWindow != nil {
		a.hideWindow(a.ctx)
		a.waitOrCancel(ctx, a.preCaptureDelay)
		if a.showWindow != nil {
			defer func() {
				a.waitOrCancel(context.Background(), a.postCaptureHold)
				a.showWindow(a.ctx)
			}()
		}
	}

	raw, meta, err := run(ctx)
	if err != nil {
		return nil, err
	}

	result := &CaptureResult{
		Format:   "png",
		MimeType: "image/png",
		Base64:   base64.StdEncoding.EncodeToString(raw),
	}
	if display := displayData(meta); len(display) > 0 {
		result.Display = display
	}
	if region := captureRegionData(meta); len(region) > 0 {
		result.CaptureRegion = region
	}

	return result, nil
}

func (a *App) ExportDocument(payload string, captureBase64 string, width int, height int, format string, copyToClipboard bool) (*ExportResult, error) {
	annotations, err := annotation.ParsePayload(payload)
	if err != nil {
		return nil, err
	}

	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "svg"
	}

	raw, err := base64.StdEncoding.DecodeString(captureBase64)
	if err != nil {
		return nil, fmt.Errorf("decode capture base64: %w", err)
	}

	svg, err := svgdoc.Compose(raw, width, height, annotations)
	if err != nil {
		return nil, err
	}

	result := &ExportResult{
		Format:          format,
		AnnotationCount: len(annotations),
		Canvas: map[string]any{
			"width":  width,
			"height": height,
		},
		CaptureRegion: map[string]any{
			"x":      0,
			"y":      0,
			"width":  width,
			"height": height,
		},
	}

	ctx, cancel := a.captureContext()
	defer cancel()

	if format == "svg" {
		result.MimeType = "image/svg+xml"
		result.SVG = svg
		if copyToClipboard {
			if err := a.writeClipboard(ctx, []byte(svg), "svg"); err != nil {
				return nil, err
			}
			result.CopiedToClipboard = true
		}
		return result, nil
	}

	converted, mimeType, err := a.convertExporter(ctx, svg, format)
	if err != nil {
		return nil, err
	}

	result.MimeType = mimeType
	result.Base64 = base64.StdEncoding.EncodeToString(converted)
	if copyToClipboard {
		if err := a.writeClipboard(ctx, converted, format); err != nil {
			return nil, err
		}
		result.CopiedToClipboard = true
	}

	return result, nil
}

func (a *App) ExportDocumentToFile(payload string, captureBase64 string, width int, height int, format string, suggestedName string) (*DocumentSaveResult, error) {
	result, err := a.ExportDocument(payload, captureBase64, width, height, format, false)
	if err != nil {
		return nil, err
	}

	preferences, err := a.preferencesStore.Load()
	if err != nil {
		return nil, err
	}
	exportDir := strings.TrimSpace(preferences.ExportDirectory)
	if exportDir == "" {
		return nil, fmt.Errorf("export folder is unavailable")
	}
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		return nil, fmt.Errorf("create export folder: %w", err)
	}
	info, err := os.Stat(exportDir)
	if err != nil {
		return nil, fmt.Errorf("stat export folder: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("export folder is not a directory: %s", exportDir)
	}

	filename := defaultExportFilename(strings.TrimSpace(suggestedName), result.Format)
	path, err := nextAvailableExportPath(exportDir, filename)
	if err != nil {
		return nil, err
	}

	var raw []byte
	if result.Format == "svg" {
		raw = []byte(result.SVG)
	} else {
		raw, err = base64.StdEncoding.DecodeString(result.Base64)
		if err != nil {
			return nil, fmt.Errorf("decode exported %s base64: %w", result.Format, err)
		}
	}

	if err := a.writeFile(path, raw, 0o644); err != nil {
		return nil, fmt.Errorf("write export: %w", err)
	}

	return &DocumentSaveResult{Path: path, Name: filepath.Base(path)}, nil
}

func (a *App) OpenDocument() (*DocumentOpenResult, error) {
	ctx, cancel := a.captureContext()
	defer cancel()

	path, err := a.openFileDialog(ctx, wailsruntime.OpenDialogOptions{
		Title:           "Open SnapVector document",
		DefaultFilename: "untitled.sv.json",
		Filters:         fileDialogFilters(),
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}

	raw, err := a.readFile(path)
	if err != nil {
		return nil, fmt.Errorf("read document: %w", err)
	}

	return &DocumentOpenResult{
		Path:     path,
		Name:     filepath.Base(path),
		Contents: string(raw),
	}, nil
}

func (a *App) SaveDocument(path string, contents string) (*DocumentSaveResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("save document requires a target path")
	}
	if err := a.writeDocument(path, contents); err != nil {
		return nil, err
	}
	return &DocumentSaveResult{Path: path, Name: filepath.Base(path)}, nil
}

func (a *App) SaveDocumentAs(suggestedName string, contents string) (*DocumentSaveResult, error) {
	ctx, cancel := a.captureContext()
	defer cancel()

	suggestedName = ensureDocumentExtension(strings.TrimSpace(suggestedName))
	if suggestedName == "" {
		suggestedName = "untitled.sv.json"
	}

	path, err := a.saveFileDialog(ctx, wailsruntime.SaveDialogOptions{
		Title:           "Save SnapVector document",
		DefaultFilename: suggestedName,
		Filters:         fileDialogFilters(),
	})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}

	path = ensureDocumentExtension(path)
	if err := a.writeDocument(path, contents); err != nil {
		return nil, err
	}
	return &DocumentSaveResult{Path: path, Name: filepath.Base(path)}, nil
}

func (a *App) writeDocument(path string, contents string) error {
	if strings.TrimSpace(contents) == "" {
		return fmt.Errorf("document contents cannot be empty")
	}
	if err := a.writeFile(path, []byte(contents), 0o600); err != nil {
		return fmt.Errorf("write document: %w", err)
	}
	return nil
}

func (a *App) waitOrCancel(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}

func (a *App) captureContext() (context.Context, context.CancelFunc) {
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	// 60s gives interactive region capture room for the user to look around
	// and drag. Non-interactive paths still finish in well under a second.
	return context.WithTimeout(base, 60*time.Second)
}

func displayData(meta capture.Meta) map[string]any {
	display := map[string]any{}
	if meta.DisplayID != "" {
		display["id"] = meta.DisplayID
	}
	if meta.Width > 0 {
		display["width"] = meta.Width
	}
	if meta.Height > 0 {
		display["height"] = meta.Height
	}
	if meta.X != 0 || meta.Y != 0 {
		display["x"] = meta.X
		display["y"] = meta.Y
	}
	if meta.ScaleFactor > 0 {
		display["scaleFactor"] = meta.ScaleFactor
	}
	return display
}

func captureRegionData(meta capture.Meta) map[string]any {
	if meta.Width <= 0 || meta.Height <= 0 {
		return nil
	}

	return map[string]any{
		"x":      meta.X,
		"y":      meta.Y,
		"width":  meta.Width,
		"height": meta.Height,
	}
}

func ensureDocumentExtension(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || strings.HasSuffix(strings.ToLower(path), ".sv.json") {
		return path
	}
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		return path[:len(path)-len(".json")] + ".sv.json"
	}
	return path + ".sv.json"
}

func defaultExportFilename(name string, format string) string {
	base := strings.TrimSpace(name)
	if base == "" {
		base = "snapvector-export"
	}
	base = filepath.Base(base)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.TrimSuffix(base, ".sv")
	base = strings.TrimSuffix(base, ".tar")
	base = strings.TrimSpace(base)
	if base == "" {
		base = "snapvector-export"
	}
	return ensureExportExtension(base, format)
}

func ensureExportExtension(path string, format string) string {
	path = strings.TrimSpace(path)
	format = strings.ToLower(strings.TrimSpace(format))
	if path == "" || format == "" {
		return path
	}
	ext := "." + format
	if strings.HasSuffix(strings.ToLower(path), ext) {
		return path
	}
	if currentExt := filepath.Ext(path); currentExt != "" {
		return strings.TrimSuffix(path, currentExt) + ext
	}
	return path + ext
}

func exportFileDialogFilters(format string) []wailsruntime.FileFilter {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "svg":
		return []wailsruntime.FileFilter{{DisplayName: "SVG Image (*.svg)", Pattern: "*.svg"}}
	case "png":
		return []wailsruntime.FileFilter{{DisplayName: "PNG Image (*.png)", Pattern: "*.png"}}
	case "jpg":
		return []wailsruntime.FileFilter{{DisplayName: "JPEG Image (*.jpg)", Pattern: "*.jpg"}}
	case "pdf":
		return []wailsruntime.FileFilter{{DisplayName: "PDF Document (*.pdf)", Pattern: "*.pdf"}}
	default:
		return nil
	}
}

func nextAvailableExportPath(dir string, filename string) (string, error) {
	dir = strings.TrimSpace(dir)
	filename = strings.TrimSpace(filename)
	if dir == "" || filename == "" {
		return "", fmt.Errorf("export folder and filename are required")
	}

	filename = filepath.Base(filename)
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	if base == "" {
		base = "snapvector-export"
	}

	candidate := filepath.Join(dir, filename)
	if _, err := os.Stat(candidate); err == nil {
		for suffix := 2; ; suffix++ {
			next := filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, suffix, ext))
			if _, err := os.Stat(next); os.IsNotExist(err) {
				return next, nil
			} else if err != nil {
				return "", fmt.Errorf("stat export target: %w", err)
			}
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat export target: %w", err)
	}

	return candidate, nil
}

func fileDialogFilters() []wailsruntime.FileFilter {
	// Wails darwin Save/Open dialogs map each extension token through
	// UTType.typeWithFilenameExtension. Multi-dot extensions like "sv.json"
	// can produce nil and crash the native dialog, so keep the filter to the
	// safe "*.json" token and enforce ".sv.json" through the filename/path.
	return []wailsruntime.FileFilter{
		{DisplayName: "SnapVector Documents (*.sv.json, *.json)", Pattern: "*.json"},
	}
}

func (a *App) GetHotkeys() ([]Hotkey, error) {
	return a.hotkeyStore.Load()
}

func (a *App) GetPreferences() (Preferences, error) {
	return a.preferencesStore.Load()
}

func (a *App) DefaultPreferences() Preferences {
	return a.preferencesStore.DefaultPreferences()
}

func (a *App) SavePreferences(preferences Preferences) (Preferences, error) {
	if err := a.preferencesStore.Save(preferences); err != nil {
		return Preferences{}, err
	}
	return a.preferencesStore.Load()
}

func (a *App) ResetPreferences() (Preferences, error) {
	return a.preferencesStore.Reset()
}

func (a *App) ChooseExportDirectory(current string) (string, error) {
	ctx, cancel := a.captureContext()
	defer cancel()

	path, err := a.openDirectoryDialog(ctx, wailsruntime.OpenDialogOptions{
		Title:                "Choose SnapVector export folder",
		DefaultDirectory:     strings.TrimSpace(current),
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	return filepath.Clean(path), nil
}

func (a *App) SaveHotkeys(bindings []Hotkey) error {
	if err := a.hotkeyStore.Save(bindings); err != nil {
		return err
	}
	a.reapplyGlobalHotkeys()
	return nil
}

func (a *App) ResetHotkeys() ([]Hotkey, error) {
	return a.hotkeyStore.Reset()
}

func (a *App) DefaultHotkeys() []Hotkey {
	return DefaultHotkeys()
}

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteOKEmitsCanonicalShape(t *testing.T) {
	var buf bytes.Buffer

	err := WriteOK(&buf, map[string]any{
		"format":   "png",
		"mimeType": "image/png",
		"base64":   "AAAA",
	})
	if err != nil {
		t.Fatalf("WriteOK returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v (raw=%q)", err, buf.String())
	}

	if got["status"] != "ok" {
		t.Fatalf("status = %v, want ok", got["status"])
	}
	if got["code"].(float64) != 0 {
		t.Fatalf("code = %v, want 0", got["code"])
	}
	if _, ok := got["error"]; ok {
		t.Fatal("ok response must not carry error field")
	}

	data := got["data"].(map[string]any)
	if data["format"] != "png" || data["mimeType"] != "image/png" || data["base64"] != "AAAA" {
		t.Fatalf("data payload mismatched: %v", data)
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Fatal("json output should end with newline")
	}
}

func TestWriteErrorEmitsCanonicalShape(t *testing.T) {
	var buf bytes.Buffer

	err := WriteError(&buf, CodePermissionDenied, "Screen capture permission denied", true, map[string]any{
		"platform": "darwin",
	})
	if err != nil {
		t.Fatalf("WriteError returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if got["status"] != "error" {
		t.Fatalf("status = %v, want error", got["status"])
	}
	if got["code"].(float64) != float64(CodePermissionDenied) {
		t.Fatalf("code = %v, want %d", got["code"], CodePermissionDenied)
	}
	if _, ok := got["data"]; ok {
		t.Fatal("error response must not carry data field")
	}

	errObj := got["error"].(map[string]any)
	if errObj["message"] != "Screen capture permission denied" {
		t.Fatalf("message = %v", errObj["message"])
	}
	if errObj["retryable"] != true {
		t.Fatalf("retryable = %v, want true", errObj["retryable"])
	}
}

func TestCodeRanges(t *testing.T) {
	cases := []struct {
		code int
		lo   int
		hi   int
	}{
		{CodeUsage, 1000, 1099},
		{CodeCaptureFailed, 1100, 1199},
		{CodePermissionDenied, 1200, 1299},
		{CodeUnsupportedPlatform, 1200, 1299},
		{CodeInjectInvalid, 1300, 1399},
		{CodeExportFailed, 1400, 1499},
	}

	for _, tc := range cases {
		if tc.code < tc.lo || tc.code > tc.hi {
			t.Fatalf("code %d not in [%d,%d]", tc.code, tc.lo, tc.hi)
		}
	}
}

package cli

import "testing"

func TestParseFlagsCaptureBase64(t *testing.T) {
	got, err := parseFlags([]string{"--capture", "--base64-stdout"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Capture || !got.Base64Stdout {
		t.Fatalf("flags = %+v", got)
	}
}

func TestParseFlagsInjectSVG(t *testing.T) {
	got, err := parseFlags([]string{"--inject-svg", "[]"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.InjectSVG != "[]" {
		t.Fatalf("inject payload = %q", got.InjectSVG)
	}
	if got.OutputFormat != "svg" {
		t.Fatalf("default output format = %q, want svg", got.OutputFormat)
	}
}

func TestParseFlagsUnknown(t *testing.T) {
	if _, err := parseFlags([]string{"--nope"}); err == nil {
		t.Fatal("expected error on unknown flag")
	}
}

func TestParseFlagsCaptureWithoutStdout(t *testing.T) {
	if _, err := parseFlags([]string{"--capture"}); err == nil {
		t.Fatal("expected error when --base64-stdout missing")
	}
}

func TestParseFlagsStdoutWithoutCapture(t *testing.T) {
	if _, err := parseFlags([]string{"--base64-stdout"}); err == nil {
		t.Fatal("expected error when --capture missing")
	}
}

func TestParseFlagsRejectsMixedCommands(t *testing.T) {
	if _, err := parseFlags([]string{"--capture", "--base64-stdout", "--inject-svg", "[]"}); err == nil {
		t.Fatal("expected error for mixed commands")
	}
}

func TestParseFlagsAcceptsOutputFormat(t *testing.T) {
	got, err := parseFlags([]string{"--inject-svg", "[]", "--output-format", "png", "--copy-to-clipboard"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.OutputFormat != "png" {
		t.Fatalf("output format = %q, want png", got.OutputFormat)
	}
	if !got.CopyToClipboard {
		t.Fatal("expected copy-to-clipboard to be true")
	}
}

func TestParseFlagsRejectsInvalidOutputFormat(t *testing.T) {
	if _, err := parseFlags([]string{"--inject-svg", "[]", "--output-format", "gif"}); err == nil {
		t.Fatal("expected invalid output format error")
	}
}

func TestParseFlagsRejectsOutputFormatWithoutInjectSVG(t *testing.T) {
	if _, err := parseFlags([]string{"--output-format", "png"}); err == nil {
		t.Fatal("expected output-format to require inject-svg")
	}
}

func TestParseFlagsRejectsClipboardWithoutInjectSVG(t *testing.T) {
	if _, err := parseFlags([]string{"--copy-to-clipboard"}); err == nil {
		t.Fatal("expected copy-to-clipboard to require inject-svg")
	}
}

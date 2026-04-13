package clipboarddoc

import (
	"context"
	"testing"
)

func TestWriteRejectsUnsupportedFormat(t *testing.T) {
	if err := Write(context.Background(), []byte("x"), "gif"); err == nil {
		t.Fatal("expected unsupported clipboard format error")
	}
}

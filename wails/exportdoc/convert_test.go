package exportdoc

import (
	"context"
	"testing"
)

func TestConvertRejectsUnsupportedFormat(t *testing.T) {
	_, _, err := Convert(context.Background(), "<svg/>", "gif")
	if err == nil {
		t.Fatal("expected unsupported format error")
	}
}

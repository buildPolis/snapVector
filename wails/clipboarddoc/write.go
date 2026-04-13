package clipboarddoc

import (
	"context"
	"fmt"
)

func Write(ctx context.Context, payload []byte, format string) error {
	switch format {
	case "svg", "png", "jpg", "pdf":
		return writePlatform(ctx, payload, format)
	default:
		return fmt.Errorf("unsupported clipboard format %q", format)
	}
}

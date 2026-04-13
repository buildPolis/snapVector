package exportdoc

import (
	"context"
	"fmt"
)

func Convert(ctx context.Context, svg string, format string) ([]byte, string, error) {
	switch format {
	case "png":
		return convertPNG(ctx, svg)
	case "jpg":
		return convertJPG(ctx, svg)
	case "pdf":
		return convertPDF(ctx, svg)
	default:
		return nil, "", fmt.Errorf("unsupported export format %q", format)
	}
}

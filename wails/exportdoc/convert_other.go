//go:build !darwin && !linux

package exportdoc

import (
	"context"
	"fmt"
	"runtime"
)

func convertPNG(context.Context, string) ([]byte, string, error) {
	return nil, "", fmt.Errorf("png export is not implemented on %s", runtime.GOOS)
}

func convertJPG(context.Context, string) ([]byte, string, error) {
	return nil, "", fmt.Errorf("jpg export is not implemented on %s", runtime.GOOS)
}

func convertPDF(context.Context, string) ([]byte, string, error) {
	return nil, "", fmt.Errorf("pdf export is not implemented on %s", runtime.GOOS)
}

//go:build !darwin

package clipboarddoc

import (
	"context"
	"fmt"
	"runtime"
)

func writePlatform(context.Context, []byte, string) error {
	return fmt.Errorf("clipboard output is not implemented on %s", runtime.GOOS)
}

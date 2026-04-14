//go:build linux

package capture

import (
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
)

// decodeAnyImage decodes PNG, JPEG, or any format registered via image.RegisterFormat.
func decodeAnyImage(r io.Reader) (image.Image, string, error) {
	return image.Decode(r)
}

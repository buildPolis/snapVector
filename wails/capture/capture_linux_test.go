//go:build linux

package capture

import "testing"

func TestShouldSkipPortal(t *testing.T) {
	cases := []struct {
		name          string
		sessionType   string
		hasNativeTool bool
		want          bool
	}{
		{
			name:          "x11 with gnome-screenshot skips portal",
			sessionType:   "x11",
			hasNativeTool: true,
			want:          true,
		},
		{
			name:          "x11 without gnome-screenshot still tries portal",
			sessionType:   "x11",
			hasNativeTool: false,
			want:          false,
		},
		{
			name:          "wayland always tries portal",
			sessionType:   "wayland",
			hasNativeTool: true,
			want:          false,
		},
		{
			name:          "wayland without tool still portal",
			sessionType:   "wayland",
			hasNativeTool: false,
			want:          false,
		},
		{
			name:          "unset session type tries portal",
			sessionType:   "",
			hasNativeTool: true,
			want:          false,
		},
		{
			name:          "tty session tries portal (portal may or may not fail, no worse than before)",
			sessionType:   "tty",
			hasNativeTool: true,
			want:          false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldSkipPortal(c.sessionType, c.hasNativeTool); got != c.want {
				t.Fatalf("shouldSkipPortal(%q, %v) = %v, want %v",
					c.sessionType, c.hasNativeTool, got, c.want)
			}
		})
	}
}

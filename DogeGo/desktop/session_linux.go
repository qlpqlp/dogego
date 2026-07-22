//go:build linux

package desktop

import "os"

func interactiveSession() bool {
	if os.Getenv("DOGEGO_HEADLESS") == "1" {
		return false
	}
	if os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "" {
		return true
	}
	return false
}

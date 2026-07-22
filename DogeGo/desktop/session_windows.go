//go:build windows

package desktop

import "os"

func interactiveSession() bool {
	if os.Getenv("DOGEGO_HEADLESS") == "1" {
		return false
	}
	return true
}

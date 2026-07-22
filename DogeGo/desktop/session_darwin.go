//go:build darwin

package desktop

import "os"

func interactiveSession() bool {
	if os.Getenv("DOGEGO_HEADLESS") == "1" {
		return false
	}
	if os.Getenv("SSH_CONNECTION") != "" && os.Getenv("DISPLAY") == "" {
		return false
	}
	return true
}

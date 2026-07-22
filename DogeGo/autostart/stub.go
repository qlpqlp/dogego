//go:build !windows && !linux && !darwin

package autostart

import "fmt"

func platformName() string { return "unsupported" }

func applyPlatform(Options) error {
	return fmt.Errorf("autostart: not supported on this operating system")
}

func removePlatform() error { return nil }

func statusPlatform() (bool, string, string) { return false, "", "" }

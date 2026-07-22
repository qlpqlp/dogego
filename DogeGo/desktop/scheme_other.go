//go:build !windows && !linux && !darwin

package desktop

import "fmt"

func registerURLSchemePlatform(scheme, exe string) error {
	return fmt.Errorf("custom URL protocol registration is not supported on this OS")
}

func unregisterURLSchemePlatform(scheme string) error {
	return fmt.Errorf("custom URL protocol registration is not supported on this OS")
}

func urlSchemeStatusPlatform(scheme string) (bool, string, error) {
	return false, "", nil
}

func platformTraySupported() bool { return false }

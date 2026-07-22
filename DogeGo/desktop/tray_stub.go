//go:build (!windows && !linux && !darwin) || (darwin && !cgo)

package desktop

func platformTraySupported() bool { return false }

func runTray(cfg TrayConfig, icon []byte) error {
	return nil
}

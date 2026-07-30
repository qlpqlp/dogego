//go:build (!windows && !linux && !darwin) || (darwin && !cgo)

package desktop

func platformTraySupported() bool { return false }

func platformTrayRequiresMainThread() bool { return false }

func platformQuitTray() {}

func runTray(cfg TrayConfig, icon []byte) error {
	return nil
}

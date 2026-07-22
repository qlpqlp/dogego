//go:build !windows && !linux && !darwin

package desktop

func runTray(cfg TrayConfig, icon []byte) error {
	return nil
}

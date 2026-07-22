//go:build windows

package launch

import (
	"os"
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func spawnDetachedNodePlatform(confPath string, tray bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{"node", "-nobrowser", "-tray=false"}
	if tray {
		args = []string{"node", "-nobrowser", "-tray"}
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "DOGECOINCONF="+confPath)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNoWindow,
		HideWindow:    true,
	}
	return cmd.Start()
}

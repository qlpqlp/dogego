//go:build !windows

package launch

import (
	"os"
	"os/exec"
	"syscall"
)

func spawnDetachedNodePlatform(confPath string, tray bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{"node", "-nobrowser"}
	if tray {
		args = append(args, "-tray")
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "DOGECOINCONF="+confPath)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	return cmd.Start()
}

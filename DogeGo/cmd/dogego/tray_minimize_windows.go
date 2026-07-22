//go:build windows

package main

import (
	"syscall"
	"time"
)

const swRestore = 9

func startTrayMinimizeWatcher() {
	go func() {
		kernel32 := syscall.NewLazyDLL("kernel32.dll")
		user32 := syscall.NewLazyDLL("user32.dll")
		getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
		isIconic := user32.NewProc("IsIconic")
		showWindow := user32.NewProc("ShowWindow")
		var wasIconic bool
		for {
			if !trayMinimizeOnClose.Load() {
				wasIconic = false
				time.Sleep(500 * time.Millisecond)
				continue
			}
			hwnd, _, _ := getConsoleWindow.Call()
			if hwnd == 0 {
				time.Sleep(400 * time.Millisecond)
				continue
			}
			iconic, _, _ := isIconic.Call(hwnd)
			if iconic != 0 {
				if !wasIconic {
					hideConsoleWindow()
					wasIconic = true
				}
			} else if wasIconic {
				// User restored from taskbar - show console again.
				_, _, _ = showWindow.Call(hwnd, swRestore)
				wasIconic = false
			}
			time.Sleep(300 * time.Millisecond)
		}
	}()
}

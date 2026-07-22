//go:build windows

package main

import (
	"syscall"
	"time"
)

const swHide = 0

func hideConsoleWindow() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	user32 := syscall.NewLazyDLL("user32.dll")
	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	showWindow := user32.NewProc("ShowWindow")
	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd != 0 {
		_, _, _ = showWindow.Call(hwnd, swHide)
	}
}

// startConsoleHideRetry hides the Windows console after the process attaches one (tray / web UI builds).
func startConsoleHideRetry() {
	go func() {
		for i := 0; i < 48; i++ {
			hideConsoleWindow()
			time.Sleep(250 * time.Millisecond)
		}
	}()
}

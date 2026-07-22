//go:build windows

package main

import "os"

func init() {
	if len(os.Args) > 1 && os.Args[1] == "open" {
		hideConsoleWindow()
	}
}

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import (
	"fmt"
	"os"
	"strings"

	"dogego/config"
)

const restartWaitPIDFlag = "-waitpid"

func parseWaitPIDArg(args []string) int {
	for _, arg := range args {
		if strings.HasPrefix(arg, restartWaitPIDFlag+"=") {
			n, err := parsePositiveInt(strings.TrimPrefix(arg, restartWaitPIDFlag+"="))
			if err == nil {
				return n
			}
		}
	}
	return 0
}

func parsePositiveInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int(c-'0')
		if n <= 0 {
			return 0, fmt.Errorf("not positive")
		}
	}
	return n, nil
}

func filterRestartArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == restartWaitPIDFlag || arg == replaceTargetFlag {
			if i+1 < len(args) {
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, restartWaitPIDFlag+"=") || strings.HasPrefix(arg, replaceTargetFlag+"=") {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func hasFlag(args []string, name string) bool {
	prefix := name + "="
	for _, arg := range args {
		if arg == name || strings.HasPrefix(arg, prefix) {
			return true
		}
	}
	return false
}

func buildRestartChildArgs(parentPID int) []string {
	restartSub := "node"
	extra := os.Args[1:]
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "node", "spvnode":
			restartSub = os.Args[1]
			extra = os.Args[2:]
		}
	}
	args := append([]string{restartSub}, filterRestartArgs(extra)...)
	appendConfigRestartFlags(&args)
	if !hasFlag(args, "-nobrowser") {
		args = append(args, "-nobrowser")
	}
	if parentPID > 0 {
		args = append(args, fmt.Sprintf("%s=%d", restartWaitPIDFlag, parentPID))
	}
	return args
}

func appendConfigRestartFlags(args *[]string) {
	if hasFlag(*args, "-datadir") {
		return
	}
	f, _ := config.LoadFirst()
	if f.DataDir == "" {
		return
	}
	*args = append(*args, "-datadir="+strings.TrimSpace(f.DataDir))
	if !hasFlag(*args, "-network") && strings.TrimSpace(f.Network) != "" {
		*args = append(*args, "-network="+strings.TrimSpace(f.Network))
	}
}

// buildApplyChildArgs returns argv for launching an update binary then replacing the install path.
func buildApplyChildArgs(parentPID int, replaceTarget string) []string {
	args := buildRestartChildArgs(parentPID)
	if strings.TrimSpace(replaceTarget) != "" {
		args = append(args, replaceTargetFlag+"="+replaceTarget)
	}
	return args
}

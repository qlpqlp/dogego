// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"dogego/config"
)

func certLoadConfig(confPath, datadirOverride string) (config.File, string, error) {
	var f config.File
	path := strings.TrimSpace(confPath)
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return f, "", fmt.Errorf("dogego cert: read config %s: %w", path, err)
		}
		if err := json.Unmarshal(b, &f); err != nil {
			return f, "", fmt.Errorf("dogego cert: parse config %s: %w", path, err)
		}
	} else {
		f, path = config.LoadFirst()
	}
	if d := strings.TrimSpace(datadirOverride); d != "" {
		f.DataDir = d
	}
	return f, path, nil
}

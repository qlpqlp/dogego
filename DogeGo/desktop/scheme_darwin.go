//go:build darwin

package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func handlerAppPath(scheme string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Applications", "DogeGo URL.app"), nil
}

func registerURLSchemePlatform(scheme, exe string) error {
	app, err := handlerAppPath(scheme)
	if err != nil {
		return err
	}
	contents := filepath.Join(app, "Contents")
	macos := filepath.Join(contents, "MacOS")
	if err := os.MkdirAll(macos, 0o755); err != nil {
		return err
	}
	script := fmt.Sprintf("#!/bin/sh\nexec %q open --url \"$1\"\n", exe)
	launcher := filepath.Join(macos, "dogego-url")
	if err := os.WriteFile(launcher, []byte(script), 0o755); err != nil {
		return err
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>CFBundleExecutable</key><string>dogego-url</string>
  <key>CFBundleIdentifier</key><string>com.dogego.url.%s</string>
  <key>CFBundleName</key><string>DogeGo URL</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleURLTypes</key>
  <array><dict>
    <key>CFBundleURLName</key><string>DogeGo Node</string>
    <key>CFBundleURLSchemes</key>
    <array><string>%s</string></array>
  </dict></array>
</dict></plist>
`, scheme, scheme)
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(plist), 0o644); err != nil {
		return err
	}
	_ = exec.Command("/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister", "-f", app).Run()
	return nil
}

func unregisterURLSchemePlatform(scheme string) error {
	app, err := handlerAppPath(scheme)
	if err != nil {
		return err
	}
	err = os.RemoveAll(app)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func urlSchemeStatusPlatform(scheme string) (bool, string, error) {
	app, err := handlerAppPath(scheme)
	if err != nil {
		return false, "", err
	}
	st, err := os.Stat(filepath.Join(app, "Contents", "Info.plist"))
	if os.IsNotExist(err) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	if st.IsDir() {
		return false, "", nil
	}
	return true, app, nil
}

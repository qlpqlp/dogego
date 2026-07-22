//go:build windows

package desktop

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

func registerURLSchemePlatform(scheme, exe string) error {
	base := `Software\Classes\` + scheme
	k, _, err := registry.CreateKey(registry.CURRENT_USER, base, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.SetStringValue("", "URL:DogeGo Node"); err != nil {
		return err
	}
	if err := k.SetStringValue("URL Protocol", ""); err != nil {
		return err
	}
	iconKey, _, err := registry.CreateKey(registry.CURRENT_USER, base+`\DefaultIcon`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	if err := iconKey.SetStringValue("", exe+",0"); err != nil {
		iconKey.Close()
		return err
	}
	iconKey.Close()
	cmdKey, _, err := registry.CreateKey(registry.CURRENT_USER, base+`\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer cmdKey.Close()
	return cmdKey.SetStringValue("", HandlerCommand(exe))
}

func unregisterURLSchemePlatform(scheme string) error {
	return registry.DeleteKey(registry.CURRENT_USER, `Software\Classes\`+scheme)
}

func urlSchemeStatusPlatform(scheme string) (bool, string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Classes\`+scheme+`\shell\open\command`, registry.QUERY_VALUE)
	if err != nil {
		return false, "", nil
	}
	defer k.Close()
	cmd, _, err := k.GetStringValue("")
	if err != nil {
		return false, "", err
	}
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return false, "", nil
	}
	return true, cmd, nil
}

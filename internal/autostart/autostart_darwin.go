//go:build darwin

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

const label = "com.github.kyungw00k.dbibackend"

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist")
}

func (m *Manager) IsEnabled() bool {
	_, err := os.Stat(plistPath())
	return err == nil
}

func (m *Manager) Enable() error {
	exe := executable()
	plst := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
</dict>
</plist>`, label, exe)

	if err := os.MkdirAll(filepath.Dir(plistPath()), 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}
	return os.WriteFile(plistPath(), []byte(plst), 0644)
}

func (m *Manager) Disable() error {
	return os.Remove(plistPath())
}

//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

func desktopPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "autostart", "dbibackend.desktop")
}

func (m *Manager) IsEnabled() bool {
	_, err := os.Stat(desktopPath())
	return err == nil
}

func (m *Manager) Enable() error {
	exe := executable()
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=dbibackend
Exec=%s
Hidden=false
NoDisplay=false
X-GNOME-Autostart-enabled=true
`, exe)

	if err := os.MkdirAll(filepath.Dir(desktopPath()), 0755); err != nil {
		return fmt.Errorf("create autostart dir: %w", err)
	}
	return os.WriteFile(desktopPath(), []byte(entry), 0644)
}

func (m *Manager) Disable() error {
	return os.Remove(desktopPath())
}

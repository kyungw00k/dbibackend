//go:build windows

package autostart

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const (
	regKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
	regName = "dbibackend"
)

func (m *Manager) IsEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	_, _, err = k.GetStringValue(regName)
	return err == nil
}

func (m *Manager) Enable() error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, regKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry key: %w", err)
	}
	defer k.Close()

	exe := executable()
	return k.SetStringValue(regName, `"`+exe+`"`)
}

func (m *Manager) Disable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry key: %w", err)
	}
	defer k.Close()

	return k.DeleteValue(regName)
}

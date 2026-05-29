package autostart

import "os"

type Manager struct{}

func New() *Manager {
	return &Manager{}
}

func executable() string {
	exe, err := os.Executable()
	if err != nil {
		return "dbibackend"
	}
	return exe
}

package menubar

import (
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/systray"
	"github.com/kyungw00k/dbibackend/internal/server"
)

type config struct {
	Paths []string `json:"paths"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dbibackend", "config.json")
}

func loadConfig() config {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return config{}
	}
	var cfg config
	json.Unmarshal(data, &cfg)
	return cfg
}

func (a *App) saveConfig() {
	cfg := config{Paths: a.paths}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.MkdirAll(filepath.Dir(configPath()), 0755)
	os.WriteFile(configPath(), data, 0644)
}

type App struct {
	logger *slog.Logger
	paths  []string
	mu     sync.Mutex
	running bool
	started bool
	stop    chan struct{}
	stopSrv chan struct{}

	mStatus    *systray.MenuItem
	mToggle    *systray.MenuItem
	mPaths     []*systray.MenuItem
	mAddDir    *systray.MenuItem
	mRemoveDir *systray.MenuItem
	sepHidden  bool
	mQuit      *systray.MenuItem
}

func NewApp(initialDir string, logger *slog.Logger) *App {
	cfg := loadConfig()
	paths := cfg.Paths
	if initialDir != "" {
		found := false
		for _, p := range paths {
			if p == initialDir {
				found = true
				break
			}
		}
		if !found {
			paths = append(paths, initialDir)
		}
	}

	return &App{
		logger:  logger,
		paths:   paths,
		stop:    make(chan struct{}),
		stopSrv: make(chan struct{}),
	}
}

func (a *App) Run() {
	systray.Run(a.onReady, a.onExit)
}

func (a *App) onReady() {
	systray.SetTitle("DBI")
	systray.SetTooltip("dbibackend — Switch USB installer")
	systray.SetIcon(iconDisconnected)

	a.mStatus = systray.AddMenuItem("Status: Stopped", "")
	a.mStatus.Disable()

	a.mToggle = systray.AddMenuItem("Start", "Start waiting for Switch")

	systray.AddSeparator()
	a.rebuildDynamicMenu()

	go a.handleEvents()
}

func (a *App) onExit() {
	close(a.stop)
}

func (a *App) rebuildDynamicMenu() {
	for _, item := range a.mPaths {
		item.Hide()
	}
	if a.mAddDir != nil {
		a.mAddDir.Hide()
	}
	if a.mRemoveDir != nil {
		a.mRemoveDir.Hide()
	}
	if a.mQuit != nil {
		a.mQuit.Hide()
	}
	a.mPaths = nil

	for i, p := range a.paths {
		item := systray.AddMenuItemCheckbox(a.displayDir(p), "", i == 0)
		if i == 0 {
			item.Check()
		}
		a.mPaths = append(a.mPaths, item)
	}

	a.mAddDir = systray.AddMenuItem("Add Directory...", "Add titles directory")
	a.mRemoveDir = systray.AddMenuItem("Remove All Directories", "Clear directory list")

	systray.AddSeparator()
	a.mQuit = systray.AddMenuItem("Quit", "Exit dbibackend")
}

func (a *App) activePaths() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string{}, a.paths...)
}

func (a *App) handleEvents() {
	for {
		select {
		case <-a.mToggle.ClickedCh:
			if a.started {
				a.stopServer()
			} else {
				a.startServer()
			}

		case <-a.mAddDir.ClickedCh:
			dir, err := pickDirectory()
			if err != nil {
				a.logger.Warn("directory picker cancelled", "err", err)
				continue
			}
			a.mu.Lock()
			for _, p := range a.paths {
				if p == dir {
					a.mu.Unlock()
					continue
				}
			}
			a.paths = append(a.paths, dir)
			a.saveConfig()
			a.rebuildDynamicMenu()
			a.mu.Unlock()
			a.logger.Info("directory added", "dir", dir)

		case <-a.mRemoveDir.ClickedCh:
			a.mu.Lock()
			a.paths = nil
			a.saveConfig()
			a.rebuildDynamicMenu()
			a.mu.Unlock()
			a.logger.Info("all directories removed")

		case <-a.mQuit.ClickedCh:
			if a.started {
				a.stopServer()
			}
			systray.Quit()
			return

		case <-a.stop:
			return
		}
	}
}

func (a *App) startServer() {
	a.mu.Lock()
	a.started = true
	a.stopSrv = make(chan struct{})
	a.mu.Unlock()

	a.mToggle.SetTitle("Stop")
	a.mStatus.SetTitle("Status: Waiting for Switch...")
	systray.SetIcon(iconWaiting)

	go a.connectLoop()
	a.logger.Info("server started")
}

func (a *App) stopServer() {
	a.mu.Lock()
	if !a.started {
		a.mu.Unlock()
		return
	}
	a.started = false
	close(a.stopSrv)
	a.mu.Unlock()

	a.mToggle.SetTitle("Start")
	a.mStatus.SetTitle("Status: Stopped")
	systray.SetIcon(iconDisconnected)
	a.logger.Info("server stopped")
}

func (a *App) connectLoop() {
	for {
		select {
		case <-a.stopSrv:
			return
		default:
		}

		paths := a.activePaths()
		if len(paths) == 0 {
			a.mStatus.SetTitle("Status: No directory")
			systray.SetIcon(iconDisconnected)
			a.waitForStopSrv()
			continue
		}

		a.logger.Info("waiting for switch", "paths", paths)

		usb, err := server.WaitForSwitch(a.logger)
		if err != nil {
			select {
			case <-a.stopSrv:
				return
			default:
			}
			a.logger.Error("connection failed", "err", err)
			continue
		}

		select {
		case <-a.stopSrv:
			usb.Close()
			return
		default:
		}

		a.mStatus.SetTitle("Status: Connected")
		systray.SetIcon(iconConnected)
		a.logger.Info("switch connected")

		a.mu.Lock()
		a.running = true
		a.mu.Unlock()

		srv := server.NewMulti(usb, paths, a.logger)
		if err := srv.Run(); err != nil {
			a.logger.Info("session ended", "err", err)
		}
		usb.Close()

		a.mu.Lock()
		a.running = false
		a.mu.Unlock()

		a.mStatus.SetTitle("Status: Waiting for Switch...")
		systray.SetIcon(iconWaiting)
	}
}

func (a *App) waitForStopSrv() {
	<-a.stopSrv
}

func (a *App) waitForStopOrDir() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	select {
	case <-sigCh:
		systray.Quit()
	case <-a.stop:
	}
}

func (a *App) displayDir(p string) string {
	home, _ := os.UserHomeDir()
	if home != "" {
		if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") {
			return "~/" + rel
		}
	}
	return p
}

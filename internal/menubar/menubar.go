package menubar

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/systray"
	"github.com/kyungw00k/dbibackend/internal/autostart"
	"github.com/kyungw00k/dbibackend/internal/server"
	"github.com/kyungw00k/dbibackend/internal/update"
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

type updateState int

const (
	updateIdle      updateState = iota
	updateAvailable              // new version info cached, ready to download
	updateReady                  // downloaded, ready to restart
)

type App struct {
	logger  *slog.Logger
	paths   []string
	mu      sync.Mutex
	running bool
	started bool
	stop    chan struct{}
	stopSrv chan struct{}

	mStatus    *systray.MenuItem
	mToggle    *systray.MenuItem
	mPaths     []*systray.MenuItem
	mAddDir    *systray.MenuItem
	mRemoveDir *systray.MenuItem
	mAutoStart *systray.MenuItem
	mUpdate    *systray.MenuItem
	mQuit      *systray.MenuItem
	autoStart  *autostart.Manager
	srv        *server.Server

	version       string
	updater       *update.Updater
	updateState   updateState
	pendingRelease *update.Release
	pendingArchive string
}

func NewApp(initialDir string, version string, logger *slog.Logger) *App {
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
		logger:    logger,
		paths:     paths,
		stop:      make(chan struct{}),
		stopSrv:   make(chan struct{}),
		autoStart: autostart.New(),
		version:   version,
		updater:   update.NewUpdater(version, logger),
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

	a.mAutoStart = systray.AddMenuItemCheckbox("Launch at Login", "Start automatically on login", a.autoStart.IsEnabled())

	a.mUpdate = systray.AddMenuItem(fmt.Sprintf("Check for Updates... (v%s)", a.version), "Check for new versions")

	systray.AddSeparator()
	a.rebuildDynamicMenu()

	go a.handleEvents()
}

func (a *App) onExit() {
	a.mu.Lock()
	if a.srv != nil {
		a.srv.Stop()
	}
	select {
	case <-a.stopSrv:
	default:
		close(a.stopSrv)
	}
	a.mu.Unlock()
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

	for _, p := range a.paths {
		item := systray.AddMenuItem(a.displayDir(p), "")
		item.Disable()
		a.mPaths = append(a.mPaths, item)
	}

	a.mAddDir = systray.AddMenuItem("Add Directory...", "Add titles directory")
	a.mRemoveDir = systray.AddMenuItem("Remove All Directories", "Clear directory list")

	if a.started {
		a.mAddDir.Disable()
		a.mRemoveDir.Disable()
	}

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

		case <-a.mAutoStart.ClickedCh:
			if a.autoStart.IsEnabled() {
				if err := a.autoStart.Disable(); err != nil {
					a.logger.Error("disable autostart", "err", err)
					continue
				}
				a.mAutoStart.Uncheck()
				a.logger.Info("autostart disabled")
			} else {
				if err := a.autoStart.Enable(); err != nil {
					a.logger.Error("enable autostart", "err", err)
					continue
				}
				a.mAutoStart.Check()
				a.logger.Info("autostart enabled")
			}

		case <-a.mUpdate.ClickedCh:
			a.mu.Lock()
			state := a.updateState
			a.mu.Unlock()

			switch state {
			case updateIdle:
				go a.checkForUpdate()
			case updateAvailable:
				go a.downloadUpdate()
			case updateReady:
				a.restartWithUpdate()
			}

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

func (a *App) resetUpdateMenu() {
	a.mu.Lock()
	a.updateState = updateIdle
	a.pendingRelease = nil
	a.pendingArchive = ""
	a.mu.Unlock()
	a.mUpdate.SetTitle(fmt.Sprintf("Check for Updates... (v%s)", a.version))
	a.mUpdate.Enable()
}

func (a *App) checkForUpdate() {
	a.mUpdate.SetTitle("Checking for updates...")
	a.mUpdate.Disable()

	release, available, err := a.updater.Check()
	if err != nil {
		a.mUpdate.SetTitle(fmt.Sprintf("Update check failed: %s", err))
		a.mUpdate.Enable()
		time.AfterFunc(5*time.Second, a.resetUpdateMenu)
		return
	}

	if !available {
		a.mUpdate.SetTitle(fmt.Sprintf("Up to date (v%s)", a.version))
		a.mUpdate.Enable()
		return
	}

	a.mu.Lock()
	a.updateState = updateAvailable
	a.pendingRelease = release
	a.mu.Unlock()

	tagName := strings.TrimPrefix(release.TagName, "v")
	a.mUpdate.SetTitle(fmt.Sprintf("Update available: v%s → v%s", a.version, tagName))
	a.mUpdate.Enable()
}

func (a *App) downloadUpdate() {
	a.mu.Lock()
	release := a.pendingRelease
	a.mu.Unlock()

	if release == nil {
		a.resetUpdateMenu()
		return
	}

	tagName := strings.TrimPrefix(release.TagName, "v")
	a.mUpdate.SetTitle(fmt.Sprintf("Downloading v%s...", tagName))
	a.mUpdate.Disable()

	archivePath, err := a.updater.Download(release)
	if err != nil {
		a.mUpdate.SetTitle(fmt.Sprintf("Download failed: %s", err))
		a.mUpdate.Enable()
		time.AfterFunc(5*time.Second, a.resetUpdateMenu)
		return
	}

	a.mu.Lock()
	a.updateState = updateReady
	a.pendingArchive = archivePath
	a.mu.Unlock()

	a.mUpdate.SetTitle(fmt.Sprintf("Restart to update to v%s", tagName))
	a.mUpdate.Enable()
}

func (a *App) restartWithUpdate() {
	a.mUpdate.SetTitle("Updating...")
	a.mUpdate.Disable()

	if err := a.updater.Apply(a.pendingArchive); err != nil {
		a.mUpdate.SetTitle(fmt.Sprintf("Update failed: %s", err))
		a.mUpdate.Enable()
		time.AfterFunc(5*time.Second, a.resetUpdateMenu)
		return
	}

	if err := update.Restart(); err != nil {
		a.logger.Error("restart failed", "err", err)
		a.mUpdate.SetTitle("Restart failed — please restart manually")
		a.mUpdate.Enable()
		return
	}

	systray.Quit()
}

func (a *App) startServer() {
	a.mu.Lock()
	a.started = true
	a.stopSrv = make(chan struct{})
	a.mu.Unlock()

	a.mToggle.SetTitle("Stop")
	a.mStatus.SetTitle("Status: Waiting for Switch...")
	systray.SetIcon(iconWaiting)
	a.rebuildDynamicMenu()

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
	if a.srv != nil {
		a.srv.Stop()
	}
	close(a.stopSrv)
	a.mu.Unlock()

	a.mToggle.SetTitle("Start")
	a.mStatus.SetTitle("Status: Stopped")
	systray.SetIcon(iconDisconnected)
	a.rebuildDynamicMenu()
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

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			select {
			case <-a.stopSrv:
				cancel()
			case <-ctx.Done():
			}
		}()

		usb, err := server.WaitForSwitchCtx(ctx, a.logger)
		if err != nil {
			cancel()
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
		a.mu.Lock()
		a.srv = srv
		a.mu.Unlock()

		if err := srv.Run(); err != nil {
			a.logger.Info("session ended", "err", err)
		}
		usb.Close()

		a.mu.Lock()
		a.running = false
		a.srv = nil
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

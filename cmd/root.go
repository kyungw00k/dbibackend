package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/kyungw00k/dbibackend/internal/menubar"
	"github.com/kyungw00k/dbibackend/internal/server"
)

var (
	Version     = "dev"
	debug       bool
	cliMode     bool
)

var rootCmd = &cobra.Command{
	Use:     "dbibackend [titles_dir]",
	Short:   "Install local titles into Nintendo Switch via USB",
	Version: Version,
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		level := slog.LevelInfo
		if debug {
			level = slog.LevelDebug
		}
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

		var titlesDir string
		if len(args) > 0 {
			titlesDir = args[0]
			info, err := os.Stat(titlesDir)
			if err != nil {
				return fmt.Errorf("cannot access path: %w", err)
			}
			if !info.IsDir() {
				return fmt.Errorf("specified path must be a directory")
			}
		}

		if !cliMode {
			app := menubar.NewApp(titlesDir, logger)
			app.Run()
			return nil
		}

		if titlesDir == "" {
			return fmt.Errorf("titles_dir is required in CLI mode")
		}

		logger.Info("connecting to switch", "dir", titlesDir)
		usb, err := server.WaitForSwitch(logger)
		if err != nil {
			return fmt.Errorf("connect: %w", err)
		}
		defer usb.Close()

		srv := server.New(usb, titlesDir, logger)
		return srv.Run()
	},
}

func init() {
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.Flags().BoolVar(&debug, "debug", false, "enable debug output")
	rootCmd.Flags().BoolVar(&cliMode, "cli", false, "run in CLI mode (default: menu bar)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

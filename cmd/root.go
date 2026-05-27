package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/kyungw00k/dbibackend/internal/server"
)

var (
	debug bool
)

var rootCmd = &cobra.Command{
	Use:   "dbibackend [titles_dir]",
	Short: "Install local titles into Nintendo Switch via USB",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		titlesDir := args[0]

		info, err := os.Stat(titlesDir)
		if err != nil {
			return fmt.Errorf("cannot access path: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("specified path must be a directory")
		}

		level := slog.LevelInfo
		if debug {
			level = slog.LevelDebug
		}
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

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

func Execute() {
	rootCmd.Flags().BoolVar(&debug, "debug", false, "enable debug output")
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

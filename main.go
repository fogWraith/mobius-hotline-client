package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/charmbracelet/log"
	"github.com/jhalter/mobius-hotline-client/ui"
	"github.com/muesli/termenv"
)

// Values swapped in by go-releaser at build time
var (
	version = "dev"
)

func main() {
	configDir := flag.String("config", defaultConfigPath(), "Path to config root")

	flag.Parse()

	// init DebugBuffer
	db := &ui.DebugBuffer{}

	logHandler := log.New(db)

	// Force color output for logger.
	// By default, the charm logger package disables color for non-TTY.
	logHandler.SetColorProfile(termenv.TrueColor)
	logHandler.SetLevel(log.DebugLevel)

	logger := slog.New(logHandler)
	logger.Info("Started Mobius client", "Version", version)

	model := ui.NewModel(*configDir, logger, db)
	if err := model.Start(); err != nil {
		logger.Error("Application error", "err", err)
		os.Exit(1)
	}
}

func defaultConfigPath() (cfgPath string) {
	switch runtime.GOOS {
	case "windows":
		cfgPath = "mobius-client-config.yaml"
	case "darwin":
		if _, err := os.Stat("/usr/local/etc/mobius-client-config.yaml"); err == nil {
			cfgPath = "/usr/local/etc/mobius-client-config.yaml"
		} else if _, err := os.Stat("/opt/homebrew/etc/mobius-client-config.yaml"); err == nil {
			cfgPath = "/opt/homebrew/etc/mobius-client-config.yaml"
		} else {
			cfgPath = "mobius-client-config.yaml"
		}
	case "linux":
		cfgPath = "/usr/local/etc/mobius-client-config.yaml"
	default:
		fmt.Printf("unsupported OS")
		os.Exit(1)
	}

	return cfgPath
}

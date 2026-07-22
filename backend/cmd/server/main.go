package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/louisboii747/syncspace/backend/internal/diagnostics"
	"github.com/louisboii747/syncspace/backend/internal/serverapp"
)

var buildVersion = "dev"

func main() {
	if handled, code := handleCommand(os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
	logBuffer := diagnostics.NewLogBuffer(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}), 500)
	logger := slog.New(logBuffer)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := serverapp.Run(ctx, logger, logBuffer, buildVersion); err != nil {
		logger.Error("Server stopped", "error", err)
		os.Exit(1)
	}
}

func handleCommand(args []string, stdout, stderr io.Writer) (bool, int) {
	if len(args) == 0 {
		return false, 0
	}
	if len(args) == 1 {
		switch args[0] {
		case "version", "--version":
			fmt.Fprintf(stdout, "SyncSpace %s\n", buildVersion)
			return true, 0
		case "help", "-h", "--help":
			fmt.Fprintln(stdout, "Usage: syncspace [--version|--help]")
			fmt.Fprintln(stdout, "Run without arguments to start the local SyncSpace service.")
			return true, 0
		}
	}
	fmt.Fprintf(stderr, "syncspace: unsupported argument: %s\n", strings.Join(args, " "))
	fmt.Fprintln(stderr, "Run 'syncspace --help' for usage.")
	return true, 2
}

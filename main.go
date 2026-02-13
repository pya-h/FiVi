package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"fivi/internal/config"
	"fivi/internal/engine"
	"fivi/internal/logx"
)

func main() {
	cfg := config.ParseFlags()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logx.Infof("fivi started with %d sources, workers=%d", len(cfg.BootstrapSources), cfg.Workers)
	if err := engine.Run(ctx, cfg); err != nil && !errors.Is(err, context.Canceled) {
		logx.Errorf("fatal: %v", err)
		os.Exit(1)
	}
	logx.Infof("fivi stopped")
}

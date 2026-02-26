package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fivi/internal/config"
	"fivi/internal/engine"
	"fivi/internal/logx"
	"fivi/internal/ui"
)

func main() {
	cfg := config.ParseFlags()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mode := ui.DetectMode(cfg.NoTUI)
	switch {
	case mode == ui.ModeTUI:
		logx.Infof("ui mode=tui (interactive terminal)")
	case cfg.NoTUI:
		logx.Infof("ui mode=log (--no-tui)")
	default:
		logx.Infof("ui mode=log (non-interactive terminal)")
	}

	logx.Infof("fivi started with engine=%s, mode=%s, profile=%s, sources=%d, workers=%d", cfg.Engine, cfg.Mode, cfg.Profile, len(cfg.BootstrapSources), cfg.Workers)
	if err := runWithMode(ctx, cfg, mode); err != nil && !errors.Is(err, context.Canceled) {
		logx.Errorf("fatal: %v", err)
		os.Exit(1)
	}
	logx.Infof("fivi stopped")
}

func runWithMode(ctx context.Context, cfg config.AppConfig, mode ui.Mode) error {
	buffer := cfg.Workers * 8
	if buffer < 128 {
		buffer = 128
	}
	events := make(chan engine.WorkerEvent, buffer)

	if mode == ui.ModeTUI {
		return runTUI(ctx, cfg, events)
	}

	return runLogMode(ctx, cfg, events)
}

func runTUI(ctx context.Context, cfg config.AppConfig, events chan engine.WorkerEvent) error {
	engineCtx, cancelEngine := context.WithCancel(ctx)
	defer cancelEngine()

	engineDone := make(chan error, 1)
	go func() {
		defer close(events)
		engineDone <- engine.RunWithEvents(engineCtx, cfg, events)
	}()

	uiErr := ui.RunDashboard(engineCtx, events)
	cancelEngine()
	engineErr := <-engineDone
	if uiErr != nil && !errors.Is(uiErr, context.Canceled) {
		return uiErr
	}
	return engineErr
}

func runLogMode(ctx context.Context, cfg config.AppConfig, events chan engine.WorkerEvent) error {
	reporterCtx, reporterCancel := context.WithCancel(ctx)
	reporterDone := make(chan struct{})
	go func() {
		defer close(reporterDone)
		ui.RunLogProgressReporter(reporterCtx, events, 2*time.Second)
	}()

	err := engine.RunWithEvents(ctx, cfg, events)
	reporterCancel()
	close(events)
	<-reporterDone
	return err
}

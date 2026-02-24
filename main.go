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

	logx.Infof("fivi started with %d sources, workers=%d", len(cfg.BootstrapSources), cfg.Workers)
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

	reporterCtx, reporterCancel := context.WithCancel(ctx)
	reporterDone := make(chan struct{})
	go func() {
		defer close(reporterDone)
		switch mode {
		case ui.ModeTUI:
			for {
				select {
				case <-reporterCtx.Done():
					return
				case _, ok := <-events:
					if !ok {
						return
					}
				}
			}
		default:
			ui.RunLogProgressReporter(reporterCtx, events, 2*time.Second)
		}
	}()

	err := engine.RunWithEvents(ctx, cfg, events)
	reporterCancel()
	close(events)
	<-reporterDone
	return err
}

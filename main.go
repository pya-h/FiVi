package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fivi/internal/cleanup"
	"fivi/internal/config"
	"fivi/internal/engine"
	"fivi/internal/logx"
	"fivi/internal/netstats"
	"fivi/internal/speedtest"
	"fivi/internal/ui"
)

// cleanupMgr is the global cleanup manager. Engine code registers cleanup
// hooks (proxy restore, TUN teardown, engine kill) via engine.SetCleanupManager.
var cleanupMgr = cleanup.New()

func main() {
	// Handle subcommands before flag parsing.
	if len(os.Args) > 1 && os.Args[1] == "fetch" {
		// Strip "fetch" so flag.Parse() sees remaining flags.
		os.Args = append(os.Args[:1], os.Args[2:]...)
		runFetchOnly()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "speedtest" {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		runSpeedTest()
		return
	}

	cfg := config.ParseFlags()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Ensure cleanup runs on any exit path.
	engine.SetCleanupManager(cleanupMgr)
	defer cleanupMgr.Run()

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
		cleanupMgr.Run()
		os.Exit(1)
	}
	logx.Infof("fivi stopped")
}

func runFetchOnly() {
	cfg := config.ParseFlags()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logx.Infof("fivi fetch: fetching configs from %d sources", len(cfg.BootstrapSources))
	if err := engine.FetchOnly(ctx, cfg); err != nil {
		logx.Errorf("fetch failed: %v", err)
		os.Exit(1)
	}
}

func runSpeedTest() {
	fs := flag.NewFlagSet("speedtest", flag.ExitOnError)
	socksAddr := fs.String("socks-addr", "127.0.0.1:10808", "SOCKS proxy address to test through")
	downloadURL := fs.String("download-url", "", "download test URL (default: Cloudflare)")
	uploadURL := fs.String("upload-url", "", "upload test URL (default: Cloudflare)")
	latencyURL := fs.String("latency-url", "", "latency test URL (default: gstatic)")
	timeout := fs.Duration("timeout", speedtest.DefaultTimeout, "overall test timeout")
	pings := fs.Int("pings", speedtest.DefaultLatencyPings, "number of latency pings")
	fs.Parse(os.Args[1:])

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := speedtest.Config{
		SocksAddr:    *socksAddr,
		DownloadURL:  *downloadURL,
		UploadURL:    *uploadURL,
		LatencyURL:   *latencyURL,
		Timeout:      *timeout,
		LatencyPings: *pings,
		Log: func(format string, args ...any) {
			fmt.Printf(format+"\n", args...)
		},
	}

	result, err := speedtest.Run(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "speed test failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(speedtest.FormatResult(result))
}

func runWithMode(ctx context.Context, cfg config.AppConfig, mode ui.Mode) error {
	buffer := cfg.Workers * 8
	if buffer < 128 {
		buffer = 128
	}
	events := make(chan engine.WorkerEvent, buffer)
	status := make(chan engine.StatusUpdate, 64)

	// Start netstats monitor if enabled.
	var nsChan <-chan netstats.Stats
	if cfg.NetStats {
		iface := cfg.NetStatsIface
		if iface == "" {
			detected, err := netstats.DetectInterface(cfg.IsTunnelMode())
			if err != nil {
				logx.Warnf("netstats: interface detection failed: %v (stats disabled)", err)
			} else {
				iface = detected
			}
		}
		if iface != "" {
			logx.Infof("netstats: monitoring interface %s", iface)
			nsChan = netstats.StartMonitor(ctx, iface, 1*time.Second)
		}
	}

	if mode == ui.ModeTUI {
		return runTUI(ctx, cfg, events, status, nsChan)
	}

	return runLogMode(ctx, cfg, events, status, nsChan)
}

func runTUI(ctx context.Context, cfg config.AppConfig, events chan engine.WorkerEvent, status chan engine.StatusUpdate, ns <-chan netstats.Stats) error {
	engineCtx, cancelEngine := context.WithCancel(ctx)
	defer cancelEngine()

	engineDone := make(chan error, 1)
	go func() {
		defer close(events)
		defer close(status)
		engineDone <- engine.RunWithEvents(engineCtx, cfg, events, status)
	}()

	uiErr := ui.RunDashboard(engineCtx, events, status, ns)
	cancelEngine()
	engineErr := <-engineDone
	if uiErr != nil && !errors.Is(uiErr, context.Canceled) {
		return uiErr
	}
	return engineErr
}

func runLogMode(ctx context.Context, cfg config.AppConfig, events chan engine.WorkerEvent, status chan engine.StatusUpdate, ns <-chan netstats.Stats) error {
	reporterCtx, reporterCancel := context.WithCancel(ctx)
	reporterDone := make(chan struct{})
	go func() {
		defer close(reporterDone)
		ui.RunLogProgressReporter(reporterCtx, events, status, 2*time.Second, ns)
	}()

	err := engine.RunWithEvents(ctx, cfg, events, status)
	reporterCancel()
	close(events)
	close(status)
	<-reporterDone
	return err
}

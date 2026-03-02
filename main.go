package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"fivi/internal/binmgr"
	"fivi/internal/cleanup"
	"fivi/internal/config"
	"fivi/internal/daemon"
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
	if len(os.Args) > 1 && os.Args[1] == "install" {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		runInstall()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "update" {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		runUpdate()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "daemon" {
		runDaemon(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "stop" {
		runStop()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "status" {
		runStatus()
		return
	}

	cfg := config.ParseFlags()
	if cfg.CheckUpdate {
		runStartupUpdateCheck()
	}
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

func runInstall() {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	fs.Parse(os.Args[1:])

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	installer, err := binmgr.NewInstaller()
	if err != nil {
		fmt.Fprintf(os.Stderr, "install failed: %v\n", err)
		os.Exit(1)
	}

	results, err := installer.InstallAll(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "install failed: %v\n", err)
		os.Exit(1)
	}

	for _, res := range results {
		fmt.Printf("installed %s %s -> %s\n", res.Engine, res.Version, res.Path)
	}
}

func runUpdate() {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	fs.Parse(os.Args[1:])

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	installer, err := binmgr.NewInstaller()
	if err != nil {
		fmt.Fprintf(os.Stderr, "update failed: %v\n", err)
		os.Exit(1)
	}

	statuses, err := installer.UpdateAll(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "update failed: %v\n", err)
		os.Exit(1)
	}

	hadErr := false
	for _, st := range statuses {
		if st.Err != nil {
			hadErr = true
			fmt.Printf("%s: error: %v\n", st.Engine, st.Err)
			continue
		}
		switch {
		case st.Updated:
			fmt.Printf("%s: updated to %s (%s)\n", st.Engine, st.Latest, st.Path)
		case st.UpdateAvailable:
			fmt.Printf("%s: update available %s -> %s\n", st.Engine, st.Installed, st.Latest)
		default:
			fmt.Printf("%s: up to date (%s)\n", st.Engine, st.Latest)
		}
	}
	if hadErr {
		os.Exit(1)
	}
}

func runStartupUpdateCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	installer, err := binmgr.NewInstaller()
	if err != nil {
		logx.Warnf("update-check: %v", err)
		return
	}
	statuses, err := installer.CheckForUpdates(ctx)
	if err != nil {
		logx.Warnf("update-check: %v", err)
		return
	}
	for _, st := range statuses {
		if st.Err != nil {
			logx.Warnf("update-check: %s: %v", st.Engine, st.Err)
			continue
		}
		if st.UpdateAvailable {
			if st.Installed == "" {
				logx.Warnf("update-check: %s not installed (latest %s); run './fivi install'", st.Engine, st.Latest)
			} else {
				logx.Warnf("update-check: %s outdated (%s < %s); run './fivi update'", st.Engine, st.Installed, st.Latest)
			}
		}
	}
}

func runDaemon(args []string) {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	runChild := fs.Bool("run-child", false, "internal flag: run daemon child process")
	fs.Parse(args)
	appArgs := fs.Args()

	if *runChild {
		runDaemonChild(appArgs)
		return
	}

	paths, err := daemon.DefaultPaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon failed: %v\n", err)
		os.Exit(1)
	}
	if pid, err := daemon.ReadPID(paths.PIDFile); err == nil && daemon.ProcessRunning(pid) {
		fmt.Fprintf(os.Stderr, "daemon already running with pid %d\n", pid)
		os.Exit(1)
	}

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon failed: %v\n", err)
		os.Exit(1)
	}

	cmdArgs := append([]string{"daemon", "--run-child"}, appArgs...)
	cmd := exec.Command(exe, cmdArgs...)
	devNull, _ := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if devNull != nil {
		defer devNull.Close()
		cmd.Stdin = devNull
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}
	daemon.ConfigureDetachedProcess(cmd)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "daemon failed to start: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("daemon started pid=%d\n", cmd.Process.Pid)
	fmt.Printf("log file: %s\n", paths.LogFile)
}

func runDaemonChild(appArgs []string) {
	paths, err := daemon.DefaultPaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon child failed: %v\n", err)
		os.Exit(1)
	}
	logFile, err := daemon.OpenLogFile(paths.LogFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon child failed: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	restoreLog := logx.SetOutput(io.MultiWriter(logFile, os.Stdout))
	defer restoreLog()

	if err := daemon.WritePID(paths.PIDFile, os.Getpid()); err != nil {
		logx.Errorf("daemon child: failed writing pid file: %v", err)
		os.Exit(1)
	}
	defer daemon.RemovePID(paths.PIDFile)

	os.Args = append([]string{os.Args[0]}, appArgs...)
	cfg := config.ParseFlags()
	cfg.NoTUI = true

	if cfg.CheckUpdate {
		runStartupUpdateCheck()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	engine.SetCleanupManager(cleanupMgr)
	defer cleanupMgr.Run()

	logx.Infof("daemon mode started pid=%d", os.Getpid())
	if err := runWithMode(ctx, cfg, ui.ModeLog); err != nil && !errors.Is(err, context.Canceled) {
		logx.Errorf("daemon fatal: %v", err)
		cleanupMgr.Run()
		os.Exit(1)
	}
	logx.Infof("daemon mode stopped")
}

func runStop() {
	paths, err := daemon.DefaultPaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "stop failed: %v\n", err)
		os.Exit(1)
	}
	pid, err := daemon.ReadPID(paths.PIDFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon is not running\n")
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stop failed: %v\n", err)
		os.Exit(1)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "stop failed: %v\n", err)
		os.Exit(1)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !daemon.ProcessRunning(pid) {
			daemon.RemovePID(paths.PIDFile)
			fmt.Printf("daemon stopped pid=%d\n", pid)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Printf("daemon stop signal sent pid=%d (still shutting down)\n", pid)
}

func runStatus() {
	paths, err := daemon.DefaultPaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "status failed: %v\n", err)
		os.Exit(1)
	}
	pid, err := daemon.ReadPID(paths.PIDFile)
	if err != nil {
		fmt.Println("daemon not running")
		return
	}
	if daemon.ProcessRunning(pid) {
		fmt.Printf("daemon running pid=%d log=%s\n", pid, paths.LogFile)
		return
	}
	daemon.RemovePID(paths.PIDFile)
	fmt.Printf("daemon not running (stale pid %s removed)\n", strconv.Itoa(pid))
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

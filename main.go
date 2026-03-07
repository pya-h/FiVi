package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"fivi/internal/binmgr"
	"fivi/internal/cleanup"
	"fivi/internal/config"
	"fivi/internal/daemon"
	"fivi/internal/dns"
	"fivi/internal/doctor"
	"fivi/internal/engine"
	"fivi/internal/logx"
	"fivi/internal/netstats"
	"fivi/internal/profile"
	"fivi/internal/routing"
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
	if len(os.Args) > 1 && os.Args[1] == "doctor" {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		runDoctor()
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
	if len(os.Args) > 1 && os.Args[1] == "restart" {
		runRestart(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "logs" {
		runLogs(os.Args[2:])
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

func runDoctor() {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	mode := fs.String("mode", "proxy", "connection mode to validate assumptions for: proxy|tunnel|urltest")
	profileName := fs.String("profile", "iran", "profile used for default routing rules when no explicit routing rules are provided")
	routingRulesCSV := fs.String("routing-rules", "", "comma-separated routing rules (e.g. domain:*.ir,domain:!forced.com,range:10.0.0.0/8)")
	routingRulesFile := fs.String("routing-rules-file", "", "path to routing rules file (one rule per line)")
	dnsPlain := fs.String("dns", "", "plain DNS server for direct resolution (bootstrap resolver)")
	dnsDoH := fs.String("doh", "", "DNS-over-HTTPS/TLS resolver for proxy-routed resolution")
	fs.Parse(os.Args[1:])

	selectedProfile, err := profile.Resolve(*profileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doctor failed: %v\n", err)
		os.Exit(1)
	}

	rulesInput := "profile-defaults"
	var rules []routing.Rule
	var rulesErr error
	switch {
	case strings.TrimSpace(*routingRulesFile) != "":
		rules, rulesErr = routing.LoadFromFile(strings.TrimSpace(*routingRulesFile))
		rulesInput = "file"
	case strings.TrimSpace(*routingRulesCSV) != "":
		rules, rulesErr = routing.ParseCSV(strings.TrimSpace(*routingRulesCSV))
		rulesInput = "inline"
	default:
		rules = routing.ProfileDefaults(selectedProfile.Name)
	}

	dnsCfg := dns.Default()
	if strings.TrimSpace(*dnsPlain) != "" {
		dnsCfg.Direct = strings.TrimSpace(*dnsPlain)
	}
	if strings.TrimSpace(*dnsDoH) != "" {
		dnsCfg.Remote = strings.TrimSpace(*dnsDoH)
	}

	report := doctor.Diagnose(doctor.Options{
		Mode:              *mode,
		DNS:               dnsCfg,
		RoutingRules:      rules,
		RoutingRulesErr:   rulesErr,
		RuleInput:         rulesInput,
		BothRuleInputsSet: strings.TrimSpace(*routingRulesFile) != "" && strings.TrimSpace(*routingRulesCSV) != "",
	}, doctor.RuntimeEnvironment())

	fmt.Print(doctor.FormatReport(report))
	if report.HasFailures() {
		os.Exit(1)
	}
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
	runChild := false
	appArgs := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--run-child":
			runChild = true
		case "-h", "--help":
			fmt.Println("usage: ./fivi daemon [fivi flags]")
			fmt.Println("starts FiVi in background mode; all normal flags are forwarded")
			return
		default:
			appArgs = append(appArgs, arg)
		}
	}

	if runChild {
		runDaemonChild(appArgs)
		return
	}

	paths, err := daemon.DefaultPaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "daemon failed: %v\n", err)
		os.Exit(1)
	}
	if pid, err := daemon.ReadPID(paths.PIDFile); err == nil {
		switch {
		case daemon.ProcessLooksLikeFiViDaemon(pid):
			fmt.Fprintf(os.Stderr, "daemon already running with pid %d\n", pid)
			os.Exit(1)
		case daemon.ProcessRunning(pid):
			fmt.Fprintf(os.Stderr, "refusing to reuse pid file: pid %d belongs to another running process\n", pid)
			os.Exit(1)
		default:
			daemon.RemovePID(paths.PIDFile)
		}
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
	startedAt := time.Now()
	if err := daemon.WritePIDMetadata(paths.PIDMetaFile, daemon.PIDMetadata{
		PID:       os.Getpid(),
		State:     "started",
		StartedAt: startedAt,
		UpdatedAt: startedAt,
	}); err != nil {
		logx.Warnf("daemon child: failed writing pid metadata: %v", err)
	}
	defer daemon.RemovePID(paths.PIDFile)
	defer daemon.RemovePIDMetadata(paths.PIDMetaFile)
	defer daemon.RemoveHeartbeat(paths.HealthFile)

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
	if err := runDaemonWithHealth(ctx, cfg, paths, startedAt); err != nil && !errors.Is(err, context.Canceled) {
		logx.Errorf("daemon fatal: %v", err)
		cleanupMgr.Run()
		os.Exit(1)
	}
	logx.Infof("daemon mode stopped")
}

func runDaemonWithHealth(ctx context.Context, cfg config.AppConfig, paths daemon.Paths, startedAt time.Time) error {
	buffer := cfg.Workers * 8
	if buffer < 128 {
		buffer = 128
	}
	events := make(chan engine.WorkerEvent, buffer)
	statusSrc := make(chan engine.StatusUpdate, 64)
	statusForReporter := make(chan engine.StatusUpdate, 64)

	var nsChan <-chan netstats.Stats
	if cfg.NetStats {
		iface := cfg.NetStatsIface
		if iface == "" {
			detected, err := netstats.DetectInterface(cfg.IsTunnelMode())
			if err == nil {
				iface = detected
			}
		}
		if iface != "" {
			nsChan = netstats.StartMonitor(ctx, iface, 1*time.Second)
		}
	}

	reporterCtx, reporterCancel := context.WithCancel(ctx)
	reporterDone := make(chan struct{})
	go func() {
		defer close(reporterDone)
		ui.RunLogProgressReporter(reporterCtx, events, statusForReporter, 2*time.Second, nsChan)
	}()

	healthDone := make(chan struct{})
	go func() {
		defer close(healthDone)
		defer close(statusForReporter)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		hb := daemon.Heartbeat{PID: os.Getpid(), Phase: "started", UpdatedAt: time.Now()}
		readyWritten := false

		flush := func() {
			if err := daemon.WriteHeartbeat(paths.HealthFile, hb); err != nil {
				logx.Warnf("daemon heartbeat write failed: %v", err)
			}
		}

		for {
			select {
			case <-ctx.Done():
				hb.UpdatedAt = time.Now()
				flush()
				return
			case update, ok := <-statusSrc:
				if !ok {
					hb.UpdatedAt = time.Now()
					flush()
					return
				}

				select {
				case statusForReporter <- update:
				default:
				}

				hb.Phase = string(update.Phase)
				hb.NodeName = update.NodeName
				hb.Protocol = update.Protocol
				hb.Ping = update.Ping
				hb.ProbeOK = update.ProbeOK
				hb.ProbeTotal = update.ProbeTotal
				hb.Failures = update.Failures
				if update.ProbeOK > 0 {
					hb.LastSuccessfulProbe = time.Now()
				}
				hb.UpdatedAt = time.Now()

				if !readyWritten && update.Phase == engine.PhaseMonitoring {
					readyWritten = true
					if err := daemon.WritePIDMetadata(paths.PIDMetaFile, daemon.PIDMetadata{
						PID:       os.Getpid(),
						State:     "ready",
						StartedAt: startedAt,
						ReadyAt:   time.Now(),
						UpdatedAt: time.Now(),
					}); err != nil {
						logx.Warnf("daemon pid metadata update failed: %v", err)
					}
				}
			case <-ticker.C:
				hb.UpdatedAt = time.Now()
				flush()
			}
		}
	}()

	err := engine.RunWithEvents(ctx, cfg, events, statusSrc)
	close(events)
	close(statusSrc)
	reporterCancel()
	<-healthDone
	<-reporterDone
	return err
}

func runStop() {
	stopDaemonIfRunning(false)
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
	if daemon.ProcessLooksLikeFiViDaemon(pid) {
		state := "unknown"
		if meta, err := daemon.ReadPIDMetadata(paths.PIDMetaFile); err == nil && meta.PID == pid {
			state = meta.State
		}
		fmt.Printf("daemon running pid=%d state=%s log=%s\n", pid, state, paths.LogFile)
		return
	}
	if daemon.ProcessRunning(pid) {
		fmt.Printf("daemon pid file points to non-FiVi process pid=%d (manual cleanup may be required)\n", pid)
		os.Exit(1)
	}
	daemon.RemovePID(paths.PIDFile)
	fmt.Printf("daemon not running (stale pid %s removed)\n", strconv.Itoa(pid))
}

func runRestart(args []string) {
	_ = stopDaemonIfRunning(true)
	runDaemon(args)
}

func runLogs(args []string) {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	tail := fs.Int("tail", 80, "number of lines to show from daemon log")
	_ = fs.Parse(args)

	paths, err := daemon.DefaultPaths()
	if err != nil {
		fmt.Fprintf(os.Stderr, "logs failed: %v\n", err)
		os.Exit(1)
	}
	lines, err := tailFile(paths.LogFile, *tail)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logs failed: %v\n", err)
		os.Exit(1)
	}
	for _, line := range lines {
		fmt.Println(line)
	}
}

func stopDaemonIfRunning(quiet bool) bool {
	paths, err := daemon.DefaultPaths()
	if err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "stop failed: %v\n", err)
		}
		return false
	}
	pid, err := daemon.ReadPID(paths.PIDFile)
	if err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "daemon is not running\n")
		}
		return false
	}
	if !daemon.ProcessRunning(pid) {
		daemon.RemovePID(paths.PIDFile)
		daemon.RemovePIDMetadata(paths.PIDMetaFile)
		daemon.RemoveHeartbeat(paths.HealthFile)
		if !quiet {
			fmt.Printf("daemon not running (stale pid %d removed)\n", pid)
		}
		return false
	}
	if !daemon.ProcessLooksLikeFiViDaemon(pid) {
		if !quiet {
			fmt.Fprintf(os.Stderr, "refusing to stop pid %d: not a FiVi daemon process\n", pid)
		}
		os.Exit(1)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "stop failed: %v\n", err)
		}
		os.Exit(1)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "stop failed: %v\n", err)
		}
		os.Exit(1)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !daemon.ProcessRunning(pid) {
			daemon.RemovePID(paths.PIDFile)
			daemon.RemovePIDMetadata(paths.PIDMetaFile)
			daemon.RemoveHeartbeat(paths.HealthFile)
			if !quiet {
				fmt.Printf("daemon stopped pid=%d\n", pid)
			}
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !quiet {
		fmt.Printf("daemon stop signal sent pid=%d (still shutting down)\n", pid)
	}
	return true
}

func tailFile(path string, n int) ([]string, error) {
	if n <= 0 {
		n = 80
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ring := make([]string, 0, n)
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		if len(ring) < n {
			ring = append(ring, line)
			continue
		}
		copy(ring, ring[1:])
		ring[n-1] = line
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return ring, nil
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

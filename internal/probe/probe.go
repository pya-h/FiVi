package probe

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

type TargetResult struct {
	URL     string
	OK      bool
	Latency time.Duration
	Status  int
	Err     string
}

type Result struct {
	Successes   int
	TargetCount int
	AvgLatency  time.Duration
	Targets     []TargetResult
}

func CheckThroughSOCKS(ctx context.Context, socksAddr string, targets []string, timeout time.Duration) Result {
	res := Result{TargetCount: len(targets), Targets: make([]TargetResult, 0, len(targets))}
	if len(targets) == 0 {
		return res
	}

	client, err := newSOCKSClient(socksAddr, timeout)
	if err != nil {
		for _, t := range targets {
			res.Targets = append(res.Targets, TargetResult{URL: t, Err: err.Error()})
		}
		return res
	}

	type item struct {
		idx int
		r   TargetResult
	}
	items := make(chan item, len(targets))
	wg := sync.WaitGroup{}
	for idx, target := range targets {
		idx := idx
		target := target
		wg.Add(1)
		go func() {
			defer wg.Done()
			t, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			start := time.Now()
			status, reqErr := request204Or200(t, client, target)
			lat := time.Since(start)
			r := TargetResult{URL: target, Status: status, Latency: lat}
			if reqErr != nil {
				r.Err = reqErr.Error()
			} else {
				r.OK = true
			}
			items <- item{idx: idx, r: r}
		}()
	}
	wg.Wait()
	close(items)

	ordered := make([]TargetResult, len(targets))
	totalLatency := time.Duration(0)
	for it := range items {
		ordered[it.idx] = it.r
	}
	for _, r := range ordered {
		res.Targets = append(res.Targets, r)
		if r.OK {
			res.Successes++
			totalLatency += r.Latency
		}
	}

	if res.Successes > 0 {
		res.AvgLatency = totalLatency / time.Duration(res.Successes)
	}
	return res
}

func MeasureDownloadSpeedMbps(ctx context.Context, socksAddr, target string, timeout time.Duration, maxBytes int64) (float64, error) {
	if maxBytes <= 0 {
		maxBytes = 128 * 1024
	}
	client, err := newSOCKSClient(socksAddr, timeout)
	if err != nil {
		return 0, err
	}
	t, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(t, http.MethodGet, target, nil)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	n, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return 0, err
	}
	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 || n <= 0 {
		return 0, fmt.Errorf("insufficient data for speed measurement")
	}
	return (float64(n*8) / 1_000_000.0) / elapsed, nil
}

func request204Or200(ctx context.Context, client *http.Client, target string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 256))
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return resp.StatusCode, nil
	}
	return resp.StatusCode, fmt.Errorf("status %d", resp.StatusCode)
}

func newSOCKSClient(socksAddr string, timeout time.Duration) (*http.Client, error) {
	base := &net.Dialer{Timeout: timeout}
	dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, base)
	if err != nil {
		return nil, err
	}

	dialContext := func(c context.Context, network, addr string) (net.Conn, error) {
		if d, ok := dialer.(proxy.ContextDialer); ok {
			return d.DialContext(c, network, addr)
		}
		return dialer.Dial(network, addr)
	}
	tr := &http.Transport{
		DialContext:           dialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		DisableKeepAlives:     true,
	}
	return &http.Client{Transport: tr, Timeout: timeout}, nil
}

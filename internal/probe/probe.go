package probe

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
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

	dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, &net.Dialer{Timeout: timeout})
	if err != nil {
		for _, t := range targets {
			res.Targets = append(res.Targets, TargetResult{URL: t, Err: err.Error()})
		}
		return res
	}

	tr := &http.Transport{
		DialContext: func(c context.Context, network, addr string) (net.Conn, error) {
			if deadline, ok := c.Deadline(); ok {
				_ = deadline
			}
			return dialer.Dial(network, addr)
		},
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		DisableKeepAlives:     true,
	}
	client := &http.Client{Transport: tr, Timeout: timeout}

	totalLatency := time.Duration(0)
	for _, target := range targets {
		t, cancel := context.WithTimeout(ctx, timeout)
		start := time.Now()
		status, err := request204Or200(t, client, target)
		lat := time.Since(start)
		cancel()

		r := TargetResult{URL: target, Status: status, Latency: lat}
		if err != nil {
			r.Err = err.Error()
			res.Targets = append(res.Targets, r)
			continue
		}
		r.OK = true
		res.Successes++
		totalLatency += lat
		res.Targets = append(res.Targets, r)

		select {
		case <-t.Done():
		default:
		}
	}

	if res.Successes > 0 {
		res.AvgLatency = totalLatency / time.Duration(res.Successes)
	}
	return res
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
	return resp.StatusCode, errors.New(fmt.Sprintf("status %d", resp.StatusCode))
}

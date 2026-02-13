package fetch

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type SourceResult struct {
	Source string
	Links  []string
	Err    error
}

func FetchAll(ctx context.Context, sources []string, timeout time.Duration) []SourceResult {
	results := make([]SourceResult, len(sources))
	wg := sync.WaitGroup{}
	for i, src := range sources {
		i := i
		src := src
		wg.Add(1)
		go func() {
			defer wg.Done()
			links, err := FetchOne(ctx, src, timeout)
			results[i] = SourceResult{Source: src, Links: links, Err: err}
		}()
	}
	wg.Wait()
	return results
}

func FetchOne(ctx context.Context, source string, timeout time.Duration) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	return DecodeSubscription(body)
}

func DecodeSubscription(raw []byte) ([]string, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return nil, errors.New("empty response")
	}

	// Many subscriptions are either plain lines or one big base64 blob.
	if maybeBase64Blob(text) {
		decoded, err := tryDecodeBase64(text)
		if err == nil {
			text = strings.TrimSpace(decoded)
		}
	}

	return parseLines(text), nil
}

func parseLines(text string) []string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 1024), 2<<20)
	out := make([]string, 0, 4096)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := scanner.Err(); err != nil {
		return out
	}
	return out
}

func maybeBase64Blob(s string) bool {
	if strings.Contains(s, "\n") {
		return false
	}
	if len(s) < 32 {
		return false
	}
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '+' || r == '/' || r == '_' || r == '-' || r == '=' {
			continue
		}
		return false
	}
	return true
}

func tryDecodeBase64(s string) (string, error) {
	s = strings.TrimSpace(s)
	decoders := []*base64.Encoding{base64.StdEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.RawURLEncoding}
	for _, d := range decoders {
		b, err := d.DecodeString(s)
		if err == nil {
			return string(b), nil
		}
	}
	return "", errors.New("invalid base64")
}

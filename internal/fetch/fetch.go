package fetch

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
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

	links := parseLines(text)
	if hasSupportedScheme(links) {
		return links, nil
	}

	// Some feeds provide JSON arrays/objects containing link strings.
	if jsonLinks := extractLinksFromJSON(text); len(jsonLinks) > 0 {
		return jsonLinks, nil
	}

	// Many subscriptions are either plain lines or one base64 blob, sometimes
	// wrapped with line breaks/whitespace by mirrors or CDNs.
	compact := compactBase64Text(text)
	if maybeBase64Blob(compact) {
		decoded, err := tryDecodeBase64(compact)
		if err == nil {
			decodedLinks := parseLines(strings.TrimSpace(decoded))
			if len(decodedLinks) > 0 {
				return decodedLinks, nil
			}
		}
	}

	return links, nil
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

func compactBase64Text(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\n', '\r', '\t':
			return -1
		default:
			return r
		}
	}, s)
}

func hasSupportedScheme(links []string) bool {
	for _, l := range links {
		if isSupportedLink(l) {
			return true
		}
	}
	return false
}

func extractLinksFromJSON(text string) []string {
	var payload any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, 256)
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for _, value := range x {
				walk(value)
			}
		case []any:
			for _, item := range x {
				walk(item)
			}
		case string:
			for _, link := range extractSupportedLinksFromText(x) {
				if _, ok := seen[link]; ok {
					continue
				}
				seen[link] = struct{}{}
				out = append(out, link)
			}
		}
	}

	walk(payload)
	return out
}

func extractSupportedLinksFromText(text string) []string {
	lines := parseLines(text)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if isSupportedLink(line) {
			out = append(out, strings.TrimSpace(line))
		}
	}
	if len(out) > 0 {
		return out
	}

	compact := compactBase64Text(text)
	if maybeBase64Blob(compact) {
		decoded, err := tryDecodeBase64(compact)
		if err == nil {
			decodedLines := parseLines(decoded)
			for _, line := range decodedLines {
				if isSupportedLink(line) {
					out = append(out, strings.TrimSpace(line))
				}
			}
		}
	}
	return out
}

func isSupportedLink(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return strings.HasPrefix(v, "vmess://") ||
		strings.HasPrefix(v, "vless://") ||
		strings.HasPrefix(v, "trojan://") ||
		strings.HasPrefix(v, "ss://")
}

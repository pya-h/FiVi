package fetch

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestDecodeSubscriptionBase64WithWhitespace(t *testing.T) {
	plain := "vmess://abc\nvless://def\n"
	blob := base64.StdEncoding.EncodeToString([]byte(plain))
	wrapped := strings.Join([]string{blob[:10], blob[10:30], blob[30:]}, "\n")

	links, err := DecodeSubscription([]byte(wrapped))
	if err != nil {
		t.Fatalf("DecodeSubscription error: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d (%v)", len(links), links)
	}
	if links[0] != "vmess://abc" || links[1] != "vless://def" {
		t.Fatalf("unexpected links: %v", links)
	}
}

func TestDecodeSubscriptionKeepsPlainTextWhenNotSupportedSchemes(t *testing.T) {
	raw := "not-a-v2ray-link\nstill-not\n"
	links, err := DecodeSubscription([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeSubscription error: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 lines, got %d (%v)", len(links), links)
	}
}

func TestDecodeSubscriptionJSONPayload(t *testing.T) {
	raw := `{"items":[{"url":"vmess://abc"},{"meta":{"link":"vless://def"}}],"other":"noop"}`
	links, err := DecodeSubscription([]byte(raw))
	if err != nil {
		t.Fatalf("DecodeSubscription error: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d (%v)", len(links), links)
	}
	if links[0] != "vmess://abc" || links[1] != "vless://def" {
		t.Fatalf("unexpected links: %v", links)
	}
}

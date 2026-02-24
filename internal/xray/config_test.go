package xray

import (
	"encoding/base64"
	"testing"
)

func TestParseURIUppercaseVMessPrefix(t *testing.T) {
	payload := `{"ps":"n1","add":"example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"tcp","tls":"tls"}`
	link := "VMESS://" + base64.StdEncoding.EncodeToString([]byte(payload))

	parsed, err := ParseURI(link)
	if err != nil {
		t.Fatalf("ParseURI error: %v", err)
	}
	if parsed.Protocol != "vmess" {
		t.Fatalf("expected vmess protocol, got %q", parsed.Protocol)
	}
}

func TestParseURIUppercaseSSPrefix(t *testing.T) {
	link := "SS://YWVzLTI1Ni1nY206cGFzc0BleGFtcGxlLmNvbTo0NDM="

	parsed, err := ParseURI(link)
	if err != nil {
		t.Fatalf("ParseURI error: %v", err)
	}
	if parsed.Protocol != "ss" {
		t.Fatalf("expected ss protocol, got %q", parsed.Protocol)
	}
}

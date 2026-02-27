package latency

import (
	"encoding/base64"
	"testing"
)

func TestExtractVMessAddr(t *testing.T) {
	payload := `{"add":"1.2.3.4","port":"443","id":"uuid","aid":"0","net":"ws","tls":"tls","ps":"test"}`
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	uri := "vmess://" + encoded

	addr, err := ExtractServerAddr(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "1.2.3.4:443" {
		t.Fatalf("expected 1.2.3.4:443, got %s", addr)
	}
}

func TestExtractVMessAddrNumericPort(t *testing.T) {
	payload := `{"add":"example.com","port":8080,"id":"uuid"}`
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	uri := "vmess://" + encoded

	addr, err := ExtractServerAddr(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "example.com:8080" {
		t.Fatalf("expected example.com:8080, got %s", addr)
	}
}

func TestExtractVLESSAddr(t *testing.T) {
	uri := "vless://uuid@server.example.com:443?type=tcp&security=tls#name"
	addr, err := ExtractServerAddr(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "server.example.com:443" {
		t.Fatalf("expected server.example.com:443, got %s", addr)
	}
}

func TestExtractTrojanAddr(t *testing.T) {
	uri := "trojan://password@trojan.example.com:8443?type=tcp#name"
	addr, err := ExtractServerAddr(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "trojan.example.com:8443" {
		t.Fatalf("expected trojan.example.com:8443, got %s", addr)
	}
}

func TestExtractSSAddr(t *testing.T) {
	// method:password@host:port format, base64-encoded
	payload := "aes-256-gcm:password@ss.example.com:8388"
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	uri := "ss://" + encoded + "#name"

	addr, err := ExtractServerAddr(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "ss.example.com:8388" {
		t.Fatalf("expected ss.example.com:8388, got %s", addr)
	}
}

func TestExtractSSAddrPlainAt(t *testing.T) {
	uri := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@1.2.3.4:8388#name"
	addr, err := ExtractServerAddr(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "1.2.3.4:8388" {
		t.Fatalf("expected 1.2.3.4:8388, got %s", addr)
	}
}

func TestExtractUnsupportedScheme(t *testing.T) {
	_, err := ExtractServerAddr("http://example.com")
	if err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
}

func TestExtractEmptyAddress(t *testing.T) {
	payload := `{"add":"","port":"443","id":"uuid"}`
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))
	uri := "vmess://" + encoded

	_, err := ExtractServerAddr(uri)
	if err == nil {
		t.Fatal("expected error for empty address")
	}
}

func TestExtractIPv6Addr(t *testing.T) {
	uri := "vless://uuid@[2001:db8::1]:443?type=tcp&security=tls#name"
	addr, err := ExtractServerAddr(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "[2001:db8::1]:443" {
		t.Fatalf("expected [2001:db8::1]:443, got %s", addr)
	}
}

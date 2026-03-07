package engine

import (
	"testing"

	"fivi/internal/config"
	"fivi/internal/engines"
)

func TestAdapterFromConfig(t *testing.T) {
	tests := []struct {
		name      string
		engine    string
		wantName  string
		wantError bool
	}{
		{name: "xray", engine: "xray", wantName: "xray"},
		{name: "singbox", engine: "singbox", wantName: "singbox"},
		{name: "sing-box alias", engine: "sing-box", wantName: "singbox"},
		{name: "hysteria2", engine: "hysteria2", wantName: "hysteria2"},
		{name: "hy2 alias", engine: "hy2", wantName: "hysteria2"},
		{name: "invalid", engine: "unknown", wantError: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adapter, err := adapterFromConfig(config.AppConfig{Engine: tc.engine})
			if tc.wantError {
				if err == nil {
					t.Fatalf("expected error for engine=%q", tc.engine)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if adapter.Name() != tc.wantName {
				t.Fatalf("adapter.Name()=%q, want %q", adapter.Name(), tc.wantName)
			}
		})
	}
}

func TestBinaryForAdapter(t *testing.T) {
	cfg := config.AppConfig{
		XRAYBin:      "/usr/bin/xray",
		SingboxBin:   "/usr/bin/sing-box",
		Hysteria2Bin: "/usr/bin/hysteria2",
	}
	if got := binaryForAdapter(cfg, engines.NewXrayAdapter()); got != "/usr/bin/xray" {
		t.Fatalf("xray bin mismatch: %q", got)
	}
	if got := binaryForAdapter(cfg, engines.NewSingboxAdapter()); got != "/usr/bin/sing-box" {
		t.Fatalf("singbox bin mismatch: %q", got)
	}
	if got := binaryForAdapter(cfg, engines.NewHysteria2Adapter()); got != "/usr/bin/hysteria2" {
		t.Fatalf("hysteria2 bin mismatch: %q", got)
	}
}

package engine

import (
	"testing"
	"time"

	"fivi/internal/config"
)

func TestEffectiveSpeedSampleTimeoutCapsLargeProbeTimeout(t *testing.T) {
	cfg := config.AppConfig{ProbeTimeout: 12 * time.Second}
	if got := effectiveSpeedSampleTimeout(cfg); got != maxSpeedSampleTimeout {
		t.Fatalf("effectiveSpeedSampleTimeout=%s want=%s", got, maxSpeedSampleTimeout)
	}
}

func TestEffectiveSpeedSampleTimeoutPreservesSmallProbeTimeout(t *testing.T) {
	cfg := config.AppConfig{ProbeTimeout: 500 * time.Millisecond}
	if got := effectiveSpeedSampleTimeout(cfg); got != 500*time.Millisecond {
		t.Fatalf("effectiveSpeedSampleTimeout=%s want=500ms", got)
	}
}

func TestEffectiveSpeedSampleTimeoutDefaultsWhenUnset(t *testing.T) {
	cfg := config.AppConfig{}
	if got := effectiveSpeedSampleTimeout(cfg); got != maxSpeedSampleTimeout {
		t.Fatalf("effectiveSpeedSampleTimeout=%s want=%s", got, maxSpeedSampleTimeout)
	}
}

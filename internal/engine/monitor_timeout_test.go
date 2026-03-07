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

func TestEffectiveURLTestBenchmarkTimeoutCapsLargeCandidateTimeout(t *testing.T) {
	cfg := config.AppConfig{CandidateTimeout: 20 * time.Second}
	if got := effectiveURLTestBenchmarkTimeout(cfg); got != maxURLTestBenchmarkTimeout {
		t.Fatalf("effectiveURLTestBenchmarkTimeout=%s want=%s", got, maxURLTestBenchmarkTimeout)
	}
}

func TestEffectiveURLTestBenchmarkTimeoutPreservesSmallCandidateTimeout(t *testing.T) {
	cfg := config.AppConfig{CandidateTimeout: 900 * time.Millisecond}
	if got := effectiveURLTestBenchmarkTimeout(cfg); got != 900*time.Millisecond {
		t.Fatalf("effectiveURLTestBenchmarkTimeout=%s want=900ms", got)
	}
}

func TestEffectiveURLTestBenchmarkTimeoutDefaultsWhenUnset(t *testing.T) {
	cfg := config.AppConfig{}
	if got := effectiveURLTestBenchmarkTimeout(cfg); got != maxURLTestBenchmarkTimeout {
		t.Fatalf("effectiveURLTestBenchmarkTimeout=%s want=%s", got, maxURLTestBenchmarkTimeout)
	}
}

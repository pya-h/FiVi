package sources

import (
	"sort"
	"sync"
	"time"
)

const (
	cooldownStartFailures = 3
	cooldownBase          = 2 * time.Minute
	cooldownMaxMultiplier = 5
)

type HealthStat struct {
	Source              string
	Successes           int
	Failures            int
	ConsecutiveFailures int
	TotalYield          int
	LastYield           int
	LastOK              time.Time
	LastSeen            time.Time
	CooldownUntil       time.Time
}

type HealthTracker struct {
	mu    sync.Mutex
	stats map[string]*HealthStat
}

func NewHealthTracker(sourceURLs []string) *HealthTracker {
	stats := make(map[string]*HealthStat, len(sourceURLs))
	for _, src := range sourceURLs {
		stats[src] = &HealthStat{Source: src}
	}
	return &HealthTracker{stats: stats}
}

func (t *HealthTracker) Observe(source string, linkCount int, err error, now time.Time) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	stat := t.ensure(source)
	stat.LastSeen = now
	if err != nil {
		stat.Failures++
		stat.ConsecutiveFailures++
		if stat.ConsecutiveFailures >= cooldownStartFailures {
			stat.CooldownUntil = now.Add(cooldownDuration(stat.ConsecutiveFailures))
		}
		return
	}

	stat.Successes++
	stat.TotalYield += linkCount
	stat.LastYield = linkCount
	stat.LastOK = now
	stat.ConsecutiveFailures = 0
	stat.CooldownUntil = time.Time{}
}

func (t *HealthTracker) IsCoolingDown(source string, now time.Time) (bool, time.Time) {
	if t == nil {
		return false, time.Time{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	stat := t.ensure(source)
	if stat.CooldownUntil.IsZero() {
		return false, time.Time{}
	}
	if now.Before(stat.CooldownUntil) {
		return true, stat.CooldownUntil
	}
	stat.CooldownUntil = time.Time{}
	return false, time.Time{}
}

func (t *HealthTracker) Snapshot() []HealthStat {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	out := make([]HealthStat, 0, len(t.stats))
	for _, stat := range t.stats {
		out = append(out, *stat)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Source < out[j].Source
	})
	return out
}

func (t *HealthTracker) ensure(source string) *HealthStat {
	stat, ok := t.stats[source]
	if ok {
		return stat
	}
	stat = &HealthStat{Source: source}
	t.stats[source] = stat
	return stat
}

func cooldownDuration(consecutiveFailures int) time.Duration {
	multiplier := consecutiveFailures - (cooldownStartFailures - 1)
	if multiplier < 1 {
		multiplier = 1
	}
	if multiplier > cooldownMaxMultiplier {
		multiplier = cooldownMaxMultiplier
	}
	return time.Duration(multiplier) * cooldownBase
}

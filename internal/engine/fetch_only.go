package engine

import (
	"context"
	"fmt"
	"time"

	"fivi/internal/config"
	"fivi/internal/fetch"
	"fivi/internal/logx"
	"fivi/internal/parser"
	"fivi/internal/sources"
)

// FetchOnly fetches all sources, saves results to cache, writes
// output/all_configs.txt, and prints a summary. No engine is started.
func FetchOnly(ctx context.Context, cfg config.AppConfig) error {
	health := sources.NewHealthTracker(cfg.BootstrapSources)
	roundSources := selectSourcesForRound(cfg.BootstrapSources, health, time.Now())
	logx.Infof("fetching subscriptions from %d sources", len(roundSources))

	fetchTimeout := cfg.FetchTimeout
	if fetchTimeout <= 0 {
		fetchTimeout = 25 * time.Second
	}
	sourceResults := fetch.FetchAll(ctx, roundSources, fetchTimeout)

	var totalLinks int
	var okSources, failSources int
	all := make([]string, 0, 4096)
	for _, r := range sourceResults {
		if r.Err == nil && len(r.Links) > 0 {
			fetch.SaveToCache(r.Source, r.Links)
			okSources++
			totalLinks += len(r.Links)
			logx.Infof("source ok %s (%d links)", r.Source, len(r.Links))
			all = append(all, r.Links...)
		} else if r.Err != nil || len(r.Links) == 0 {
			failSources++
			// Try cached data as fallback.
			if cached, ok := fetch.LoadFromCache(r.Source); ok {
				if r.Err != nil {
					logx.Warnf("source failed %s: %v (using %d cached links)", r.Source, r.Err, len(cached))
				} else {
					logx.Warnf("source returned no links %s (using %d cached links)", r.Source, len(cached))
				}
				totalLinks += len(cached)
				all = append(all, cached...)
			} else {
				if r.Err != nil {
					logx.Warnf("source failed %s: %v (no cache)", r.Source, r.Err)
				} else {
					logx.Warnf("source returned no links %s (no cache)", r.Source)
				}
			}
		}
		if health != nil {
			health.Observe(r.Source, len(r.Links), r.Err, time.Now())
		}
	}

	if len(all) == 0 {
		return fmt.Errorf("no links fetched from any source")
	}

	cands := parser.Normalize(all, cfg.OnlySchemes, 0) // No limit for fetch-only.
	writeAllConfigs(cands)

	// Write source report.
	if health != nil {
		stats := health.Snapshot()
		if err := writeSourceReport(stats); err != nil {
			logx.Warnf("failed to write source report: %v", err)
		}
	}

	logx.Infof("fetch complete: %d/%d sources ok, %d total links, %d unique configs",
		okSources, len(roundSources), totalLinks, len(cands))
	logx.Infof("configs written to output/all_configs.txt")
	logx.Infof("cache stored under FIVI_CACHE_DIR or ~/.fivi/cache/sources/")
	return nil
}

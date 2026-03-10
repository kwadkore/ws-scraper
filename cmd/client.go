package cmd

import (
	"fmt"
	"time"

	"github.com/kwadkore/ws-scraper/fetch"
)

func newFetchClient() (*fetch.Client, error) {
	ttl, err := time.ParseDuration(cacheTTL)
	if err != nil {
		return nil, fmt.Errorf("invalid cache ttl: %w", err)
	}

	opts := []fetch.Option{
		fetch.WithRequestsPerSecond(requestsPerSec),
		fetch.WithBurst(burst),
		fetch.WithNetworkConcurrency(netConcurrency),
		fetch.WithMaxRetries(maxRetries),
		fetch.WithRespectRobots(respectRobots),
	}

	if proxyURL != "" {
		opts = append(opts, fetch.WithProxyURL(proxyURL))
	}
	if cacheDir != "" {
		opts = append(opts, fetch.WithCache(cacheDir, ttl))
	}

	return fetch.NewClient(opts...)
}

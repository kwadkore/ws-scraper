package fetch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"
)

const (
	defaultRequestsPerSecond = 1.0
	defaultBurst             = 1
	defaultNetConcurrency    = 1
	defaultMaxRetries        = 4
	defaultRequestTimeout    = 45 * time.Second
	defaultCacheTTL          = 24 * time.Hour
)

var ErrBlocked = errors.New("scraper blocked by remote site")

type Option func(*Client) error

type Client struct {
	httpClient      *http.Client
	transport       *http.Transport
	userAgent       string
	maxRetries      int
	requestTimeout  time.Duration
	cacheDir        string
	cacheTTL        time.Duration
	respectRobots   bool
	networkSlots    chan struct{}
	limiter         *requestLimiter
	requestsPerSec  float64
	burst           int
	randomMu        sync.Mutex
	random          *rand.Rand
	robotsMu        sync.Mutex
	robotsByHost    map[string]robotsPolicy
	promoListingsMu sync.Mutex
	promoListings   map[SiteLanguage]map[string]promoListingEntry
	productPagesMu  sync.Mutex
	productPages    map[string]resolvedExpansion
	logger          *slog.Logger
}

type cacheEntry struct {
	StatusCode int         `json:"statusCode"`
	Header     http.Header `json:"header"`
	Body       []byte      `json:"body"`
	SavedAt    time.Time   `json:"savedAt"`
}

type responseData struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	Request    *http.Request
}

type requestOptions struct {
	Method  string
	URL     string
	Referer string
	Form    url.Values
}

type robotsPolicy struct {
	crawlDelay    time.Duration
	disallowPaths []string
}

type requestLimiter struct {
	interval time.Duration
	tokens   chan struct{}
	stop     chan struct{}
}

func NewClient(opts ...Option) (*Client, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}

	client := &Client{
		transport:      transport,
		httpClient:     &http.Client{Transport: transport, Jar: jar, Timeout: defaultRequestTimeout},
		userAgent:      defaultUserAgent,
		maxRetries:     defaultMaxRetries,
		requestTimeout: defaultRequestTimeout,
		cacheTTL:       defaultCacheTTL,
		respectRobots:  true,
		networkSlots:   make(chan struct{}, defaultNetConcurrency),
		limiter:        newRequestLimiter(defaultRequestsPerSecond, defaultBurst),
		requestsPerSec: defaultRequestsPerSecond,
		burst:          defaultBurst,
		random:         rand.New(rand.NewSource(time.Now().UnixNano())),
		robotsByHost:   make(map[string]robotsPolicy),
		promoListings:  make(map[SiteLanguage]map[string]promoListingEntry),
		productPages:   make(map[string]resolvedExpansion),
		logger:         slog.Default(),
	}

	for _, opt := range opts {
		if err := opt(client); err != nil {
			client.Close()
			return nil, err
		}
	}

	client.httpClient.Timeout = client.requestTimeout
	if client.logger == nil {
		client.logger = slog.Default()
	}

	return client, nil
}

func (c *Client) Close() {
	if c.limiter != nil {
		c.limiter.stopOnce()
	}
	if c.transport != nil {
		c.transport.CloseIdleConnections()
	}
}

func WithRequestsPerSecond(rps float64) Option {
	return func(c *Client) error {
		if rps <= 0 {
			return fmt.Errorf("requests per second must be > 0")
		}
		c.requestsPerSec = rps
		if c.limiter != nil {
			c.limiter.stopOnce()
		}
		c.limiter = newRequestLimiter(rps, c.burst)
		return nil
	}
}

func WithBurst(burst int) Option {
	return func(c *Client) error {
		if burst <= 0 {
			return fmt.Errorf("burst must be > 0")
		}
		c.burst = burst
		if c.limiter != nil {
			c.limiter.stopOnce()
		}
		c.limiter = newRequestLimiter(c.requestsPerSec, burst)
		return nil
	}
}

func WithNetworkConcurrency(concurrency int) Option {
	return func(c *Client) error {
		if concurrency <= 0 {
			return fmt.Errorf("network concurrency must be > 0")
		}
		c.networkSlots = make(chan struct{}, concurrency)
		return nil
	}
}

func WithMaxRetries(maxRetries int) Option {
	return func(c *Client) error {
		if maxRetries < 0 {
			return fmt.Errorf("max retries must be >= 0")
		}
		c.maxRetries = maxRetries
		return nil
	}
}

func WithProxyURL(raw string) Option {
	return func(c *Client) error {
		if raw == "" {
			return nil
		}
		parsed, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("parse proxy url: %w", err)
		}
		c.transport.Proxy = http.ProxyURL(parsed)
		return nil
	}
}

func WithCache(dir string, ttl time.Duration) Option {
	return func(c *Client) error {
		if dir == "" {
			c.cacheDir = ""
			return nil
		}
		if ttl <= 0 {
			ttl = defaultCacheTTL
		}
		c.cacheDir = dir
		c.cacheTTL = ttl
		return nil
	}
}

func WithUserAgent(userAgent string) Option {
	return func(c *Client) error {
		if strings.TrimSpace(userAgent) == "" {
			return fmt.Errorf("user agent must not be empty")
		}
		c.userAgent = userAgent
		return nil
	}
}

func WithRespectRobots(enabled bool) Option {
	return func(c *Client) error {
		c.respectRobots = enabled
		return nil
	}
}

func WithRequestTimeout(timeout time.Duration) Option {
	return func(c *Client) error {
		if timeout <= 0 {
			return fmt.Errorf("request timeout must be > 0")
		}
		c.requestTimeout = timeout
		return nil
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(c *Client) error {
		c.logger = logger
		return nil
	}
}

func newRequestLimiter(rps float64, burst int) *requestLimiter {
	if burst <= 0 {
		burst = defaultBurst
	}
	interval := time.Duration(float64(time.Second) / math.Max(rps, 0.000001))
	rl := &requestLimiter{
		interval: interval,
		tokens:   make(chan struct{}, burst),
		stop:     make(chan struct{}),
	}
	for i := 0; i < burst; i++ {
		rl.tokens <- struct{}{}
	}
	go rl.refill()
	return rl
}

func (r *requestLimiter) refill() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			select {
			case r.tokens <- struct{}{}:
			default:
			}
		}
	}
}

func (r *requestLimiter) wait(ctx context.Context) error {
	if r == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.tokens:
		return nil
	}
}

func (r *requestLimiter) stopOnce() {
	select {
	case <-r.stop:
	default:
		close(r.stop)
	}
}

func (c *Client) withRequestSlot(ctx context.Context, fn func() error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.networkSlots <- struct{}{}:
	}
	defer func() {
		<-c.networkSlots
	}()
	return fn()
}

func (c *Client) request(ctx context.Context, opts requestOptions) (*responseData, error) {
	if opts.Method == "" {
		opts.Method = http.MethodGet
	}
	if err := c.checkRobots(ctx, opts.URL); err != nil {
		return nil, err
	}

	cacheKey := ""
	if c.cacheDir != "" {
		cacheKey = c.cacheKey(opts)
		if cached, ok := c.loadCache(cacheKey); ok {
			req, _ := http.NewRequestWithContext(ctx, opts.Method, opts.URL, nil)
			return &responseData{
				StatusCode: cached.StatusCode,
				Header:     cached.Header,
				Body:       cached.Body,
				Request:    req,
			}, nil
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			if err := c.sleepWithJitter(ctx, c.backoff(attempt)); err != nil {
				return nil, err
			}
		}

		var respData *responseData
		err := c.withRequestSlot(ctx, func() error {
			if err := c.limiter.wait(ctx); err != nil {
				return err
			}
			if err := c.sleepWithJitter(ctx, c.jitterDuration()); err != nil {
				return err
			}

			req, err := c.buildRequest(ctx, opts)
			if err != nil {
				return err
			}
			resp, err := c.httpClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}

			respData = &responseData{
				StatusCode: resp.StatusCode,
				Header:     resp.Header.Clone(),
				Body:       body,
				Request:    req,
			}
			return nil
		})
		if err != nil {
			lastErr = err
			continue
		}

		switch respData.StatusCode {
		case http.StatusOK:
			if cacheKey != "" {
				c.storeCache(cacheKey, cacheEntry{
					StatusCode: respData.StatusCode,
					Header:     respData.Header,
					Body:       respData.Body,
					SavedAt:    time.Now(),
				})
			}
			return respData, nil
		case http.StatusTooManyRequests:
			lastErr = fmt.Errorf("rate limited")
			wait := retryAfter(respData.Header.Get("Retry-After"))
			if wait <= 0 {
				wait = c.backoff(attempt + 2)
			}
			if err := c.sleepWithJitter(ctx, wait); err != nil {
				return nil, err
			}
		case http.StatusForbidden:
			lastErr = fmt.Errorf("%w: status 403 for %s", ErrBlocked, opts.URL)
		default:
			lastErr = fmt.Errorf("unexpected status %d for %s", respData.StatusCode, opts.URL)
		}

		if errors.Is(lastErr, ErrBlocked) {
			break
		}
	}

	return nil, lastErr
}

func (c *Client) buildRequest(ctx context.Context, opts requestOptions) (*http.Request, error) {
	var body io.Reader
	if opts.Method == http.MethodPost && opts.Form != nil {
		body = strings.NewReader(opts.Form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, opts.Method, opts.URL, body)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req, opts.Referer)
	if opts.Method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return req, nil
}

func (c *Client) setHeaders(req *http.Request, referer string) {
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
}

func (c *Client) cacheKey(opts requestOptions) string {
	h := sha256.New()
	h.Write([]byte(opts.Method))
	h.Write([]byte{0})
	h.Write([]byte(opts.URL))
	h.Write([]byte{0})
	h.Write([]byte(opts.Referer))
	h.Write([]byte{0})
	if opts.Form != nil {
		h.Write([]byte(opts.Form.Encode()))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (c *Client) cachePath(key string) string {
	return filepath.Join(c.cacheDir, key+".json")
}

func (c *Client) loadCache(key string) (cacheEntry, bool) {
	path := c.cachePath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return cacheEntry{}, false
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return cacheEntry{}, false
	}
	if c.cacheTTL > 0 && time.Since(entry.SavedAt) > c.cacheTTL {
		return cacheEntry{}, false
	}
	return entry, true
}

func (c *Client) storeCache(key string, entry cacheEntry) {
	if err := os.MkdirAll(c.cacheDir, 0o755); err != nil {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_ = os.WriteFile(c.cachePath(key), data, 0o644)
}

func (c *Client) checkRobots(ctx context.Context, rawURL string) error {
	if !c.respectRobots {
		return nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	hostKey := parsed.Scheme + "://" + parsed.Host

	c.robotsMu.Lock()
	policy, ok := c.robotsByHost[hostKey]
	c.robotsMu.Unlock()
	if !ok {
		policy = c.fetchRobots(ctx, hostKey)
		c.robotsMu.Lock()
		c.robotsByHost[hostKey] = policy
		c.robotsMu.Unlock()
	}
	for _, disallow := range policy.disallowPaths {
		if disallow == "" {
			continue
		}
		if strings.HasPrefix(parsed.Path, disallow) {
			return fmt.Errorf("robots.txt disallows %s", rawURL)
		}
	}
	if len(policy.disallowPaths) == 1 && policy.disallowPaths[0] == "/" {
		return fmt.Errorf("robots.txt disallows %s", rawURL)
	}
	if policy.crawlDelay > 0 {
		if err := c.sleepWithJitter(ctx, policy.crawlDelay); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) fetchRobots(ctx context.Context, hostKey string) robotsPolicy {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hostKey+"/robots.txt", nil)
	if err != nil {
		return robotsPolicy{}
	}
	c.setHeaders(req, "")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return robotsPolicy{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return robotsPolicy{}
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return robotsPolicy{}
	}
	return parseRobots(string(body))
}

func parseRobots(contents string) robotsPolicy {
	policy := robotsPolicy{}
	currentMatches := false

	lines := strings.Split(contents, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		switch key {
		case "user-agent":
			currentMatches = value == "*"
		case "disallow":
			if currentMatches {
				policy.disallowPaths = append(policy.disallowPaths, value)
			}
		case "crawl-delay":
			if currentMatches {
				if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
					policy.crawlDelay = time.Duration(seconds) * time.Second
				}
			}
		}
	}

	return policy
}

func (c *Client) backoff(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	delay := time.Second * time.Duration(1<<min(attempt-1, 5))
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	return delay
}

func (c *Client) jitterDuration() time.Duration {
	c.randomMu.Lock()
	defer c.randomMu.Unlock()
	maxJitter := 150 * time.Millisecond
	return time.Duration(c.random.Int63n(int64(maxJitter) + 1))
}

func (c *Client) sleepWithJitter(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		return time.Until(when)
	}
	return 0
}

func responseToHTTPResponse(data *responseData) *http.Response {
	return &http.Response{
		StatusCode: data.StatusCode,
		Header:     data.Header.Clone(),
		Body:       io.NopCloser(bytes.NewReader(data.Body)),
		Request:    data.Request,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

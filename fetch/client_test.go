package fetch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type clientRoundTripFunc func(*http.Request) (*http.Response, error)

func (f clientRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newHTTPResponse(r *http.Request, status int, headers http.Header, body string) *http.Response {
	if headers == nil {
		headers = make(http.Header)
	}
	return &http.Response{
		StatusCode: status,
		Header:     headers,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    r,
	}
}

func TestClientReusesCookiesAcrossRequests(t *testing.T) {
	var sawCookie atomic.Bool
	client, err := NewClient(WithRespectRobots(false))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()
	client.httpClient.Transport = clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/first":
			headers := make(http.Header)
			headers.Add("Set-Cookie", "sid=abc123; Path=/")
			return newHTTPResponse(r, http.StatusOK, headers, "ok"), nil
		case "/second":
			if cookie, err := r.Cookie("sid"); err == nil && cookie.Value == "abc123" {
				sawCookie.Store(true)
			}
			return newHTTPResponse(r, http.StatusOK, nil, "ok"), nil
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
			return nil, nil
		}
	})

	if _, err := client.request(context.Background(), requestOptions{Method: http.MethodGet, URL: "https://example.test/first"}); err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	if _, err := client.request(context.Background(), requestOptions{Method: http.MethodGet, URL: "https://example.test/second"}); err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	if !sawCookie.Load() {
		t.Fatal("expected second request to reuse cookie jar state")
	}
}

func TestClientRateLimitingAppliesAcrossRequests(t *testing.T) {
	client, err := NewClient(
		WithRespectRobots(false),
		WithRequestsPerSecond(5),
		WithBurst(1),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()
	client.httpClient.Transport = clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return newHTTPResponse(r, http.StatusOK, nil, "ok"), nil
	})

	start := time.Now()
	for i := 0; i < 2; i++ {
		if _, err := client.request(context.Background(), requestOptions{Method: http.MethodGet, URL: "https://example.test/rate"}); err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
	}
	if elapsed := time.Since(start); elapsed < 180*time.Millisecond {
		t.Fatalf("expected requests to be rate limited, got elapsed=%v", elapsed)
	}
}

func TestClientRetriesAfter429(t *testing.T) {
	var attempts atomic.Int32
	client, err := NewClient(
		WithRespectRobots(false),
		WithMaxRetries(2),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()
	client.httpClient.Transport = clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if attempts.Add(1) == 1 {
			headers := make(http.Header)
			headers.Set("Retry-After", "1")
			return newHTTPResponse(r, http.StatusTooManyRequests, headers, ""), nil
		}
		return newHTTPResponse(r, http.StatusOK, nil, "ok"), nil
	})

	start := time.Now()
	if _, err := client.request(context.Background(), requestOptions{Method: http.MethodGet, URL: "https://example.test/retry"}); err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts.Load())
	}
	if elapsed := time.Since(start); elapsed < time.Second {
		t.Fatalf("expected retry-after delay to be honored, got %v", elapsed)
	}
}

func TestClientStopsOnForbidden(t *testing.T) {
	var attempts atomic.Int32
	client, err := NewClient(
		WithRespectRobots(false),
		WithMaxRetries(3),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()
	client.httpClient.Transport = clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		attempts.Add(1)
		return newHTTPResponse(r, http.StatusForbidden, nil, ""), nil
	})

	_, err = client.request(context.Background(), requestOptions{Method: http.MethodGet, URL: "https://example.test/blocked"})
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked, got %v", err)
	}
	if attempts.Load() != 1 {
		t.Fatalf("expected one forbidden attempt before aborting, got %d", attempts.Load())
	}
}

func TestClientCacheHonorsTTL(t *testing.T) {
	var requests atomic.Int32
	cacheDir := t.TempDir()
	client, err := NewClient(
		WithRespectRobots(false),
		WithCache(cacheDir, 100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()
	client.httpClient.Transport = clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		return newHTTPResponse(r, http.StatusOK, nil, "ok"), nil
	})

	opts := requestOptions{Method: http.MethodGet, URL: "https://example.test/cached"}
	if _, err := client.request(context.Background(), opts); err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	if _, err := client.request(context.Background(), opts); err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("expected cached second response, got %d network requests", got)
	}

	time.Sleep(150 * time.Millisecond)

	if _, err := client.request(context.Background(), opts); err != nil {
		t.Fatalf("third request failed: %v", err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("expected cache refresh after ttl, got %d network requests", got)
	}
}

func TestClientHonorsRobotsDisallow(t *testing.T) {
	var privateRequests atomic.Int32
	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()
	client.httpClient.Transport = clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/robots.txt":
			return newHTTPResponse(r, http.StatusOK, nil, "User-agent: *\nDisallow: /private\n"), nil
		case "/private":
			privateRequests.Add(1)
			return newHTTPResponse(r, http.StatusOK, nil, "ok"), nil
		default:
			return newHTTPResponse(r, http.StatusOK, nil, "ok"), nil
		}
	})

	_, err = client.request(context.Background(), requestOptions{Method: http.MethodGet, URL: "https://example.test/private"})
	if err == nil {
		t.Fatal("expected robots disallow error")
	}
	if got := privateRequests.Load(); got != 0 {
		t.Fatalf("expected blocked path not to be requested, got %d", got)
	}
}

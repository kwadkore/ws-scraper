package fetch

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestSetDefaultHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	setDefaultHeaders(req, "https://en.ws-tcg.com/cardlist/")

	if got := req.Header.Get("User-Agent"); got != defaultUserAgent {
		t.Fatalf("User-Agent mismatch: got %q want %q", got, defaultUserAgent)
	}
	if got := req.Header.Get("Accept"); got == "" {
		t.Fatal("Accept header was not set")
	}
	if got := req.Header.Get("Accept-Language"); got == "" {
		t.Fatal("Accept-Language header was not set")
	}
	if got := req.Header.Get("Referer"); got != "https://en.ws-tcg.com/cardlist/" {
		t.Fatalf("Referer mismatch: got %q", got)
	}
}

func TestGetWithHeadersIncludesBrowserUserAgent(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if got := r.Header.Get("User-Agent"); got != defaultUserAgent {
				t.Fatalf("User-Agent mismatch: got %q want %q", got, defaultUserAgent)
			}
			if got := r.Header.Get("Accept"); got == "" {
				t.Fatal("Accept header was not set")
			}
			if got := r.Header.Get("Accept-Language"); got == "" {
				t.Fatal("Accept-Language header was not set")
			}
			if got := r.Header.Get("Referer"); got != "https://en.ws-tcg.com/cardlist/" {
				t.Fatalf("Referer mismatch: got %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	resp, err := getWithHeaders(client, "https://example.com/cardlist/", "https://en.ws-tcg.com/cardlist/")
	if err != nil {
		t.Fatalf("getWithHeaders failed: %v", err)
	}
	resp.Body.Close()
}

func TestPostFormWithHeadersIncludesBrowserUserAgent(t *testing.T) {
	form := url.Values{
		"view":     {"text"},
		"parallel": {"1"},
	}

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost {
				t.Fatalf("method mismatch: got %s want %s", r.Method, http.MethodPost)
			}
			if got := r.Header.Get("User-Agent"); got != defaultUserAgent {
				t.Fatalf("User-Agent mismatch: got %q want %q", got, defaultUserAgent)
			}
			if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
				t.Fatalf("Content-Type mismatch: got %q", got)
			}
			if got := r.Header.Get("Referer"); got != "https://en.ws-tcg.com/cardlist/" {
				t.Fatalf("Referer mismatch: got %q", got)
			}

			b, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed reading body: %v", err)
			}
			if got := string(b); got != form.Encode() {
				t.Fatalf("body mismatch: got %q want %q", got, form.Encode())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	resp, err := postFormWithHeaders(client, "https://example.com/cardlist/searchresults/?page=1", form, "https://en.ws-tcg.com/cardlist/")
	if err != nil {
		t.Fatalf("postFormWithHeaders failed: %v", err)
	}
	resp.Body.Close()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

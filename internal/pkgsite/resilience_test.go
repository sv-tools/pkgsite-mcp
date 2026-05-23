package pkgsite

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// countingServer replies with status/body for the first failN calls, then 200
// with okBody, recording how many requests it received.
func countingServer(t *testing.T, failN int, status int, okBody string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if int(calls.Add(1)) <= failN {
			w.WriteHeader(status)
			return
		}
		_, _ = w.Write([]byte(okBody))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestRetryRecoversFromTransientStatus(t *testing.T) {
	srv, calls := countingServer(t, 2, http.StatusServiceUnavailable, `{"path":"m","version":"v1.0.0"}`)
	c, err := New(srv.URL, WithRetry(3, time.Millisecond))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	mod, err := c.GetModule(context.Background(), "m", "", ModuleOptions{})
	if err != nil {
		t.Fatalf("GetModule: %v", err)
	}
	if mod.Version != "v1.0.0" {
		t.Errorf("version = %q, want v1.0.0", mod.Version)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("calls = %d, want 3 (2 failures + 1 success)", got)
	}
}

func TestRetryGivesUpAfterMax(t *testing.T) {
	srv, calls := countingServer(t, 100, http.StatusServiceUnavailable, "")
	c, _ := New(srv.URL, WithRetry(2, time.Millisecond))

	_, err := c.GetModule(context.Background(), "m", "", ModuleOptions{})
	var aerr *APIError
	if !errors.As(err, &aerr) || aerr.Code != http.StatusServiceUnavailable {
		t.Fatalf("err = %v, want *APIError with code 503", err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("calls = %d, want 3 (1 initial + 2 retries)", got)
	}
}

func TestNoRetryByDefault(t *testing.T) {
	srv, calls := countingServer(t, 100, http.StatusServiceUnavailable, "")
	c, _ := New(srv.URL) // retries disabled unless WithRetry is set

	if _, err := c.GetModule(context.Background(), "m", "", ModuleOptions{}); err == nil {
		t.Fatal("expected error")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("calls = %d, want 1 (no retries by default)", got)
	}
}

func TestNonRetryableStatusNotRetried(t *testing.T) {
	srv, calls := countingServer(t, 100, http.StatusNotFound, "")
	c, _ := New(srv.URL, WithRetry(3, time.Millisecond))

	if _, err := c.GetModule(context.Background(), "m", "", ModuleOptions{}); err == nil {
		t.Fatal("expected error")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("calls = %d, want 1 (404 must not be retried)", got)
	}
}

func TestCacheServesRepeatRequests(t *testing.T) {
	srv, calls := countingServer(t, 0, 0, `{"path":"m","version":"v1.0.0"}`)
	c, _ := New(srv.URL, WithCache(time.Minute, 8))

	for i := 0; i < 3; i++ {
		mod, err := c.GetModule(context.Background(), "m", "", ModuleOptions{})
		if err != nil {
			t.Fatalf("GetModule #%d: %v", i, err)
		}
		if mod.Version != "v1.0.0" {
			t.Errorf("version = %q, want v1.0.0", mod.Version)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("upstream calls = %d, want 1 (subsequent calls cached)", got)
	}
}

func TestCacheKeyedByURL(t *testing.T) {
	srv, calls := countingServer(t, 0, 0, `{"path":"m"}`)
	c, _ := New(srv.URL, WithCache(time.Minute, 8))

	_, _ = c.GetModule(context.Background(), "a", "", ModuleOptions{})
	_, _ = c.GetModule(context.Background(), "b", "", ModuleOptions{})
	if got := calls.Load(); got != 2 {
		t.Errorf("calls = %d, want 2 (distinct paths are distinct keys)", got)
	}
}

func TestParseRetryAfter(t *testing.T) {
	if d := parseRetryAfter("2"); d != 2*time.Second {
		t.Errorf("seconds: got %v, want 2s", d)
	}
	if d := parseRetryAfter(""); d != 0 {
		t.Errorf("empty: got %v, want 0", d)
	}
	if d := parseRetryAfter("garbage"); d != 0 {
		t.Errorf("garbage: got %v, want 0", d)
	}
	if d := parseRetryAfter("Mon, 02 Jan 2006 15:04:05 GMT"); d != 0 {
		t.Errorf("past date: got %v, want 0", d)
	}
}

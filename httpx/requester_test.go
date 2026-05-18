package httpx

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRequesterAppliesAuthAndLogsProgress(t *testing.T) {
	oldLogEvery := defaultLogEvery
	SetDefaultLogEvery(2)
	defer SetDefaultLogEvery(oldLogEvery)

	var authHeader string
	requester := &Requester{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			authHeader = r.Header.Get("Authorization")
			return &http.Response{StatusCode: 200, Status: "200 OK", Body: io.NopCloser(strings.NewReader("ok"))}, nil
		})},
		TargetType: "jira-dc",
		TargetName: "prod",
	}

	var buf bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(oldLogger)

	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("GET", "http://example.com/rest/api/2/search?startAt=0", nil)
		req.Header.Set("Authorization", "Bearer token")
		if _, err := requester.Do(req); err != nil {
			t.Fatalf("Do() failed: %v", err)
		}
	}

	if authHeader != "Bearer token" {
		t.Fatalf("expected auth header to be set, got %q", authHeader)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 progress log line, got %d", len(lines))
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &event); err != nil {
		t.Fatalf("failed to decode progress event: %v", err)
	}
	if event["event"] != "target.http.progress" {
		t.Fatalf("unexpected event: %v", event["event"])
	}
	if event["requests_total"].(float64) != 2 {
		t.Fatalf("expected requests_total=2, got %v", event["requests_total"])
	}
	if event["last_path"] != "/rest/api/2/search" {
		t.Fatalf("unexpected last_path: %v", event["last_path"])
	}
}

func TestRequesterThrottle(t *testing.T) {
	requester := &Requester{
		HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Status: "200 OK", Body: http.NoBody}, nil
		})},
		Throttle: 100 * time.Millisecond,
	}

	start := time.Now()
	for i := 0; i < 5; i++ {
		req, _ := http.NewRequest("GET", "http://example.com/test", nil)
		if _, err := requester.Do(req); err != nil {
			t.Fatalf("Do() failed: %v", err)
		}
	}

	if elapsed := time.Since(start); elapsed < 400*time.Millisecond {
		t.Fatalf("elapsed %v, want >= 400ms", elapsed)
	}
}

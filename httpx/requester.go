package httpx

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

var defaultLogEvery int

func SetDefaultLogEvery(n int) {
	defaultLogEvery = n
}

type Requester struct {
	HTTPClient    *http.Client
	Throttle      time.Duration
	ResponseError func(*http.Request, *http.Response) error
	TargetType    string
	TargetName    string

	mu            sync.Mutex
	lastRequest   time.Time
	requests      int
	failures      int
	durationTotal time.Duration
}

func (r *Requester) Do(req *http.Request) (*http.Response, error) {
	r.wait()

	start := time.Now()
	resp, err := r.HTTPClient.Do(req)
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}

	if err == nil && (status < 200 || status >= 300) {
		if r.ResponseError != nil {
			err = r.ResponseError(req, resp)
		} else {
			err = fmt.Errorf("request to %s failed with status %s", req.URL.String(), resp.Status)
		}
	}

	r.record(req, status, err, time.Since(start))
	return resp, err
}

func (r *Requester) wait() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Throttle > 0 {
		if elapsed := time.Since(r.lastRequest); elapsed < r.Throttle {
			time.Sleep(r.Throttle - elapsed)
		}
	}
	r.lastRequest = time.Now()
}

func (r *Requester) record(req *http.Request, status int, err error, duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.requests++
	if err != nil {
		r.failures++
	}
	r.durationTotal += duration

	if defaultLogEvery > 0 && r.requests%defaultLogEvery == 0 {
		slog.Info("target http progress",
			"event", "target.http.progress",
			"target_type", r.TargetType,
			"target_name", r.TargetName,
			"requests_total", r.requests,
			"requests_failed", r.failures,
			"last_method", req.Method,
			"last_path", req.URL.Path,
			"last_status", status,
			"duration_avg_ms", r.durationTotal.Milliseconds()/int64(r.requests),
		)
	}

}

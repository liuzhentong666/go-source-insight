package handlers_test

import (
	"context"
	"go-ai-study/api/fault"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestLivenessAndCoreReadinessDoNotInitializeAI(t *testing.T) {
	h := newHarness()
	h.factoryErr = true
	f := newHTTPFixture(t, h.settings())
	for _, route := range []string{"/health", "/ready"} {
		if got := f.request(t, http.MethodGet, route, "", 0); got.Code != 200 {
			t.Fatalf("%s=%d %s", route, got.Code, got.Body.String())
		}
	}
	h.mu.Lock()
	calls := len(h.opts)
	h.mu.Unlock()
	if calls != 0 {
		t.Fatal("readiness initialized model clients")
	}
	f.repository.mu.Lock()
	f.repository.readyErr = fault.ErrStorage
	f.repository.mu.Unlock()
	if got := f.request(t, http.MethodGet, "/ready", "", 0); got.Code != 503 || !strings.Contains(got.Body.String(), "not_ready") {
		t.Fatalf("unavailable required storage=%d %s", got.Code, got.Body.String())
	}
	if got := f.request(t, http.MethodGet, "/health", "", 0); got.Code != 200 {
		t.Fatal("liveness depended on storage")
	}
	f.repository.mu.Lock()
	f.repository.readyErr = nil
	f.repository.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := f.svc.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if got := f.request(t, http.MethodGet, "/ready", "", 0); got.Code != 503 {
		t.Fatal("stopped instance reported ready")
	}
}

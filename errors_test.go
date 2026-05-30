package vibeproxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestAcquireLease_ReasonPreserved verifies a reason-carrying 409/404 from
// vibe-proxy surfaces as a structured *APIError (with Reason) rather than
// being collapsed into the bare ErrPoolFull/ErrNotFound sentinels by the
// status-code switch in parseAPIError.
func TestAcquireLease_ReasonPreserved(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		wantReason string
	}{
		{"disabled", http.StatusConflict, `{"error":"preferred proxy x is disabled","reason":"proxy_disabled"}`, ReasonProxyDisabled},
		{"unhealthy", http.StatusConflict, `{"error":"preferred proxy x is unhealthy","reason":"proxy_unhealthy"}`, ReasonProxyUnhealthy},
		{"not_found", http.StatusNotFound, `{"error":"preferred proxy x not found","reason":"proxy_not_found"}`, ReasonProxyNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c := NewClient(srv.URL, "k")
			_, err := c.AcquireLease(context.Background(), AcquireLeaseInput{ConsumerID: "c"})
			if err == nil {
				t.Fatal("expected error")
			}
			if got := LeaseRejectionReason(err); got != tc.wantReason {
				t.Fatalf("LeaseRejectionReason = %q, want %q (err: %v)", got, tc.wantReason, err)
			}
		})
	}
}

// TestFindByObservedIP_MissStillNil confirms a reason-less 404 (the "no proxy
// found" miss) still collapses to (nil, nil) — the reorder must not change it.
func TestFindByObservedIP_MissStillNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"no proxy found for ip within window"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	got, err := c.FindProxyByObservedIP(context.Background(), "203.0.113.1", time.Minute)
	if err != nil {
		t.Fatalf("expected (nil,nil), got err: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil proxy on miss, got %v", got)
	}
}

// TestLeaseRejectionReason_NonAPIError returns "" for nil and plain errors.
func TestLeaseRejectionReason_NonAPIError(t *testing.T) {
	if r := LeaseRejectionReason(nil); r != "" {
		t.Fatalf("nil err: want empty, got %q", r)
	}
	if r := LeaseRejectionReason(errors.New("boom")); r != "" {
		t.Fatalf("plain err: want empty, got %q", r)
	}
}

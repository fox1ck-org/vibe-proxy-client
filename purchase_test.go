package vibeproxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

// TestPurchaseForRequest_NeverRetries is the money test. The shared `do` path
// retries a 503 twice; on the buy route that is a second charge at the vendor
// for one account's proxy.
func TestPurchaseForRequest_NeverRetries(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	id := mustUUID(t, "22222222-2222-2222-2222-222222222222")
	if _, err := c.PurchaseForRequest(context.Background(), id, PurchaseInput{Vendor: "proxyline"}); err == nil {
		t.Fatal("expected an error from a 503")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("purchase hit the vendor route %d times, want exactly 1", got)
	}
}

// TestPurchaseForRequest_PayloadAndDecode pins the route, the body and the
// outcome shape.
func TestPurchaseForRequest_PayloadAndDecode(t *testing.T) {
	var gotPath string
	var gotBody PurchaseInput
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		_, _ = w.Write([]byte(`{"proxyIds":["33333333-3333-3333-3333-333333333333"],"count":1,"request":{"id":"22222222-2222-2222-2222-222222222222","status":"fulfilled"}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	id := mustUUID(t, "22222222-2222-2222-2222-222222222222")
	out, err := c.PurchaseForRequest(context.Background(), id, PurchaseInput{
		Vendor: "proxyline", ProxyType: "resident", IPVersion: 4, Country: "PL", Quantity: 1, PeriodDays: 30,
	})
	if err != nil {
		t.Fatalf("purchase: %v", err)
	}
	if want := "/api/v1/requests/22222222-2222-2222-2222-222222222222/purchase"; gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}
	if gotBody.Country != "PL" || gotBody.Quantity != 1 || gotBody.PeriodDays != 30 {
		t.Fatalf("body = %+v", gotBody)
	}
	if out.Count != 1 || len(out.ProxyIDs) != 1 {
		t.Fatalf("outcome = %+v", out)
	}
	if out.Request == nil || out.Request.Status != "fulfilled" {
		t.Fatalf("request = %+v", out.Request)
	}
}

// TestPurchaseOutcome_FulfilledWithNoProxies decodes the outcome vibe-proxy
// really can produce: it skips a proxy it failed to import and fulfils anyway.
// Callers must be able to see that, so it must not decode as an error.
func TestPurchaseOutcome_FulfilledWithNoProxies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"proxyIds":[],"count":0,"request":{"id":"22222222-2222-2222-2222-222222222222","status":"fulfilled"}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	out, err := c.PurchaseForRequest(context.Background(), mustUUID(t, "22222222-2222-2222-2222-222222222222"), PurchaseInput{})
	if err != nil {
		t.Fatalf("purchase: %v", err)
	}
	if out.Count != 0 || len(out.ProxyIDs) != 0 {
		t.Fatalf("outcome = %+v", out)
	}
}

// TestProxiesForRequest_QueriesByLabel pins the only link between a request and
// what it bought.
func TestProxiesForRequest_QueriesByLabel(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"items":[{"id":"33333333-3333-3333-3333-333333333333","name":"proxyline/PL/9","host":"1.2.3.4","status":"enabled","rotationType":"static"}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	got, err := c.ProxiesForRequest(context.Background(), mustUUID(t, "22222222-2222-2222-2222-222222222222"))
	if err != nil {
		t.Fatalf("proxies for request: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d proxies, want 1", len(got))
	}
	if want := "labels=request_id%3D22222222-2222-2222-2222-222222222222&limit=100"; gotQuery != want {
		t.Fatalf("query = %q, want %q", gotQuery, want)
	}
}

// TestClaimRequest_UsesRetryingPath — claim is idempotent server-side, so it
// keeps the shared retrying transport.
func TestClaimRequest_RetriesOn503(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"id":"22222222-2222-2222-2222-222222222222","status":"in_progress"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	out, err := c.ClaimRequest(context.Background(), mustUUID(t, "22222222-2222-2222-2222-222222222222"))
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if out.Status != "in_progress" {
		t.Fatalf("status = %q", out.Status)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("claim made %d calls, want 2 (one retry)", got)
	}
}

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", s, err)
	}
	return id
}

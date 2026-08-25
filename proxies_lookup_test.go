package vibeproxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestListProxies_FiltersAreQueryParams pins the axes the picker sends.
func TestListProxies_FiltersAreQueryParams(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Encode()
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	if _, err := c.ListProxies(context.Background(), ListProxiesInput{
		CountryCode: "pl", Status: "enabled", Limit: 25, Sort: "created_at", Order: "desc",
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	// country_code is upper-cased client-side: vibe-proxy matches it
	// case-insensitively, but a mixed-case value in a log reads as a bug.
	want := "country_code=PL&limit=25&order=desc&sort=created_at&status=enabled"
	if gotQuery != want {
		t.Fatalf("query = %q, want %q", gotQuery, want)
	}
}

// TestListProxies_DecodesLifecycleFields — expiry and effective health are the
// two facts the list shape alone does not carry, and the reason this type
// exists rather than reusing ProxyListItem.
func TestListProxies_DecodesLifecycleFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"id":"33333333-3333-3333-3333-333333333333","name":"proxyline/PL/9","host":"1.2.3.4","status":"enabled","rotationType":"static","expiresAt":"2026-01-01T00:00:00Z","healthStatus":"unknown","labels":{"provider":"proxyline"}}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	got, err := c.ListProxies(context.Background(), ListProxiesInput{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	p := got[0]
	if p.HealthStatus == nil || *p.HealthStatus != "unknown" {
		t.Fatalf("healthStatus = %v", p.HealthStatus)
	}
	if p.Provider() != "proxyline" {
		t.Fatalf("provider = %q", p.Provider())
	}
	if !p.Expired(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("a proxy whose term ran out in January must read as expired in June")
	}
	if p.Expired(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("a proxy with time left must not read as expired")
	}
}

// TestGetProxy_NotFound keeps the sentinel: a missing proxy is a normal answer
// for a caller reconciling a stale id, not a transport failure.
func TestGetProxy_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	_, err := c.GetProxy(context.Background(), mustUUID(t, "33333333-3333-3333-3333-333333333333"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// TestDefaultEndpoint_PrefersHTTP guards the Chromium constraint: an
// authenticated SOCKS5 proxy is accepted and then fails every navigation with
// ERR_NO_SUPPORTED_PROXIES, so HTTP wins even when SOCKS is flagged default.
func TestDefaultEndpoint_PrefersHTTP(t *testing.T) {
	p := Proxy{ProxyListItem: ProxyListItem{Endpoints: []ProxyEndpoint{
		{Protocol: "socks5", Port: 1080, IsDefault: true},
		{Protocol: "http", Port: 8080},
	}}}
	proto, port, ok := DefaultEndpoint(p)
	if !ok || proto != "http" || port != 8080 {
		t.Fatalf("got %q:%d ok=%v, want http:8080", proto, port, ok)
	}
}

// TestDefaultEndpoint_FallsBackToSocks — a SOCKS-only proxy is still better
// than no endpoint at all; the caller decides what to do with it.
func TestDefaultEndpoint_FallsBackToSocks(t *testing.T) {
	p := Proxy{ProxyListItem: ProxyListItem{Endpoints: []ProxyEndpoint{
		{Protocol: "socks5", Port: 1080},
	}}}
	proto, port, ok := DefaultEndpoint(p)
	if !ok || proto != "socks5" || port != 1080 {
		t.Fatalf("got %q:%d ok=%v", proto, port, ok)
	}
}

// TestDefaultEndpoint_NoEndpoints must not claim port 0 is usable.
func TestDefaultEndpoint_NoEndpoints(t *testing.T) {
	if _, _, ok := DefaultEndpoint(Proxy{}); ok {
		t.Fatal("a proxy with no endpoints must report ok=false")
	}
}

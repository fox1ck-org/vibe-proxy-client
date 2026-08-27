package vibeproxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

const renewProxyID = "22222222-2222-2222-2222-222222222222"

// A renewal posts the term and comes back with the proxy carrying its new
// expiry — which is what lets a caller show "ещё 30 дней" without a second GET.
func TestRenewProxy_PayloadAndDecode(t *testing.T) {
	var gotBody RenewProxyInput
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"` + renewProxyID + `","name":"co-1","host":"154.21.8.9",` +
			`"status":"enabled","rotationType":"static","expiresAt":"2026-09-26T00:00:00Z",` +
			`"externalIp":"154.21.8.9","healthStatus":"healthy"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	got, err := c.RenewProxy(context.Background(), uuid.MustParse(renewProxyID), 30)
	if err != nil {
		t.Fatalf("RenewProxy: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/proxies/"+renewProxyID+"/renew" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}
	if gotBody.PeriodDays != 30 {
		t.Fatalf("periodDays = %d, want 30", gotBody.PeriodDays)
	}
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expiresAt = %v, want the renewed term", got.ExpiresAt)
	}
	// The point of renewing rather than replacing: the address survives.
	if got.ExternalIP == nil || *got.ExternalIP != "154.21.8.9" {
		t.Fatalf("externalIp = %v, want the same exit ip", got.ExternalIP)
	}
}

// The two refusals must arrive as reasons, not as prose. A caller offers "buy a
// replacement" on one and nothing at all on the other, so mixing them up either
// spends money or hides the only way forward.
func TestRenewProxy_RejectionReasons(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantReason string
	}{
		{
			name:       "vendor refused",
			status:     http.StatusConflict,
			body:       `{"error":"proxyline did not renew 88213","reason":"renew_refused"}`,
			wantReason: ReasonRenewRefused,
		},
		{
			name:       "nothing to renew",
			status:     http.StatusBadRequest,
			body:       `{"error":"this proxy cannot be renewed: it was not bought through a provider","reason":"renew_unsupported"}`,
			wantReason: ReasonRenewUnsupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := NewClient(srv.URL, "k")
			_, err := c.RenewProxy(context.Background(), uuid.MustParse(renewProxyID), 0)
			if err == nil {
				t.Fatal("expected an error")
			}
			if got := RenewRejectionReason(err); got != tt.wantReason {
				t.Fatalf("RenewRejectionReason = %q, want %q", got, tt.wantReason)
			}
		})
	}
}

// An unreachable provider is not a refusal: it carries no renew reason, so a
// caller keeps retrying instead of buying a second proxy it does not need.
func TestRenewProxy_ProviderOutageCarriesNoReason(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"renew at proxyline: provider error: timeout"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	_, err := c.RenewProxy(context.Background(), uuid.MustParse(renewProxyID), 30)
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := RenewRejectionReason(err); got != "" {
		t.Fatalf("RenewRejectionReason = %q, want none", got)
	}
}

// A lease rejection must never be readable as a renew rejection, or a caller
// that branches on one vocabulary silently acts on the other's codes.
func TestRenewRejectionReason_IgnoresLeaseCodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"none healthy","reason":"no_healthy_proxies"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	_, err := c.RenewProxy(context.Background(), uuid.MustParse(renewProxyID), 30)
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := RenewRejectionReason(err); got != "" {
		t.Fatalf("RenewRejectionReason = %q, want none for a lease code", got)
	}
	if got := LeaseRejectionReason(err); got != ReasonNoHealthyProxies {
		t.Fatalf("LeaseRejectionReason = %q, want the lease code", got)
	}
}

// The on-demand check reports what the probe saw, including whether it saw
// anything conclusive at all.
func TestCheckProxyHealth(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"33333333-3333-3333-3333-333333333333","proxyId":"` + renewProxyID + `",` +
			`"endpointId":"44444444-4444-4444-4444-444444444444","checkType":"full","success":true,` +
			`"conclusive":true,"latencyMs":412,"externalIp":"154.21.8.9","checkedAt":"2026-08-27T10:00:00Z"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	got, err := c.CheckProxyHealth(context.Background(), uuid.MustParse(renewProxyID))
	if err != nil {
		t.Fatalf("CheckProxyHealth: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/proxies/"+renewProxyID+"/health/check" {
		t.Fatalf("request = %s %s", gotMethod, gotPath)
	}
	if !got.Success || !got.Conclusive {
		t.Fatalf("check = %+v, want a conclusive success", got)
	}
	if got.LatencyMs == nil || *got.LatencyMs != 412 {
		t.Fatalf("latencyMs = %v", got.LatencyMs)
	}
}

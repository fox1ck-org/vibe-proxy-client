package vibeproxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// The exclusions travel under the server's names, and the exit block comes back
// typed — the two halves of asking for an exit without repeats.
func TestAcquireSendsExclusionsAndReadsTheExit(t *testing.T) {
	prev := uuid.New()
	var sent map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&sent)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"lease":{},"connection":{"host":"h","port":1,"protocol":"socks5","externalIp":"198.51.100.2"},
			"proxy":{"externalIp":"198.51.100.2","externalIpObservedAt":"2026-10-08T10:00:00Z","rotationType":"rotating"}}`))
	}))
	defer srv.Close()

	resp, err := NewClient(srv.URL, "").AcquireLease(context.Background(), AcquireLeaseInput{
		PoolID: uuid.New(), ConsumerID: "phone-1",
		ExcludeProxyIDs: []uuid.UUID{prev}, AvoidExternalIPs: []string{"198.51.100.1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ids, _ := sent["excludeProxyIds"].([]any); len(ids) != 1 || ids[0] != prev.String() {
		t.Errorf("excludeProxyIds = %v", sent["excludeProxyIds"])
	}
	if ips, _ := sent["avoidExternalIps"].([]any); len(ips) != 1 || ips[0] != "198.51.100.1" {
		t.Errorf("avoidExternalIps = %v", sent["avoidExternalIps"])
	}
	if resp.Proxy.ExternalIP == nil || *resp.Proxy.ExternalIP != "198.51.100.2" ||
		resp.Proxy.ExternalIPObservedAt == nil || resp.Proxy.RotationType != "rotating" {
		t.Errorf("exit = %+v", resp.Proxy)
	}
}

func TestNoOtherProxiesIsAReason(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"every matching proxy is excluded","reason":"no_other_proxies"}`))
	}))
	defer srv.Close()
	_, err := NewClient(srv.URL, "").AcquireLease(context.Background(), AcquireLeaseInput{
		PoolID: uuid.New(), ConsumerID: "phone-1", AvoidExternalIPs: []string{"198.51.100.1"},
	})
	if got := LeaseRejectionReason(err); got != ReasonNoOtherProxies {
		t.Fatalf("reason = %q, want %q (err %v)", got, ReasonNoOtherProxies, err)
	}
	if LeaseRejectionNeedsOperator(err) {
		t.Error("no_other_proxies is transient, not an operator matter")
	}
}

package vibeproxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

const zProxyID = "208b3b5d-869d-49bd-881b-732b8bdea709"

func TestCheckExitIP_RequestAndDecode(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		_, _ = fmt.Fprintf(w, `{"ip":"46.133.1.2","consistent":true,"match":"network",
			"ipNetwork":{"asn":21497,"isp":"PrJSC VF UKRAINE","countryCode":"UA","source":"ip-api.com","resolvedAt":"2026-09-15T10:00:00Z"},
			"proxy":{"id":%q,"name":"Z-Proxy/UA/1","host":"connect-ua.z-proxy.com","status":"enabled","rotationType":"rotating","connectionType":"mobile","asn":21497,"expired":false,"leasability":"ok","createdAt":"2026-09-01T00:00:00Z"}}`, zProxyID)
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k").CheckExitIP(context.Background(), uuid.MustParse(zProxyID), " 46.133.1.2 ", 10*time.Minute)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if gotPath != "/api/v1/proxies/"+zProxyID+"/exit-ip-match" || gotQuery != "ip=46.133.1.2&within=10m0s" {
		t.Fatalf("path=%q query=%q", gotPath, gotQuery)
	}
	if !got.Consistent || got.Match != MatchNetwork || got.MismatchReason != "" {
		t.Fatalf("got %+v", got)
	}
	if got.IPNetwork == nil || got.IPNetwork.ASN == nil || *got.IPNetwork.ASN != 21497 || got.IPNetwork.Source != "ip-api.com" {
		t.Fatalf("ipNetwork = %+v", got.IPNetwork)
	}
	if got.Proxy.ID.String() != zProxyID || got.Proxy.Leasability != LeasabilityOK {
		t.Fatalf("proxy = %+v", got.Proxy)
	}
}

func TestCheckExitIP_MismatchOnOtherProxy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ip":"46.133.1.2","consistent":false,"match":"none","mismatchReason":"ip_observed_on_other_proxy",
			"observedOn":{"id":"11111111-1111-1111-1111-111111111111","name":"Z-Proxy/UA/2"},
			"proxy":{"id":"` + zProxyID + `","name":"Z-Proxy/UA/1"}}`))
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "k").CheckExitIP(context.Background(), uuid.MustParse(zProxyID), "46.133.1.2", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if got.Consistent || got.Match != MatchNone || got.MismatchReason != MismatchIPObservedOnOtherProxy {
		t.Fatalf("got %+v", got)
	}
	if got.ObservedOn == nil || got.ObservedOn.Name != "Z-Proxy/UA/2" {
		t.Fatalf("observedOn = %+v", got.ObservedOn)
	}
}

func TestCheckExitIP_ValidatesLocally(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls++ }))
	defer srv.Close()
	c := NewClient(srv.URL, "k")
	id := uuid.MustParse(zProxyID)

	for name, call := range map[string]func() error{
		"nil id":   func() error { _, err := c.CheckExitIP(context.Background(), uuid.Nil, "1.2.3.4", time.Minute); return err },
		"empty ip": func() error { _, err := c.CheckExitIP(context.Background(), id, "  ", time.Minute); return err },
		"zero win": func() error { _, err := c.CheckExitIP(context.Background(), id, "1.2.3.4", 0); return err },
	} {
		if err := call(); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrInvalidInput", name, err)
		}
	}
	if calls != 0 {
		t.Fatalf("%d requests sent for input the server is guaranteed to refuse", calls)
	}
}

func TestCheckExitIP_ErrorReasons(t *testing.T) {
	cases := []struct {
		name          string
		status        int
		body          string
		wantReason    string
		needsOperator bool
	}{
		{"pinned proxy gone", http.StatusNotFound, `{"error":"proxy not found","reason":"proxy_not_found"}`, ReasonProxyNotFound, true},
		{"network lookup down", http.StatusServiceUnavailable, `{"error":"ip network lookup failed","reason":"network_lookup_unavailable"}`, ReasonNetworkLookupUnavailable, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			_, err := NewClient(srv.URL, "k").CheckExitIP(context.Background(), uuid.MustParse(zProxyID), "1.2.3.4", time.Minute)
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != tc.status {
				t.Fatalf("err = %#v, want *APIError %d", err, tc.status)
			}
			if got := LeaseRejectionReason(err); got != tc.wantReason {
				t.Fatalf("reason = %q, want %q", got, tc.wantReason)
			}
			if LeaseRejectionNeedsOperator(err) != tc.needsOperator {
				t.Fatalf("needsOperator = %v", !tc.needsOperator)
			}
		})
	}
}

func TestLeaseRejectionNeedsOperator(t *testing.T) {
	for reason, want := range map[string]bool{
		ReasonProxyExpired:             true,
		ReasonProxyDisabled:            true,
		ReasonProxyNotFound:            true,
		ReasonProxyUnhealthy:           false,
		ReasonNoHealthyProxies:         false,
		ReasonNoMatchingProxies:        false,
		ReasonNetworkLookupUnavailable: false,
		"":                             false,
	} {
		err := &APIError{StatusCode: 409, Reason: reason}
		if got := LeaseRejectionNeedsOperator(err); got != want {
			t.Errorf("%q: got %v, want %v", reason, got, want)
		}
	}
	if LeaseRejectionNeedsOperator(errors.New("dial tcp: timeout")) {
		t.Error("a transport error is transient")
	}
	if LeaseRejectionNeedsOperator(nil) {
		t.Error("nil is not a rejection")
	}
}

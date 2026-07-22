package vibeproxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestCreateProxy_PayloadAndDecode verifies CreateProxy POSTs the nested
// endpoint/credential/label body to /api/v1/proxies and decodes the created
// proxy from the 201 response.
func TestCreateProxy_PayloadAndDecode(t *testing.T) {
	var gotBody CreateProxyInput
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"11111111-1111-1111-1111-111111111111","name":"agency-1","host":"184.174.21.221","status":"enabled","rotationType":"static"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	exp := time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)
	us := "US"
	got, err := c.CreateProxy(context.Background(), CreateProxyInput{
		Name:         "agency-1",
		Host:         "184.174.21.221",
		RotationType: RotationStatic,
		CountryCode:  &us,
		ExpiresAt:    &exp,
		Endpoints:    []CreateEndpointInput{{Protocol: ProtocolHTTP, Port: 12324, IsDefault: true}},
		Credentials:  []CreateCredentialInput{{AuthMethod: AuthMethodUserPass, Username: "14a5963c26e40", Password: "d6fc0fc931", IsPrimary: true}},
		Labels:       map[string]string{"source": "vibe-accounts", "provider": "AgencyX"},
	})
	if err != nil {
		t.Fatalf("CreateProxy: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/proxies" {
		t.Fatalf("request = %s %s, want POST /api/v1/proxies", gotMethod, gotPath)
	}
	if got.ID.String() != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("decoded id = %s", got.ID)
	}
	if len(gotBody.Endpoints) != 1 || gotBody.Endpoints[0].Port != 12324 {
		t.Fatalf("endpoints not serialized: %+v", gotBody.Endpoints)
	}
	if len(gotBody.Credentials) != 1 || gotBody.Credentials[0].Password != "d6fc0fc931" {
		t.Fatalf("credentials not serialized: %+v", gotBody.Credentials)
	}
	if gotBody.CountryCode == nil || *gotBody.CountryCode != "US" {
		t.Fatalf("countryCode not serialized: %+v", gotBody.CountryCode)
	}
	if gotBody.Labels["source"] != "vibe-accounts" {
		t.Fatalf("labels not serialized: %+v", gotBody.Labels)
	}
}

// TestCreateProxy_ErrorSurfaced confirms a 4xx surfaces as an error, not a
// zero-value proxy.
func TestCreateProxy_ErrorSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"host is required"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	if _, err := c.CreateProxy(context.Background(), CreateProxyInput{}); err == nil {
		t.Fatal("expected error on 400")
	}
}

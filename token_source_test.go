package vibeproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"
)

func TestTokenSourceReplacesTheAPIKey(t *testing.T) {
	var auth, key string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, key = r.Header.Get("Authorization"), r.Header.Get("X-API-Key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"total":0}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "", WithTokenSource(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "sa-token"})))
	resp, err := c.do(context.Background(), http.MethodGet, "/api/v1/proxies", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if auth != "Bearer sa-token" || key != "" {
		t.Fatalf("Authorization %q, X-API-Key %q", auth, key)
	}

	c = NewClient(srv.URL, "k")
	resp, err = c.do(context.Background(), http.MethodGet, "/api/v1/proxies", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if auth != "" || key != "k" {
		t.Fatalf("API key path: Authorization %q, X-API-Key %q", auth, key)
	}
}

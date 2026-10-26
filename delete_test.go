package vibeproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestDeleteProxy(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	for _, tc := range []struct {
		name    string
		status  int
		wantErr bool
	}{
		{"deleted", http.StatusNoContent, false},
		{"already gone", http.StatusNotFound, false},
		{"refused", http.StatusForbidden, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath = r.Method, r.URL.Path
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()
			err := NewClient(srv.URL, "k").DeleteProxy(context.Background(), id)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if gotMethod != http.MethodDelete || gotPath != "/api/v1/proxies/"+id.String() {
				t.Fatalf("%s %s", gotMethod, gotPath)
			}
		})
	}
}

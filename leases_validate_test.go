package vibeproxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// An acquire the server is certain to reject must never leave the process. The
// failure this closes: a caller with an empty consumerId sent it anyway and took
// a 400 on every attempt — 740k of them against one deployment in 30 hours,
// drowning the proxy's request log and hiding the caller behind an opaque status.
func TestAcquireLeaseRejectsDoomedInputWithoutCallingTheServer(t *testing.T) {
	pool := uuid.New()
	cases := []struct {
		name  string
		input AcquireLeaseInput
		want  string
	}{
		{
			name:  "empty consumerId",
			input: AcquireLeaseInput{PoolID: pool, ConsumerID: ""},
			want:  "consumerId is required",
		},
		{
			name:  "blank consumerId",
			input: AcquireLeaseInput{PoolID: pool, ConsumerID: "   "},
			want:  "consumerId is required",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			called := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusBadRequest)
			}))
			defer srv.Close()

			_, err := NewClient(srv.URL, "").AcquireLease(context.Background(), c.input)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("error must be typed ErrInvalidInput, got %v", err)
			}
			if got := err.Error(); !strings.Contains(got, c.want) {
				t.Errorf("error %q does not say %q", got, c.want)
			}
			if called {
				t.Error("a doomed request must not reach the server")
			}
		})
	}
}

// A well-formed acquire must still go out untouched — the guard must not become
// a second, stricter validator that rejects what the server would accept.
func TestAcquireLeasePassesValidInputThrough(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"lease":{},"connection":{"host":"h","port":1,"protocol":"socks5"}}`))
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "").AcquireLease(context.Background(), AcquireLeaseInput{
		PoolID:     uuid.New(),
		ConsumerID: "acct-1",
		Sticky:     true,
	})
	if err != nil {
		t.Fatalf("valid input must reach the server: %v", err)
	}
	if !called {
		t.Error("server was never called")
	}
}

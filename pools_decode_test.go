package vibeproxy

import (
	"encoding/json"
	"testing"
)

// TestPoolListAcceptsBothShapes pins the asymmetry that broke every
// resolve-pool-by-name: GET /api/v1/pools returns a bare array, while
// /api/v1/proxies returns {"items":[…]}. Decoding only the object shape made
// ListPools fail on every call, and the failure surfaced as a failed proxy
// lease in the consumer rather than as anything about JSON.
func TestPoolListAcceptsBothShapes(t *testing.T) {
	const id = "d04cefb2-4187-4fc2-ae61-546adfa60542"

	for _, tc := range []struct {
		name string
		body string
	}{
		{"bare array — what the server actually returns", `[{"id":"` + id + `","name":"anty"}]`},
		{"object with items — the other list endpoints' shape", `{"items":[{"id":"` + id + `","name":"anty"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got PoolList
			if err := json.Unmarshal([]byte(tc.body), &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(got.Items) != 1 {
				t.Fatalf("got %d pools, want 1", len(got.Items))
			}
			if got.Items[0].Name != "anty" {
				t.Fatalf("got name %q, want %q", got.Items[0].Name, "anty")
			}
		})
	}

	t.Run("garbage still errors", func(t *testing.T) {
		var got PoolList
		if err := json.Unmarshal([]byte(`"not-a-list"`), &got); err == nil {
			t.Fatal("want an error for a non-list body")
		}
	})
}

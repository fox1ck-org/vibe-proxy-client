package vibeproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListStockCharges(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/money/stock-charges" ||
			r.URL.Query().Get("after") != "7" || r.URL.Query().Get("limit") != "100" {
			t.Errorf("request %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"charges":[{"seq":8,"key":"proxy_purchase:p1","proxy_id":"p1",
			"account_id":"48acfe5e-0933-40bf-a4fa-b3d9f672e7f9","amount_minor":177,"currency":"USD",
			"occurred_at":"2026-11-05T12:00:00Z"}],"next":8}`))
	}))
	defer srv.Close()
	page, err := NewClient(srv.URL, "key").ListStockCharges(context.Background(), 7, 100)
	if err != nil {
		t.Fatal(err)
	}
	if page.Next != 8 || len(page.Charges) != 1 {
		t.Fatalf("page %+v", page)
	}
	c := page.Charges[0]
	if c.Seq != 8 || c.Key != "proxy_purchase:p1" || c.AccountID != "48acfe5e-0933-40bf-a4fa-b3d9f672e7f9" ||
		c.AmountMinor != 177 || c.Currency != "USD" || c.OccurredAt.IsZero() {
		t.Fatalf("charge %+v", c)
	}
}

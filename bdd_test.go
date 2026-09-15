package vibeproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

// bddWorld is one scenario: a canned vibe-proxy (httptest) and what the client
// made of its answer.
type bddWorld struct {
	srv       *httptest.Server
	pinned    uuid.UUID
	pinName   string
	status    int
	body      string
	listItems []map[string]any
	lastQuery string

	match   *ExitIPMatch
	proxies []Proxy
	err     error
}

func TestClientBDD(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: func(sc *godog.ScenarioContext) {
			w := &bddWorld{}
			sc.Before(func(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
				*w = bddWorld{pinned: uuid.MustParse(zProxyID), status: http.StatusOK}
				w.srv = httptest.NewServer(http.HandlerFunc(w.serve))
				return ctx, nil
			})
			sc.After(func(ctx context.Context, _ *godog.Scenario, err error) (context.Context, error) {
				w.srv.Close()
				return ctx, nil
			})
			w.register(sc)
		},
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
			Strict:   true,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("BDD suite failed")
	}
}

func (w *bddWorld) serve(rw http.ResponseWriter, r *http.Request) {
	w.lastQuery = r.URL.RawQuery
	if r.URL.Path == "/api/v1/proxies" {
		_ = json.NewEncoder(rw).Encode(map[string]any{"items": w.listItems})
		return
	}
	rw.WriteHeader(w.status)
	_, _ = rw.Write([]byte(w.body))
}

func (w *bddWorld) register(sc *godog.ScenarioContext) {
	sc.Step(`^the pinned proxy "([^"]*)" is a rotating mobile proxy on ASN (\d+)$`, w.pinnedMobile)
	sc.Step(`^vibe-proxy answers the check with match "([^"]*)"$`, w.answerMatch)
	sc.Step(`^vibe-proxy answers the check with match "([^"]*)" and IP ASN (\d+)$`, w.answerMatchASN)
	sc.Step(`^vibe-proxy answers the check with mismatch "([^"]*)" and IP ASN (\d+)$`, w.answerMismatchASN)
	sc.Step(`^vibe-proxy answers the check with mismatch "([^"]*)" on proxy "([^"]*)"$`, w.answerOtherProxy)
	sc.Step(`^vibe-proxy cannot resolve the IP network$`, w.lookupDown)
	sc.Step(`^the pinned proxy does not exist$`, w.proxyGone)
	sc.Step(`^the caller checks exit IP "([^"]*)" against the pinned proxy$`, w.check)
	sc.Step(`^the IP is consistent with match "([^"]*)"$`, w.consistent)
	sc.Step(`^the resolved IP network is ASN (\d+)$`, w.resolvedASN)
	sc.Step(`^the IP is not consistent because "([^"]*)"$`, w.inconsistent)
	sc.Step(`^the other proxy is "([^"]*)"$`, w.otherProxy)
	sc.Step(`^the check fails with reason "([^"]*)"$`, w.failsWith)
	sc.Step(`^the lease fails with reason "([^"]*)"$`, w.failsWith)
	sc.Step(`^the failure needs an operator$`, func() error { return w.needsOperator(true) })
	sc.Step(`^the failure does not need an operator$`, func() error { return w.needsOperator(false) })

	sc.Step(`^vibe-proxy holds proxy "([^"]*)" with status "([^"]*)", expired (true|false) and leasability "([^"]*)"$`, w.holds)
	sc.Step(`^the caller lists proxies$`, func() error { return w.list(nil) })
	sc.Step(`^the caller lists only expired proxies$`, func() error { b := true; return w.list(&b) })
	sc.Step(`^proxy "([^"]*)" has status "([^"]*)"$`, w.hasStatus)
	sc.Step(`^proxy "([^"]*)" is expired with leasability "([^"]*)"$`, func(n, l string) error { return w.lifecycle(n, true, l) })
	sc.Step(`^proxy "([^"]*)" is not expired with leasability "([^"]*)"$`, func(n, l string) error { return w.lifecycle(n, false, l) })
	sc.Step(`^proxy "([^"]*)" needs an operator$`, func(n string) error { return w.proxyNeedsOperator(n, true) })
	sc.Step(`^proxy "([^"]*)" does not need an operator$`, func(n string) error { return w.proxyNeedsOperator(n, false) })
	sc.Step(`^the list request carries "([^"]*)"$`, w.listQuery)
	sc.Step(`^vibe-proxy rejects pinned leases with status (\d+) and reason "([^"]*)"$`, w.rejectLeases)
	sc.Step(`^the caller acquires a lease pinned to the proxy$`, w.acquire)
}

// ── Given ────────────────────────────────────────────────────────────────────

func (w *bddWorld) pinnedMobile(name string, _ int) error {
	w.pinName = name
	return nil
}

func (w *bddWorld) proxyJSON() string {
	return fmt.Sprintf(`{"id":%q,"name":%q,"status":"enabled","rotationType":"rotating","connectionType":"mobile","asn":21497,"expired":false,"leasability":"ok"}`,
		w.pinned, w.pinName)
}

func (w *bddWorld) answerMatch(kind string) error {
	w.body = fmt.Sprintf(`{"ip":"x","consistent":true,"match":%q,"proxy":%s}`, kind, w.proxyJSON())
	return nil
}

func (w *bddWorld) answerMatchASN(kind string, asn int) error {
	w.body = fmt.Sprintf(`{"ip":"x","consistent":true,"match":%q,"ipNetwork":{"asn":%d,"source":"ip-api.com","resolvedAt":"2026-09-15T10:00:00Z"},"proxy":%s}`,
		kind, asn, w.proxyJSON())
	return nil
}

func (w *bddWorld) answerMismatchASN(reason string, asn int) error {
	w.body = fmt.Sprintf(`{"ip":"x","consistent":false,"match":"none","mismatchReason":%q,"ipNetwork":{"asn":%d,"source":"ip-api.com","resolvedAt":"2026-09-15T10:00:00Z"},"proxy":%s}`,
		reason, asn, w.proxyJSON())
	return nil
}

func (w *bddWorld) answerOtherProxy(reason, other string) error {
	w.body = fmt.Sprintf(`{"ip":"x","consistent":false,"match":"none","mismatchReason":%q,"observedOn":{"id":%q,"name":%q},"proxy":%s}`,
		reason, uuid.NewString(), other, w.proxyJSON())
	return nil
}

func (w *bddWorld) lookupDown() error {
	w.status = http.StatusServiceUnavailable
	w.body = `{"error":"ip network lookup failed","reason":"network_lookup_unavailable"}`
	return nil
}

func (w *bddWorld) proxyGone() error {
	w.status = http.StatusNotFound
	w.body = `{"error":"proxy not found","reason":"proxy_not_found"}`
	return nil
}

func (w *bddWorld) holds(name, status, expired, leasability string) error {
	w.listItems = append(w.listItems, map[string]any{
		"id": uuid.NewString(), "name": name, "status": status,
		"expired": expired == "true", "leasability": leasability,
		"expiresAt": time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339),
	})
	return nil
}

func (w *bddWorld) rejectLeases(status int, reason string) error {
	w.status = status
	w.body = fmt.Sprintf(`{"error":"preferred proxy rejected","reason":%q}`, reason)
	return nil
}

// ── When ─────────────────────────────────────────────────────────────────────

func (w *bddWorld) check(ip string) error {
	w.match, w.err = NewClient(w.srv.URL, "k").CheckExitIP(context.Background(), w.pinned, ip, 10*time.Minute)
	return nil
}

func (w *bddWorld) list(expired *bool) error {
	w.proxies, w.err = NewClient(w.srv.URL, "k").ListProxies(context.Background(), ListProxiesInput{Expired: expired})
	return w.err
}

func (w *bddWorld) acquire() error {
	_, w.err = NewClient(w.srv.URL, "k").AcquireLease(context.Background(), AcquireLeaseInput{
		PoolID: uuid.New(), ConsumerID: "fb-account-1", PreferredProxyID: &w.pinned, Sticky: true,
	})
	return nil
}

// ── Then ─────────────────────────────────────────────────────────────────────

func (w *bddWorld) consistent(kind string) error {
	if w.err != nil {
		return w.err
	}
	if !w.match.Consistent || string(w.match.Match) != kind || w.match.MismatchReason != "" {
		return fmt.Errorf("got consistent=%v match=%q reason=%q", w.match.Consistent, w.match.Match, w.match.MismatchReason)
	}
	return nil
}

func (w *bddWorld) resolvedASN(asn int) error {
	if w.match.IPNetwork == nil || w.match.IPNetwork.ASN == nil || *w.match.IPNetwork.ASN != asn {
		return fmt.Errorf("ipNetwork = %+v", w.match.IPNetwork)
	}
	return nil
}

func (w *bddWorld) inconsistent(reason string) error {
	if w.err != nil {
		return w.err
	}
	if w.match.Consistent || w.match.Match != MatchNone || string(w.match.MismatchReason) != reason {
		return fmt.Errorf("got consistent=%v match=%q reason=%q", w.match.Consistent, w.match.Match, w.match.MismatchReason)
	}
	return nil
}

func (w *bddWorld) otherProxy(name string) error {
	if w.match.ObservedOn == nil || w.match.ObservedOn.Name != name {
		return fmt.Errorf("observedOn = %+v", w.match.ObservedOn)
	}
	return nil
}

func (w *bddWorld) failsWith(reason string) error {
	if w.err == nil {
		return fmt.Errorf("expected a failure with reason %q", reason)
	}
	if got := LeaseRejectionReason(w.err); got != reason {
		return fmt.Errorf("reason = %q, want %q (err %v)", got, reason, w.err)
	}
	return nil
}

func (w *bddWorld) needsOperator(want bool) error {
	if got := LeaseRejectionNeedsOperator(w.err); got != want {
		return fmt.Errorf("needs operator = %v, want %v", got, want)
	}
	return nil
}

func (w *bddWorld) find(name string) (*Proxy, error) {
	for i := range w.proxies {
		if w.proxies[i].Name == name {
			return &w.proxies[i], nil
		}
	}
	return nil, fmt.Errorf("proxy %q not in the list", name)
}

func (w *bddWorld) hasStatus(name, status string) error {
	p, err := w.find(name)
	if err != nil {
		return err
	}
	if p.Status != status {
		return fmt.Errorf("status = %q, want %q", p.Status, status)
	}
	return nil
}

func (w *bddWorld) lifecycle(name string, expired bool, leasability string) error {
	p, err := w.find(name)
	if err != nil {
		return err
	}
	if p.Expired != expired || string(p.Leasability) != leasability {
		return fmt.Errorf("expired=%v leasability=%q, want %v/%q", p.Expired, p.Leasability, expired, leasability)
	}
	return nil
}

func (w *bddWorld) proxyNeedsOperator(name string, want bool) error {
	p, err := w.find(name)
	if err != nil {
		return err
	}
	if p.NeedsOperator() != want {
		return fmt.Errorf("NeedsOperator = %v, want %v", !want, want)
	}
	return nil
}

func (w *bddWorld) listQuery(want string) error {
	if !strings.Contains(w.lastQuery, want) {
		return fmt.Errorf("query = %q, want it to carry %q", w.lastQuery, want)
	}
	return nil
}

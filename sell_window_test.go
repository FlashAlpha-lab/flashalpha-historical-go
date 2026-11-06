package flashalphahistorical

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sellWindowBody = `{"symbol":"SPY","session":"2026-10-16","spot":772.255,"quoted_at":"2026-10-16T13:35:00Z","legs_indicative":false,"windows":[{"expiry":"2026-10-16","days_to_expiry":0,"action":"sell","candidates":[{"structure":"short_put","exit":"expiry","legs":[{"side":"sell","quantity":1,"type":"put","strike":769,"delta":-0.19,"bid":0.39,"ask":0.4}],"mid_credit":0.395}]},{"expiry":"2026-10-23","days_to_expiry":7,"action":"skip"},{"expiry":"2026-10-30","days_to_expiry":14,"action":"skip"},{"action":"unavailable"}],"endpoint_version":"2026.09.30","data_as_of":{"node":"f3"},"archive_as_of":{"node":"f3","equity_options_feed":"2026-10-16T13:35:00Z"}}`

func TestSellWindowTypedSendsAtAndBindsEveryField(t *testing.T) {
	var path, query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sellWindowBody))
	}))
	defer srv.Close()

	got, err := NewClientWithURL("KEY", srv.URL).SellWindowTyped(context.Background(), "SPY", "2026-10-16T09:35:00")
	if err != nil {
		t.Fatalf("SellWindowTyped: %v", err)
	}
	if path != "/v1/strategies/sell-window/SPY" || !strings.Contains(query, "at=2026-10-16T09%3A35%3A00") {
		t.Errorf("path = %q query = %q", path, query)
	}
	if len(got.Windows) != 4 || got.Windows[0].Action != "sell" {
		t.Fatalf("windows = %+v", got.Windows)
	}
	trade := got.Windows[0].Candidates[0]
	if trade.MidCredit == nil || *trade.MidCredit != 0.395 || trade.Legs[0].Strike != 769 || trade.Exit != "expiry" {
		t.Errorf("trade = %+v", trade)
	}
	if got.Windows[3].Expiry != nil || got.Spot == nil || got.LegsIndicative == nil || *got.LegsIndicative {
		t.Errorf("optionals = %+v", got)
	}
	if got.ArchiveAsOf == nil || got.ArchiveAsOf.Node != "f3" || got.EndpointVersion != "2026.09.30" {
		t.Errorf("envelope = %q %+v", got.EndpointVersion, got.ArchiveAsOf)
	}
}

func TestSellWindowInvalidAtMapsToTypedError(t *testing.T) {
	srv := mockServer(http.StatusBadRequest, `{"error":"invalid_at","message":"at is in the future."}`)
	defer srv.Close()
	_, err := NewClientWithURL("KEY", srv.URL).SellWindow(context.Background(), "SPY", "2099-01-02T09:35:00")
	var iae *InvalidAtError
	if !errors.As(err, &iae) {
		t.Errorf("err = %T %v", err, err)
	}
}

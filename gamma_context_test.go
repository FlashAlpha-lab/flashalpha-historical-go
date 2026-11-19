package flashalphahistorical

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

const gammaAt = "2026-10-09T15:55:00"

type gammaRequest struct{ path, query string }

// serveGammaFixture serves a real production replay captured on 2026-11-19 (lists
// trimmed) byte for byte, and records the request.
func serveGammaFixture(t *testing.T, status int, name string) (*Client, *gammaRequest) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "gamma_context", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	got := &gammaRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path, got.query = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return NewClientWithURL("KEY", srv.URL), got
}

func TestExpiryImpactSendsAtAndOmitsUnsetOptions(t *testing.T) {
	c, got := serveGammaFixture(t, 200, "expiry_impact_historical.json")
	if _, err := c.ExpiryImpactTyped(context.Background(), "SPY", gammaAt); err != nil {
		t.Fatalf("ExpiryImpactTyped: %v", err)
	}
	if got.path != "/v1/exposure/expiry-impact/SPY" || got.query != "at=2026-10-09T15%3A55%3A00" {
		t.Errorf("path = %q query = %q", got.path, got.query)
	}
}

func TestExpiryImpactSendsExpiryAndLimit(t *testing.T) {
	c, got := serveGammaFixture(t, 200, "expiry_impact_historical.json")
	if _, err := c.ExpiryImpact(context.Background(), "QQQ", "2026-10-09", WithExpiry("2026-10-12"), WithLimit(3)); err != nil {
		t.Fatalf("ExpiryImpact: %v", err)
	}
	if got.path != "/v1/exposure/expiry-impact/QQQ" || got.query != "at=2026-10-09&expiry=2026-10-12&limit=3" {
		t.Errorf("path = %q query = %q", got.path, got.query)
	}
}

func TestExpiryImpactTypedBindsEveryLevel(t *testing.T) {
	c, _ := serveGammaFixture(t, 200, "expiry_impact_historical.json")
	r, err := c.ExpiryImpactTyped(context.Background(), "SPY", gammaAt)
	if err != nil {
		t.Fatalf("ExpiryImpactTyped: %v", err)
	}

	if r.Symbol != "SPY" || r.UnderlyingPrice != 778.405 || r.PricingBasis != "historical_quote_repriced_dated_inputs" || r.DataStatus != "partial" {
		t.Errorf("metadata = %q %v %q %q", r.Symbol, r.UnderlyingPrice, r.PricingBasis, r.DataStatus)
	}
	if len(r.Reasons) != 4 || r.Reasons[0].Code != "historical_model_reconstruction" || r.Reasons[0].ContractCount != nil || r.Reasons[1].ContractCount == nil || *r.Reasons[1].ContractCount != 302 {
		t.Errorf("reasons = %+v", r.Reasons)
	}
	if r.QuoteAt == nil || *r.QuoteAt != "2026-10-09T19:55:00Z" || r.OiAsOf != nil || r.ObservedMaxStrike == nil || *r.ObservedMaxStrike != 1480 {
		t.Errorf("timestamps = %v %v %v", r.QuoteAt, r.OiAsOf, r.ObservedMaxStrike)
	}

	// Both provenance objects plus the version.
	if r.EndpointVersion != "2026.09.30" || r.DataAsOf == nil || r.DataAsOf.Node != "f3" || r.DataAsOf.EquityFeed != nil {
		t.Errorf("data_as_of = %q %+v", r.EndpointVersion, r.DataAsOf)
	}
	if r.ArchiveAsOf == nil || r.ArchiveAsOf.Node != "f3" || r.ArchiveAsOf.OiFeed == nil || *r.ArchiveAsOf.OiFeed != "2026-10-08T20:00:00Z" {
		t.Errorf("archive_as_of = %+v", r.ArchiveAsOf)
	}

	if r.SelectedExpiry != "2026-10-09" || len(r.AvailableExpiries) != 3 || r.Current.ContractCount != 12078 || r.Current.Status != nil {
		t.Errorf("selection = %q %v %+v", r.SelectedExpiry, r.AvailableExpiries, r.Current)
	}
	if r.ExcludingExpiry.Status == nil || *r.ExcludingExpiry.Status != "available" || r.ExcludedExpiry.ContractCount != 100 {
		t.Errorf("excluded/excluding = %+v %+v", r.ExcludedExpiry, r.ExcludingExpiry)
	}

	im := r.Impact
	if im.RemovedGrossSharePct == nil || *im.RemovedGrossSharePct != 27.355976302706107 || im.NetSignChanged == nil || *im.NetSignChanged || im.NetSignAfter != "positive" {
		t.Errorf("impact = %+v", im)
	}
	if len(im.LargestGrossReductions) != 2 || im.LargestGrossReductions[0].Strike != 778 || im.LargestGrossReductions[0].RemainingNetGexUsd != 280859003.66476536 {
		t.Errorf("largest_gross_reductions = %+v", im.LargestGrossReductions)
	}
	if s := im.StrongestRemainingConcentrations; len(s) != 2 || s[1].Strike != 785 || s[1].ExcludedNetGexUsd != -186739.42129722596 {
		t.Errorf("strongest_remaining_concentrations = %+v", s)
	}
	if len(r.Strikes) != 3 || r.Strikes[2].Strike != 60 || r.Strikes[2].ExcludedGrossGexUsd != 0 {
		t.Errorf("strikes = %+v", r.Strikes)
	}
	if len(r.Facts) != 2 || r.Facts[0].SupportingValues["expiry"] != "2026-10-09" || r.Facts[1].SupportingValues["underlying_price"] != 778.405 {
		t.Errorf("facts = %+v", r.Facts)
	}
}

func TestStrikeDependencySendsAtAndExactStrike(t *testing.T) {
	cases := map[float64]string{
		672.5: "at=2026-10-09T15%3A55%3A00&strike=672.5",
		670:   "at=2026-10-09T15%3A55%3A00&strike=670",
		52.25: "at=2026-10-09T15%3A55%3A00&strike=52.25",
	}
	for strike, want := range cases {
		c, got := serveGammaFixture(t, 200, "strike_dependency_historical.json")
		if _, err := c.StrikeDependency(context.Background(), "SPY", gammaAt, strike); err != nil {
			t.Fatalf("StrikeDependency(%v): %v", strike, err)
		}
		if got.path != "/v1/exposure/strike-dependency/SPY" || got.query != want {
			t.Errorf("strike %v: path = %q query = %q, want %q", strike, got.path, got.query, want)
		}
	}
}

func TestStrikeDependencyTypedBindsEveryLevel(t *testing.T) {
	c, _ := serveGammaFixture(t, 200, "strike_dependency_historical.json")
	r, err := c.StrikeDependencyTyped(context.Background(), "SPY", gammaAt, 670)
	if err != nil {
		t.Fatalf("StrikeDependencyTyped: %v", err)
	}

	if r.Strike != 670 || r.ContractCount != 60 || r.NetGexUsd != -95819046.48603098 || r.AnalysisType != "strike_attribution" {
		t.Errorf("top level = %v %d %v %q", r.Strike, r.ContractCount, r.NetGexUsd, r.AnalysisType)
	}
	if r.DominantExpiry == nil || *r.DominantExpiry != "2026-12-18" || r.DominantExpirySharePct == nil || *r.DominantExpirySharePct != 29.844720445178503 {
		t.Errorf("dominant = %v %v", r.DominantExpiry, r.DominantExpirySharePct)
	}
	if r.ArchiveAsOf == nil || r.ArchiveAsOf.Node != "f3" || r.ArchiveAsOf.EquityOptionsFeed == nil || *r.ArchiveAsOf.EquityOptionsFeed != "2026-10-09T19:55:00Z" {
		t.Errorf("archive_as_of = %+v", r.ArchiveAsOf)
	}
	if r.DataAsOf == nil || r.DataAsOf.Node != "f3" || r.EndpointVersion != "2026.09.30" {
		t.Errorf("data_as_of = %q %+v", r.EndpointVersion, r.DataAsOf)
	}

	if len(r.ByExpiry) != 3 {
		t.Fatalf("by_expiry = %+v", r.ByExpiry)
	}
	row := r.ByExpiry[0]
	if row.Expiry != "2026-10-13" || row.ValuationCutoffAt != "2026-10-13T20:00:00Z" || row.LastTradingAt != "2026-10-13T20:15:00Z" || row.CalendarDaysToExpiry != 4 {
		t.Errorf("by_expiry[0] clocks = %+v", row)
	}
	if row.SettlementAt != nil || row.SettlementStatus != "unknown" || row.ShareOfStrikeGrossPct == nil || *row.ShareOfStrikeGrossPct != 0.0026251306222739787 {
		t.Errorf("by_expiry[0] status = %+v", row)
	}
	// GammaTotals fields are promoted onto the row.
	if row.GrossGexUsd != 3035.560157574413 || row.CallGexUsd != 0 || row.ContractCount != 2 || row.Status != nil {
		t.Errorf("by_expiry[0] totals = %+v", row.GammaTotals)
	}
}

func TestGammaContextBelowAlphaIsTierRestricted(t *testing.T) {
	srv := mockServer(http.StatusForbidden, `{"error":"tier_restricted","message":"Requires Alpha plan.","current_plan":"growth","required_plan":"alpha"}`)
	defer srv.Close()
	c := NewClientWithURL("KEY", srv.URL)

	var te *TierRestrictedError
	if _, err := c.ExpiryImpactTyped(context.Background(), "SPY", gammaAt); !errors.As(err, &te) || te.RequiredPlan != "alpha" {
		t.Errorf("ExpiryImpactTyped err = %T %v", err, err)
	}
	if _, err := c.StrikeDependencyTyped(context.Background(), "SPY", gammaAt, 670); !errors.As(err, &te) {
		t.Errorf("StrikeDependencyTyped err = %T %v", err, err)
	}
}

func TestGammaContextUnsupportedSymbolSurfaces422(t *testing.T) {
	c, _ := serveGammaFixture(t, 422, "error_unsupported_symbol.json")
	for name, call := range map[string]func() error{
		"ExpiryImpactTyped": func() error {
			_, err := c.ExpiryImpactTyped(context.Background(), "AAPL", gammaAt)
			return err
		},
		"StrikeDependencyTyped": func() error {
			_, err := c.StrikeDependencyTyped(context.Background(), "AAPL", gammaAt, 200)
			return err
		},
	} {
		var apiErr *APIError
		if err := call(); !errors.As(err, &apiErr) || apiErr.StatusCode != 422 || apiErr.Code != "unsupported_symbol" {
			t.Errorf("%s err = %T %v", name, err, err)
		}
	}
}

func TestStrikeDependencyStrikeNotFoundIsNoData(t *testing.T) {
	c, _ := serveGammaFixture(t, 404, "error_strike_not_found.json")
	_, err := c.StrikeDependencyTyped(context.Background(), "SPY", gammaAt, 671.3)
	var nd *NoDataError
	if !errors.As(err, &nd) || nd.Code != "strike_not_found" || nd.Response["requested_strike"] != 671.3 {
		t.Errorf("err = %T %v", err, err)
	}
}

package flashalphahistorical

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

// integrationClient returns a real client tied to https://historical.flashalpha.com,
// or skips the test if FLASHALPHA_API_KEY is not set.
func integrationClient(t *testing.T) (*Client, context.Context) {
	t.Helper()
	key := os.Getenv("FLASHALPHA_API_KEY")
	if key == "" {
		t.Skip("FLASHALPHA_API_KEY not set; skipping integration test")
	}
	c := NewClient(key)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	return c, ctx
}

const (
	spyAt        = "2024-08-05T15:30:00"
	spyDate      = "2024-08-05"
	expectedSpot = 516.435
	spotTol      = 1.0
)

var regimes = map[string]struct{}{
	"positive_gamma": {}, "negative_gamma": {}, "unknown": {},
}

// ── coverage ────────────────────────────────────────────────────────────────

func TestIntegrationTickersListsSpy(t *testing.T) {
	c, ctx := integrationClient(t)
	out, err := c.Tickers(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	count, _ := out["count"].(float64)
	if count < 1 {
		t.Fatalf("count=%v, want >=1", out["count"])
	}
	tickers, _ := out["tickers"].([]interface{})
	found := false
	for _, t_ := range tickers {
		m, _ := t_.(map[string]interface{})
		if m["symbol"] == "SPY" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("SPY not in tickers list")
	}
}

func TestIntegrationTickersFilteredReturnsObject(t *testing.T) {
	c, ctx := integrationClient(t)
	out, err := c.Tickers(ctx, "SPY")
	if err != nil {
		t.Fatal(err)
	}
	if out["symbol"] != "SPY" {
		t.Fatalf("symbol=%v", out["symbol"])
	}
	cov, _ := out["coverage"].(map[string]interface{})
	first, _ := cov["first"].(string)
	last, _ := cov["last"].(string)
	if first > "2024-08-05" || last < "2024-08-05" {
		t.Errorf("coverage [%s,%s] does not span 2024-08-05", first, last)
	}
}

func TestIntegrationUnknownSymbolNoCoverage(t *testing.T) {
	c, ctx := integrationClient(t)
	_, err := c.Tickers(ctx, "ZZZZZ")
	var ncErr *NoCoverageError
	if !errors.As(err, &ncErr) {
		t.Fatalf("expected NoCoverageError, got %T: %v", err, err)
	}
}

// ── market data ─────────────────────────────────────────────────────────────

func TestIntegrationStockQuote(t *testing.T) {
	c, ctx := integrationClient(t)
	q, err := c.StockQuote(ctx, "SPY", spyAt)
	if err != nil {
		t.Fatal(err)
	}
	if q["ticker"] != "SPY" {
		t.Errorf("ticker=%v", q["ticker"])
	}
	bid, _ := q["bid"].(float64)
	mid, _ := q["mid"].(float64)
	ask, _ := q["ask"].(float64)
	if bid > mid || mid > ask {
		t.Errorf("bid/mid/ask out of order: %v/%v/%v", bid, mid, ask)
	}
	if math.Abs(mid-expectedSpot) > spotTol {
		t.Errorf("mid=%v, want ~%v", mid, expectedSpot)
	}
	if q["lastUpdate"] != spyAt {
		t.Errorf("lastUpdate=%v, want %s", q["lastUpdate"], spyAt)
	}
}

func TestIntegrationStockQuoteDateOnlyDefaultsToClose(t *testing.T) {
	c, ctx := integrationClient(t)
	q, err := c.StockQuote(ctx, "SPY", spyDate)
	if err != nil {
		t.Fatal(err)
	}
	if last, _ := q["lastUpdate"].(string); !strings.HasSuffix(last, "T16:00:00") {
		t.Errorf("lastUpdate=%v, want T16:00:00 suffix", q["lastUpdate"])
	}
}

func TestIntegrationOptionQuoteAllFilters(t *testing.T) {
	c, ctx := integrationClient(t)
	q, err := c.OptionQuote(ctx, "SPY", spyAt,
		WithExpiry("2024-08-09"), WithStrike(520), WithType("C"))
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := q["strike"].(float64); v != 520 {
		t.Errorf("strike=%v", q["strike"])
	}
	if q["type"] != "C" {
		t.Errorf("type=%v", q["type"])
	}
	for _, g := range []string{"delta", "gamma", "theta", "vega", "rho", "vanna", "charm"} {
		if _, ok := q[g].(float64); !ok {
			t.Errorf("greek %s missing/non-number: %v", g, q[g])
		}
	}
	// Documented historical-mode gaps
	if v, _ := q["bidSize"].(float64); v != 0 {
		t.Errorf("bidSize=%v, want 0", q["bidSize"])
	}
	if v, _ := q["askSize"].(float64); v != 0 {
		t.Errorf("askSize=%v, want 0", q["askSize"])
	}
	if v, _ := q["volume"].(float64); v != 0 {
		t.Errorf("volume=%v, want 0", q["volume"])
	}
	if q["svi_vol"] != nil {
		t.Errorf("svi_vol=%v, want nil", q["svi_vol"])
	}
	if q["svi_vol_gated"] != "backtest_mode" {
		t.Errorf("svi_vol_gated=%v", q["svi_vol_gated"])
	}
}

// ── exposure ────────────────────────────────────────────────────────────────

// TestIntegrationExposureSummary: every field declared in ExposureSummaryResponse
// must be referenced by at least one assertion.
func TestIntegrationExposureSummary(t *testing.T) {
	c, ctx := integrationClient(t)
	s, err := c.ExposureSummary(ctx, "SPY", spyAt)
	if err != nil {
		t.Fatal(err)
	}
	// ── top-level scalars ──
	if sym, _ := s["symbol"].(string); sym != "SPY" {
		t.Errorf("symbol=%q, want SPY", sym)
	}
	if _, ok := s["underlying_price"].(float64); !ok {
		t.Errorf("underlying_price missing/non-number: %v", s["underlying_price"])
	}
	asOf, _ := s["as_of"].(string)
	if asOf == "" {
		t.Errorf("as_of missing/empty: %v", s["as_of"])
	}
	if asOf != spyAt {
		t.Errorf("as_of=%q, want %q (historical snaps to requested minute)", asOf, spyAt)
	}
	regime, _ := s["regime"].(string)
	if _, ok := regimes[regime]; !ok {
		t.Errorf("regime=%q not in known set", regime)
	}
	if _, ok := s["gamma_flip"].(float64); !ok {
		t.Errorf("gamma_flip missing/non-number: %v", s["gamma_flip"])
	}
	// ── exposures block (4 fields) ──
	exp, _ := s["exposures"].(map[string]interface{})
	for _, k := range []string{"net_gex", "net_dex", "net_vex", "net_chex"} {
		if _, ok := exp[k].(float64); !ok {
			t.Errorf("exposures.%s missing/non-number", k)
		}
	}
	// ── interpretation block (3 fields) ──
	interp, _ := s["interpretation"].(map[string]interface{})
	for _, k := range []string{"gamma", "vanna", "charm"} {
		v, ok := interp[k].(string)
		if !ok || v == "" {
			t.Errorf("interpretation.%s missing/empty", k)
		}
	}
	// ── hedging_estimate (every leaf on both sides) ──
	hedging, _ := s["hedging_estimate"].(map[string]interface{})
	for _, sideKey := range []string{"spot_up_1pct", "spot_down_1pct"} {
		side, _ := hedging[sideKey].(map[string]interface{})
		dir, _ := side["direction"].(string)
		if dir != "buy" && dir != "sell" {
			t.Errorf("%s.direction=%q, want buy/sell", sideKey, dir)
		}
		if _, ok := side["dealer_shares_to_trade"].(float64); !ok {
			t.Errorf("%s.dealer_shares_to_trade missing/non-number", sideKey)
		}
		notional, ok := side["notional_usd"].(float64)
		if !ok {
			t.Errorf("%s.notional_usd missing/non-number", sideKey)
		}
		if notional == 0 {
			t.Errorf("%s.notional_usd is zero", sideKey)
		}
	}
	up := hedging["spot_up_1pct"].(map[string]interface{})
	dn := hedging["spot_down_1pct"].(map[string]interface{})
	upShares, _ := up["dealer_shares_to_trade"].(float64)
	dnShares, _ := dn["dealer_shares_to_trade"].(float64)
	if upShares != -dnShares {
		t.Errorf("hedging not symmetric: up=%v down=%v", upShares, dnShares)
	}
	// ── zero_dte block (3 fields) ──
	z, ok := s["zero_dte"].(map[string]interface{})
	if !ok {
		t.Fatal("zero_dte block missing or wrong type")
	}
	if _, present := z["net_gex"]; !present {
		t.Error("zero_dte.net_gex key missing")
	} else if v := z["net_gex"]; v != nil {
		if _, ok := v.(float64); !ok {
			t.Errorf("zero_dte.net_gex non-number: %T", v)
		}
	}
	if _, present := z["pct_of_total_gex"]; !present {
		t.Error("zero_dte.pct_of_total_gex key missing")
	} else if v := z["pct_of_total_gex"]; v != nil {
		if _, ok := v.(float64); !ok {
			t.Errorf("zero_dte.pct_of_total_gex non-number: %T", v)
		}
	}
	if _, present := z["expiration"]; !present {
		t.Error("zero_dte.expiration key missing")
	} else if v := z["expiration"]; v != nil {
		if _, ok := v.(string); !ok {
			t.Errorf("zero_dte.expiration non-string: %T", v)
		}
	}
}

func TestIntegrationLevels(t *testing.T) {
	c, ctx := integrationClient(t)
	out, err := c.ExposureLevels(ctx, "SPY", spyAt)
	if err != nil {
		t.Fatal(err)
	}
	levels, _ := out["levels"].(map[string]interface{})
	for _, k := range []string{"gamma_flip", "max_positive_gamma", "max_negative_gamma",
		"call_wall", "put_wall", "highest_oi_strike"} {
		if _, ok := levels[k]; !ok {
			t.Errorf("levels.%s missing", k)
		}
	}
}

func TestIntegrationGexStrikes(t *testing.T) {
	c, ctx := integrationClient(t)
	gex, err := c.Gex(ctx, "SPY", spyAt, WithMinOI(100))
	if err != nil {
		t.Fatal(err)
	}
	strikes, _ := gex["strikes"].([]interface{})
	if len(strikes) <= 5 {
		t.Fatalf("got %d strikes, want >5", len(strikes))
	}
	first, _ := strikes[0].(map[string]interface{})
	if v, _ := first["call_volume"].(float64); v != 0 {
		t.Errorf("call_volume=%v, want 0", first["call_volume"])
	}
	if v, _ := first["put_volume"].(float64); v != 0 {
		t.Errorf("put_volume=%v, want 0", first["put_volume"])
	}
	if first["call_oi_change"] != nil || first["put_oi_change"] != nil {
		t.Errorf("oi_change should be null: call=%v put=%v", first["call_oi_change"], first["put_oi_change"])
	}
}

func TestIntegrationDexVexChex(t *testing.T) {
	c, ctx := integrationClient(t)
	type call func(context.Context, string, string, ...Option) (map[string]interface{}, error)
	tests := []struct {
		name      string
		fn        call
		netKey    string
		interpKey string
	}{
		{"Dex", c.Dex, "net_dex", ""},
		{"Vex", c.Vex, "net_vex", "vex_interpretation"},
		{"Chex", c.Chex, "net_chex", "chex_interpretation"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.fn(ctx, "SPY", spyAt)
			if err != nil {
				t.Fatal(err)
			}
			payload, _ := out["payload"].(map[string]interface{})
			if _, ok := payload[tc.netKey].(float64); !ok {
				t.Errorf("%s missing", tc.netKey)
			}
			if tc.interpKey != "" {
				if _, ok := payload[tc.interpKey].(string); !ok {
					t.Errorf("%s missing", tc.interpKey)
				}
			}
		})
	}
}

func TestIntegrationNarrative(t *testing.T) {
	c, ctx := integrationClient(t)
	out, err := c.Narrative(ctx, "SPY", spyAt)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := out["narrative"].(map[string]interface{})
	for _, b := range []string{"regime", "gex_change", "key_levels", "flow", "vanna", "charm", "zero_dte"} {
		if _, ok := n[b].(string); !ok {
			t.Errorf("narrative.%s missing", b)
		}
	}
	data, _ := n["data"].(map[string]interface{})
	changes, _ := data["top_oi_changes"].([]interface{})
	if len(changes) != 0 {
		t.Errorf("top_oi_changes should be empty, got %d", len(changes))
	}
}

func TestIntegrationZeroDte(t *testing.T) {
	c, ctx := integrationClient(t)
	out, err := c.ZeroDte(ctx, "SPY", spyAt)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"expiration", "regime", "exposures"} {
		if _, ok := out[k]; !ok {
			t.Errorf("%s missing", k)
		}
	}
}

// ── composite & vol ─────────────────────────────────────────────────────────

func TestIntegrationStockSummary(t *testing.T) {
	c, ctx := integrationClient(t)
	s, err := c.StockSummary(ctx, "SPY", spyAt)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"price", "volatility", "options_flow", "exposure", "macro"} {
		if _, ok := s[k]; !ok {
			t.Errorf("missing %s", k)
		}
	}
	flow, _ := s["options_flow"].(map[string]interface{})
	if v, _ := flow["total_call_volume"].(float64); v != 0 {
		t.Errorf("total_call_volume=%v, want 0", flow["total_call_volume"])
	}
	if flow["pc_ratio_volume"] != nil {
		t.Errorf("pc_ratio_volume should be nil, got %v", flow["pc_ratio_volume"])
	}
	macro, _ := s["macro"].(map[string]interface{})
	if macro["vix_futures"] != nil {
		t.Errorf("vix_futures should be nil")
	}
	if macro["fear_and_greed"] != nil {
		t.Errorf("fear_and_greed should be nil")
	}
}

func TestIntegrationVolatility(t *testing.T) {
	c, ctx := integrationClient(t)
	v, err := c.Volatility(ctx, "SPY", spyAt)
	if err != nil {
		t.Fatal(err)
	}
	rv, _ := v["realized_vol"].(map[string]interface{})
	for _, w := range []string{"rv_5d", "rv_10d", "rv_20d", "rv_30d", "rv_60d"} {
		if _, ok := rv[w]; !ok {
			t.Errorf("rv.%s missing", w)
		}
	}
}

func TestIntegrationAdvVolatility(t *testing.T) {
	c, ctx := integrationClient(t)
	adv, err := c.AdvVolatility(ctx, "SPY", spyAt)
	if err != nil {
		t.Fatal(err)
	}
	svi, _ := adv["svi_parameters"].([]interface{})
	if len(svi) == 0 {
		t.Fatal("svi_parameters empty")
	}
	first, _ := svi[0].(map[string]interface{})
	for _, k := range []string{"expiry", "a", "b", "rho", "m", "sigma", "forward"} {
		if _, ok := first[k]; !ok {
			t.Errorf("svi[0].%s missing", k)
		}
	}
}

// ── surface ─────────────────────────────────────────────────────────────────

func TestIntegrationSurface(t *testing.T) {
	c, ctx := integrationClient(t)
	out, err := c.Surface(ctx, "SPY", spyAt)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := out["grid_size"].(float64); v != 50 {
		t.Errorf("grid_size=%v, want 50", out["grid_size"])
	}
	tenors, _ := out["tenors"].([]interface{})
	moneyness, _ := out["moneyness"].([]interface{})
	iv, _ := out["iv"].([]interface{})
	if len(tenors) != 50 || len(moneyness) != 50 || len(iv) != 50 {
		t.Fatalf("grid dims: tenors=%d, moneyness=%d, iv=%d", len(tenors), len(moneyness), len(iv))
	}
	row0, _ := iv[0].([]interface{})
	if len(row0) != 50 {
		t.Errorf("iv[0] len=%d, want 50", len(row0))
	}
}

// ── vrp ─────────────────────────────────────────────────────────────────────

// TestIntegrationVrp — every field declared in VrpResponse must be referenced.
// Mirrors the 100% field-coverage discipline used for ExposureSummary.
func TestIntegrationVrp(t *testing.T) {
	c, ctx := integrationClient(t)
	v, err := c.Vrp(ctx, "SPY", spyAt)
	if err != nil {
		t.Fatal(err)
	}

	// ── top-level scalars ──
	if v["symbol"] != "SPY" {
		t.Errorf("symbol=%v", v["symbol"])
	}
	if _, ok := v["underlying_price"].(float64); !ok {
		t.Errorf("underlying_price missing/non-number")
	}
	if asOf, _ := v["as_of"].(string); asOf == "" {
		t.Errorf("as_of empty")
	}
	if _, ok := v["market_open"].(bool); !ok {
		t.Errorf("market_open non-bool")
	}
	for _, k := range []string{"variance_risk_premium", "convexity_premium", "fair_vol"} {
		if _, ok := v[k].(float64); !ok {
			t.Errorf("%s missing/non-number", k)
		}
	}
	if _, ok := v["warnings"].([]interface{}); !ok {
		t.Errorf("warnings missing/non-array")
	}
	// strategy_scores / net_harvest_score / dealer_flow_risk: nullable on hist
	if _, present := v["strategy_scores"]; !present {
		t.Error("strategy_scores key missing")
	}
	if _, present := v["net_harvest_score"]; !present {
		t.Error("net_harvest_score key missing")
	}
	if _, present := v["dealer_flow_risk"]; !present {
		t.Error("dealer_flow_risk key missing")
	}
	// Customer trap: net_gex must NOT be top-level
	if _, ok := v["net_gex"]; ok {
		t.Error("net_gex must NOT be top-level on vrp endpoint")
	}

	// ── vrp.* core block ──
	core, ok := v["vrp"].(map[string]interface{})
	if !ok {
		t.Fatal("vrp block missing")
	}
	for _, k := range []string{"atm_iv", "rv_5d", "rv_10d", "rv_20d", "rv_30d",
		"vrp_5d", "vrp_10d", "vrp_20d", "vrp_30d"} {
		if _, ok := core[k].(float64); !ok {
			t.Errorf("vrp.%s missing/non-number", k)
		}
	}
	// z_score / percentile nullable on historical
	if _, present := core["z_score"]; !present {
		t.Error("vrp.z_score key missing")
	}
	if _, present := core["percentile"]; !present {
		t.Error("vrp.percentile key missing")
	}
	if _, ok := core["history_days"].(float64); !ok {
		t.Error("vrp.history_days missing/non-number")
	}

	// ── directional ──
	dir, _ := v["directional"].(map[string]interface{})
	for _, k := range []string{"put_wing_iv_25d", "call_wing_iv_25d",
		"downside_rv_20d", "upside_rv_20d", "downside_vrp", "upside_vrp"} {
		if _, ok := dir[k].(float64); !ok {
			t.Errorf("directional.%s missing/non-number", k)
		}
	}
	if _, ok := dir["put_vrp"]; ok {
		t.Error("directional.put_vrp must NOT exist")
	}
	if _, ok := dir["call_vrp"]; ok {
		t.Error("directional.call_vrp must NOT exist")
	}

	// ── term_vrp[] ──
	term, ok := v["term_vrp"].([]interface{})
	if !ok || len(term) == 0 {
		t.Fatal("term_vrp empty/missing")
	}
	first, _ := term[0].(map[string]interface{})
	for _, k := range []string{"dte", "iv", "rv", "vrp"} {
		if _, present := first[k]; !present {
			t.Errorf("term_vrp[0].%s missing", k)
		}
	}

	// ── gex_conditioned + vanna_conditioned ──
	gc, _ := v["gex_conditioned"].(map[string]interface{})
	if _, ok := gc["regime"].(string); !ok {
		t.Error("gex_conditioned.regime missing")
	}
	if _, ok := gc["harvest_score"].(float64); !ok {
		t.Error("gex_conditioned.harvest_score missing")
	}
	if _, ok := gc["interpretation"].(string); !ok {
		t.Error("gex_conditioned.interpretation missing")
	}
	vc, _ := v["vanna_conditioned"].(map[string]interface{})
	if _, ok := vc["outlook"].(string); !ok {
		t.Error("vanna_conditioned.outlook missing")
	}
	if _, ok := vc["interpretation"].(string); !ok {
		t.Error("vanna_conditioned.interpretation missing")
	}

	// ── regime — net_gex lives HERE ──
	reg, _ := v["regime"].(map[string]interface{})
	if _, ok := reg["gamma"].(string); !ok {
		t.Error("regime.gamma missing")
	}
	// vrp_regime: nullable on historical
	if _, present := reg["vrp_regime"]; !present {
		t.Error("regime.vrp_regime key missing")
	}
	if _, ok := reg["net_gex"].(float64); !ok {
		t.Error("regime.net_gex missing")
	}
	if _, ok := reg["gamma_flip"].(float64); !ok {
		t.Error("regime.gamma_flip missing")
	}

	// ── macro (historical-specific shape) ──
	macro, _ := v["macro"].(map[string]interface{})
	for _, k := range []string{"vix", "vix_3m", "vix_term_slope", "dgs10", "hy_spread"} {
		if _, ok := macro[k].(float64); !ok {
			t.Errorf("macro.%s missing/non-number", k)
		}
	}
	// fed_funds is live-only — must NOT be present on historical
	if _, ok := macro["fed_funds"]; ok {
		t.Error("macro.fed_funds must NOT exist on historical")
	}
}

// ── max pain ────────────────────────────────────────────────────────────────

func TestIntegrationMaxPain(t *testing.T) {
	c, ctx := integrationClient(t)
	mp, err := c.MaxPain(ctx, "SPY", spyAt, WithExpiration("2024-08-09"))
	if err != nil {
		t.Fatal(err)
	}
	if mp["expiration"] != "2024-08-09" {
		t.Errorf("expiration=%v", mp["expiration"])
	}
	maxPainStrike, _ := mp["max_pain_strike"].(float64)
	curve, _ := mp["pain_curve"].([]interface{})
	if len(curve) == 0 {
		t.Fatal("pain_curve empty")
	}
	bestStrike := math.NaN()
	bestPain := math.MaxFloat64
	for _, row := range curve {
		r, _ := row.(map[string]interface{})
		strike, _ := r["strike"].(float64)
		total, _ := r["total_pain"].(float64)
		if total < bestPain {
			bestPain = total
			bestStrike = strike
		}
	}
	if math.Abs(bestStrike-maxPainStrike) > 5 {
		t.Errorf("min total_pain at %v but max_pain_strike=%v", bestStrike, maxPainStrike)
	}
}

// TestMaxPain_EveryFieldDeclaredInPocoMustBeReferenced — 100% field-coverage
// against the typed MaxPainResponse POCO. Historical-specific:
//   - oi_by_strike[].call_volume / put_volume are always 0.
func TestMaxPain_EveryFieldDeclaredInPocoMustBeReferenced(t *testing.T) {
	c, ctx := integrationClient(t)
	// Full chain (no expiration filter) so MaxPainByExpiration is populated.
	raw, err := c.MaxPain(ctx, "SPY", spyAt)
	if err != nil {
		t.Fatal(err)
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	r := &MaxPainResponse{}
	if err := json.Unmarshal(buf, r); err != nil {
		t.Fatalf("decode into MaxPainResponse: %v", err)
	}

	// ── top-level scalars ──
	if r.Symbol != "SPY" {
		t.Errorf("Symbol=%q", r.Symbol)
	}
	if r.UnderlyingPrice == nil || *r.UnderlyingPrice <= 0 {
		t.Errorf("UnderlyingPrice=%v", r.UnderlyingPrice)
	}
	if r.AsOf == "" {
		t.Error("AsOf empty")
	}
	if r.MaxPainStrike == nil {
		t.Error("MaxPainStrike nil")
	}
	if r.Signal == nil ||
		(*r.Signal != "bullish" && *r.Signal != "bearish" && *r.Signal != "neutral") {
		t.Errorf("Signal=%v", r.Signal)
	}
	if r.Expiration == nil || *r.Expiration == "" {
		t.Error("Expiration empty")
	}
	if r.PutCallOiRatio == nil {
		t.Error("PutCallOiRatio nil")
	}
	if r.Regime == nil {
		t.Error("Regime nil")
	} else {
		switch *r.Regime {
		case "positive_gamma", "negative_gamma", "unknown":
		default:
			t.Errorf("Regime=%q", *r.Regime)
		}
	}
	if r.PinProbability == nil || *r.PinProbability < 0 || *r.PinProbability > 100 {
		t.Errorf("PinProbability=%v", r.PinProbability)
	}

	// ── distance ──
	if r.Distance == nil {
		t.Fatal("Distance nil")
	}
	if r.Distance.Absolute == nil {
		t.Error("Distance.Absolute nil")
	}
	if r.Distance.Percent == nil {
		t.Error("Distance.Percent nil")
	}
	if r.Distance.Direction == nil ||
		(*r.Distance.Direction != "above" && *r.Distance.Direction != "below" && *r.Distance.Direction != "at") {
		t.Errorf("Distance.Direction=%v", r.Distance.Direction)
	}

	// ── pain_curve[] ──
	if len(r.PainCurve) == 0 {
		t.Fatal("PainCurve empty")
	}
	pc := r.PainCurve[0]
	if pc.Strike == nil {
		t.Error("PainCurve[0].Strike nil")
	}
	if pc.CallPain == nil {
		t.Error("PainCurve[0].CallPain nil")
	}
	if pc.PutPain == nil {
		t.Error("PainCurve[0].PutPain nil")
	}
	if pc.TotalPain == nil {
		t.Error("PainCurve[0].TotalPain nil")
	}

	// ── oi_by_strike[] — historical: volume fields always 0 ──
	if len(r.OiByStrike) == 0 {
		t.Fatal("OiByStrike empty")
	}
	oi := r.OiByStrike[0]
	if oi.Strike == nil {
		t.Error("OiByStrike[0].Strike nil")
	}
	if oi.CallOi == nil {
		t.Error("OiByStrike[0].CallOi nil")
	}
	if oi.PutOi == nil {
		t.Error("OiByStrike[0].PutOi nil")
	}
	if oi.TotalOi == nil {
		t.Error("OiByStrike[0].TotalOi nil")
	}
	if oi.CallVolume == nil || *oi.CallVolume != 0 {
		t.Errorf("OiByStrike[0].CallVolume=%v (historical expects 0)", oi.CallVolume)
	}
	if oi.PutVolume == nil || *oi.PutVolume != 0 {
		t.Errorf("OiByStrike[0].PutVolume=%v (historical expects 0)", oi.PutVolume)
	}

	// ── max_pain_by_expiration[] ──
	if len(r.MaxPainByExpiration) == 0 {
		t.Fatal("MaxPainByExpiration empty")
	}
	mr := r.MaxPainByExpiration[0]
	if mr.Expiration == nil || *mr.Expiration == "" {
		t.Error("MaxPainByExpiration[0].Expiration empty")
	}
	if mr.MaxPainStrike == nil {
		t.Error("MaxPainByExpiration[0].MaxPainStrike nil")
	}
	if mr.Dte == nil {
		t.Error("MaxPainByExpiration[0].Dte nil")
	}
	if mr.TotalOi == nil {
		t.Error("MaxPainByExpiration[0].TotalOi nil")
	}

	// ── dealer_alignment ──
	if r.DealerAlignment == nil {
		t.Fatal("DealerAlignment nil")
	}
	da := r.DealerAlignment
	if da.Alignment == nil {
		t.Error("DealerAlignment.Alignment nil")
	} else {
		switch *da.Alignment {
		case "converging", "moderate", "diverging", "unknown":
		default:
			t.Errorf("DealerAlignment.Alignment=%q", *da.Alignment)
		}
	}
	if da.Description == nil || *da.Description == "" {
		t.Error("DealerAlignment.Description empty")
	}
	if da.GammaFlip == nil {
		t.Error("DealerAlignment.GammaFlip nil")
	}
	if da.CallWall == nil {
		t.Error("DealerAlignment.CallWall nil")
	}
	if da.PutWall == nil {
		t.Error("DealerAlignment.PutWall nil")
	}

	// ── expected_move ──
	if r.ExpectedMove == nil {
		t.Fatal("ExpectedMove nil")
	}
	em := r.ExpectedMove
	if em.StraddlePrice == nil {
		t.Error("ExpectedMove.StraddlePrice nil")
	}
	if em.AtmIv == nil {
		t.Error("ExpectedMove.AtmIv nil")
	}
	if em.MaxPainWithinExpectedRange == nil {
		t.Error("ExpectedMove.MaxPainWithinExpectedRange nil")
	}
}

// TestMaxPain_ExpirationFilterSuppressesCalendar — when the expiration
// filter is set, max_pain_by_expiration MUST be null.
func TestMaxPain_ExpirationFilterSuppressesCalendar(t *testing.T) {
	c, ctx := integrationClient(t)
	mp, err := c.MaxPain(ctx, "SPY", spyAt, WithExpiration("2024-08-09"))
	if err != nil {
		t.Fatal(err)
	}
	if mp["max_pain_by_expiration"] != nil {
		t.Errorf("max_pain_by_expiration should be null when filter set, got %v",
			mp["max_pain_by_expiration"])
	}
}

// ── errors ──────────────────────────────────────────────────────────────────

func TestIntegrationInvalidAt(t *testing.T) {
	c, ctx := integrationClient(t)
	_, err := c.ExposureSummary(ctx, "SPY", "garbage")
	var iae *InvalidAtError
	if !errors.As(err, &iae) {
		t.Fatalf("expected InvalidAtError, got %T: %v", err, err)
	}
}

func TestIntegrationOutOfCoverage(t *testing.T) {
	c, ctx := integrationClient(t)
	_, err := c.ExposureSummary(ctx, "SPY", "2017-01-01")
	var ndErr *NoDataError
	if !errors.As(err, &ndErr) {
		t.Fatalf("expected NoDataError, got %T: %v", err, err)
	}
}

func TestIntegrationHoliday(t *testing.T) {
	c, ctx := integrationClient(t)
	_, err := c.ExposureSummary(ctx, "SPY", "2024-01-01")
	var ndErr *NoDataError
	if !errors.As(err, &ndErr) {
		t.Fatalf("expected NoDataError, got %T: %v", err, err)
	}
}

func TestIntegrationOptionQuoteNonexistentStrike(t *testing.T) {
	c, ctx := integrationClient(t)
	_, err := c.OptionQuote(ctx, "SPY", spyAt,
		WithExpiry("2024-08-09"), WithStrike(99999), WithType("C"))
	var ndErr *NoDataError
	if !errors.As(err, &ndErr) {
		t.Fatalf("expected NoDataError, got %T: %v", err, err)
	}
}

// ── replay & backtester ─────────────────────────────────────────────────────

func TestIntegrationReplayWeek(t *testing.T) {
	c, ctx := integrationClient(t)
	start, _ := time.Parse(AtFormatDate, "2024-08-05")
	end, _ := time.Parse(AtFormatDate, "2024-08-09")
	steps, err := Replay(ctx, c, EndpointExposureSummary, "SPY", IterDays(start, end), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 5 {
		t.Fatalf("got %d steps, want 5", len(steps))
	}
	for _, s := range steps {
		if s.Response["symbol"] != "SPY" {
			t.Errorf("symbol=%v at %s", s.Response["symbol"], s.At)
		}
		regime, _ := s.Response["regime"].(string)
		if _, ok := regimes[regime]; !ok {
			t.Errorf("unknown regime %q at %s", regime, s.At)
		}
	}
}

func TestIntegrationReplay30MinStep(t *testing.T) {
	c, ctx := integrationClient(t)
	d, _ := time.Parse(AtFormatDate, "2024-08-05")
	steps, err := Replay(ctx, c, EndpointExposureSummary, "SPY", IterMinutes(d, d, 30), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 14 {
		t.Fatalf("got %d steps, want 14", len(steps))
	}
	spots := map[float64]struct{}{}
	for _, s := range steps {
		v, _ := s.Response["underlying_price"].(float64)
		spots[v] = struct{}{}
	}
	if len(spots) <= 1 {
		t.Errorf("spot constant across day; got %d unique values", len(spots))
	}
}

func TestIntegrationReplaySkipsHolidaySilently(t *testing.T) {
	c, ctx := integrationClient(t)
	good, _ := time.Parse(AtFormatMinute, "2024-08-05T15:30:00")
	holiday, _ := time.Parse(AtFormatDate, "2024-01-01")
	holiday = time.Date(holiday.Year(), holiday.Month(), holiday.Day(), 16, 0, 0, 0, holiday.Location())

	var errored []time.Time
	steps, err := Replay(ctx, c, EndpointExposureSummary, "SPY",
		[]time.Time{good, holiday},
		&ReplayOptions{
			SkipMissing: true,
			OnError:     func(at time.Time, _ error) { errored = append(errored, at) },
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(steps))
	}
	if len(errored) != 1 {
		t.Fatalf("got %d errors, want 1", len(errored))
	}
}

func TestIntegrationBacktester(t *testing.T) {
	c, ctx := integrationClient(t)
	bt := NewBacktester(c)
	bt.Endpoint = EndpointStockSummary
	bt.Symbol = "SPY"

	start, _ := time.Parse(AtFormatDate, "2024-08-05")
	end, _ := time.Parse(AtFormatDate, "2024-08-09")
	results, err := bt.Run(ctx, IterDays(start, end), func(at string, snap map[string]interface{}) interface{} {
		vol, _ := snap["volatility"].(map[string]interface{})
		exp, _ := snap["exposure"].(map[string]interface{})
		return map[string]interface{}{
			"vrp":    vol["vrp"],
			"regime": exp["regime"],
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 5 {
		t.Fatalf("got %d, want 5", len(results))
	}
	for _, r := range results {
		out, _ := r.Output.(map[string]interface{})
		regime, _ := out["regime"].(string)
		if _, ok := regimes[regime]; !ok {
			t.Errorf("unknown regime %q at %s", regime, r.At)
		}
	}
}

// ── rc.4 typed-POCO field-walk tests ─────────────────────────────────────────
//
// Each test below decodes the historical raw response into the canonical typed
// POCO and asserts every exported pointer field on a SPY response at spyAt is
// non-nil (modulo documented nullable / historical-mode-gap fields).

// TestIntegrationStockSummary_EveryFieldDeclaredInPocoMustBeReferenced —
// 100% field-coverage walk against StockSummaryResponse on SPY @ spyAt.
//
// Historical-mode gaps that are intentionally NOT asserted non-nil:
//   - OptionsFlow.TotalCallVolume / TotalPutVolume / PcRatioVolume
//     (volumes are always 0/nil on historical — documented backtest_mode gap)
//   - Macro.VixFutures / Macro.FearAndGreed (live-only feeds)
//   - StockSummarySkew25d.* (skew fitter best-effort at the as-of minute)
func TestIntegrationStockSummary_EveryFieldDeclaredInPocoMustBeReferenced(t *testing.T) {
	c, ctx := integrationClient(t)
	raw, err := c.StockSummary(ctx, "SPY", spyAt)
	if err != nil {
		t.Fatal(err)
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	r := &StockSummaryResponse{}
	if err := json.Unmarshal(buf, r); err != nil {
		t.Fatalf("decode into StockSummaryResponse: %v", err)
	}

	// ── top-level scalars ──
	if r.Symbol != "SPY" {
		t.Errorf("Symbol=%q", r.Symbol)
	}
	if r.AsOf == "" {
		t.Error("AsOf empty")
	}
	_ = r.MarketOpen

	// ── Price ──
	if r.Price == nil {
		t.Fatal("Price nil")
	}
	if r.Price.Bid == nil {
		t.Error("Price.Bid nil")
	}
	if r.Price.Ask == nil {
		t.Error("Price.Ask nil")
	}
	if r.Price.Mid == nil || *r.Price.Mid <= 0 {
		t.Errorf("Price.Mid=%v (canonical reference price)", r.Price.Mid)
	}
	if r.Price.Last == nil {
		t.Error("Price.Last nil")
	}
	if r.Price.LastUpdate == nil {
		t.Error("Price.LastUpdate nil")
	}

	// ── Volatility ──
	if r.Volatility == nil {
		t.Fatal("Volatility nil")
	}
	if r.Volatility.AtmIv == nil {
		t.Error("Volatility.AtmIv nil")
	}
	if r.Volatility.Hv20 == nil {
		t.Error("Volatility.Hv20 nil")
	}
	if r.Volatility.Hv60 == nil {
		t.Error("Volatility.Hv60 nil")
	}
	if r.Volatility.Vrp == nil {
		t.Error("Volatility.Vrp nil")
	}
	_ = r.Volatility.Skew25d
	if len(r.Volatility.IvTermStructure) == 0 {
		t.Error("Volatility.IvTermStructure empty")
	} else {
		p := r.Volatility.IvTermStructure[0]
		if p.Expiry == nil {
			t.Error("IvTermStructure[0].Expiry nil")
		}
		if p.Iv == nil {
			t.Error("IvTermStructure[0].Iv nil")
		}
		if p.DaysToExpiry == nil {
			t.Error("IvTermStructure[0].DaysToExpiry nil")
		}
	}

	// ── OptionsFlow (volumes are 0/nil on historical) ──
	if r.OptionsFlow == nil {
		t.Fatal("OptionsFlow nil")
	}
	if r.OptionsFlow.TotalCallOi == nil {
		t.Error("OptionsFlow.TotalCallOi nil")
	}
	if r.OptionsFlow.TotalPutOi == nil {
		t.Error("OptionsFlow.TotalPutOi nil")
	}
	if r.OptionsFlow.PcRatioOi == nil {
		t.Error("OptionsFlow.PcRatioOi nil")
	}
	if r.OptionsFlow.ActiveExpirations == nil {
		t.Error("OptionsFlow.ActiveExpirations nil")
	}
	// Volume fields documented-zero on historical — just reference them.
	_ = r.OptionsFlow.TotalCallVolume
	_ = r.OptionsFlow.TotalPutVolume
	_ = r.OptionsFlow.PcRatioVolume

	// ── Exposure ──
	if r.Exposure == nil {
		t.Fatal("Exposure nil")
	}
	for label, ptr := range map[string]*float64{
		"NetGex":          r.Exposure.NetGex,
		"NetDex":          r.Exposure.NetDex,
		"NetVex":          r.Exposure.NetVex,
		"NetChex":         r.Exposure.NetChex,
		"GammaFlip":       r.Exposure.GammaFlip,
		"CallWall":        r.Exposure.CallWall,
		"PutWall":         r.Exposure.PutWall,
		"MaxPain":         r.Exposure.MaxPain,
		"HighestOiStrike": r.Exposure.HighestOiStrike,
		"OiWeightedDte":   r.Exposure.OiWeightedDte,
	} {
		if ptr == nil {
			t.Errorf("Exposure.%s nil", label)
		}
	}
	switch r.Exposure.Regime {
	case "positive_gamma", "negative_gamma", "unknown":
	default:
		t.Errorf("Exposure.Regime=%q", r.Exposure.Regime)
	}
	if r.Exposure.Interpretation == nil {
		t.Error("Exposure.Interpretation nil")
	} else {
		if r.Exposure.Interpretation.Gamma == "" {
			t.Error("Exposure.Interpretation.Gamma empty")
		}
		if r.Exposure.Interpretation.Vanna == "" {
			t.Error("Exposure.Interpretation.Vanna empty")
		}
		if r.Exposure.Interpretation.Charm == "" {
			t.Error("Exposure.Interpretation.Charm empty")
		}
	}
	if r.Exposure.HedgingEstimate == nil {
		t.Fatal("Exposure.HedgingEstimate nil")
	}
	for label, side := range map[string]*StockSummaryHedgingMove{
		"SpotUp1Pct":   r.Exposure.HedgingEstimate.SpotUp1Pct,
		"SpotDown1Pct": r.Exposure.HedgingEstimate.SpotDown1Pct,
	} {
		if side == nil {
			t.Errorf("HedgingEstimate.%s nil", label)
			continue
		}
		if side.DealerShares == nil {
			t.Errorf("HedgingEstimate.%s.DealerShares nil", label)
		}
		if side.Direction != "buy" && side.Direction != "sell" {
			t.Errorf("HedgingEstimate.%s.Direction=%q", label, side.Direction)
		}
		if side.NotionalUsd == nil {
			t.Errorf("HedgingEstimate.%s.NotionalUsd nil", label)
		}
	}
	_ = r.Exposure.ZeroDte
	if len(r.Exposure.TopStrikes) == 0 {
		t.Error("Exposure.TopStrikes empty")
	} else {
		ts := r.Exposure.TopStrikes[0]
		if ts.Strike == nil {
			t.Error("TopStrikes[0].Strike nil")
		}
		if ts.NetGex == nil {
			t.Error("TopStrikes[0].NetGex nil")
		}
		if ts.CallOi == nil {
			t.Error("TopStrikes[0].CallOi nil")
		}
		if ts.PutOi == nil {
			t.Error("TopStrikes[0].PutOi nil")
		}
		if ts.TotalOi == nil {
			t.Error("TopStrikes[0].TotalOi nil")
		}
	}

	// ── Macro (VixFutures + FearAndGreed nil on historical) ──
	if r.Macro == nil {
		t.Fatal("Macro nil")
	}
	checkQuote := func(name string, q *StockSummaryMacroQuote) {
		t.Helper()
		if q == nil {
			t.Errorf("Macro.%s nil", name)
			return
		}
		if q.Value == nil {
			t.Errorf("Macro.%s.Value nil", name)
		}
	}
	checkQuote("Vix", r.Macro.Vix)
	checkQuote("Vvix", r.Macro.Vvix)
	checkQuote("Skew", r.Macro.Skew)
	checkQuote("Spx", r.Macro.Spx)
	checkQuote("Move", r.Macro.Move)
	if r.Macro.VixTermStructure == nil {
		t.Fatal("Macro.VixTermStructure nil")
	}
	if r.Macro.VixTermStructure.Levels == nil {
		t.Fatal("Macro.VixTermStructure.Levels nil")
	}
	for label, ptr := range map[string]*float64{
		"Vix9d": r.Macro.VixTermStructure.Levels.Vix9d,
		"Vix":   r.Macro.VixTermStructure.Levels.Vix,
		"Vix3m": r.Macro.VixTermStructure.Levels.Vix3m,
		"Vix6m": r.Macro.VixTermStructure.Levels.Vix6m,
	} {
		if ptr == nil {
			t.Errorf("VixTermStructure.Levels.%s nil (silent-null trap if json tag drifted)", label)
		}
	}
	if r.Macro.VixTermStructure.NearSlopePct == nil {
		t.Error("Macro.VixTermStructure.NearSlopePct nil")
	}
	if r.Macro.VixTermStructure.Structure == nil {
		t.Error("Macro.VixTermStructure.Structure nil")
	}
	// VixFutures + FearAndGreed are live-only — historical returns nil.
	_ = r.Macro.VixFutures
	_ = r.Macro.FearAndGreed
}

// TestIntegrationNarrative_EveryFieldDeclaredInPocoMustBeReferenced —
// 100% field-coverage walk against NarrativeResponse on SPY @ spyAt.
//
// Historical-mode: TopOiChanges is documented-empty on historical (no
// prior-session OI baseline) — only assert shape when populated.
func TestIntegrationNarrative_EveryFieldDeclaredInPocoMustBeReferenced(t *testing.T) {
	c, ctx := integrationClient(t)
	raw, err := c.Narrative(ctx, "SPY", spyAt)
	if err != nil {
		t.Fatal(err)
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	r := &NarrativeResponse{}
	if err := json.Unmarshal(buf, r); err != nil {
		t.Fatalf("decode into NarrativeResponse: %v", err)
	}

	if r.Symbol != "SPY" {
		t.Errorf("Symbol=%q", r.Symbol)
	}
	if r.UnderlyingPrice == nil || *r.UnderlyingPrice <= 0 {
		t.Errorf("UnderlyingPrice=%v", r.UnderlyingPrice)
	}
	if r.AsOf == "" {
		t.Error("AsOf empty")
	}
	if r.Narrative == nil {
		t.Fatal("Narrative nil")
	}
	for label, s := range map[string]string{
		"Regime":    r.Narrative.Regime,
		"GexChange": r.Narrative.GexChange,
		"KeyLevels": r.Narrative.KeyLevels,
		"Flow":      r.Narrative.Flow,
		"Vanna":     r.Narrative.Vanna,
		"Charm":     r.Narrative.Charm,
		"ZeroDte":   r.Narrative.ZeroDte,
		"Outlook":   r.Narrative.Outlook,
	} {
		if s == "" {
			t.Errorf("Narrative.%s empty", label)
		}
	}
	if r.Narrative.Data == nil {
		t.Fatal("Narrative.Data nil")
	}
	d := r.Narrative.Data
	if d.NetGex == nil {
		t.Error("Data.NetGex nil")
	}
	if d.NetGexPrior == nil {
		t.Error("Data.NetGexPrior nil")
	}
	if d.NetGexChangePct == nil {
		t.Error("Data.NetGexChangePct nil")
	}
	if d.Vix == nil {
		t.Error("Data.Vix nil")
	}
	if d.GammaFlip == nil {
		t.Error("Data.GammaFlip nil")
	}
	if d.CallWall == nil {
		t.Error("Data.CallWall nil")
	}
	if d.PutWall == nil {
		t.Error("Data.PutWall nil")
	}
	switch d.Regime {
	case "positive_gamma", "negative_gamma", "unknown":
	default:
		t.Errorf("Data.Regime=%q", d.Regime)
	}
	if d.ZeroDtePct == nil {
		t.Error("Data.ZeroDtePct nil")
	}
	// TopOiChanges documented-empty on historical — only walk fields if any.
	if len(d.TopOiChanges) > 0 {
		row := d.TopOiChanges[0]
		if row.Strike == nil {
			t.Error("TopOiChanges[0].Strike nil")
		}
		if row.Type != "call" && row.Type != "put" {
			t.Errorf("TopOiChanges[0].Type=%q", row.Type)
		}
		if row.OiChange == nil {
			t.Error("TopOiChanges[0].OiChange nil")
		}
		if row.Volume == nil {
			t.Error("TopOiChanges[0].Volume nil")
		}
	}
}

// TestIntegrationLevels_EveryFieldDeclaredInPocoMustBeReferenced —
// 100% field-coverage walk against LevelsResponse on SPY @ spyAt. ZeroDteMagnet
// is asserted non-nil specifically — 2024-08-05 is a Monday and SPY had a
// 0DTE chain at 15:30 ET.
func TestIntegrationLevels_EveryFieldDeclaredInPocoMustBeReferenced(t *testing.T) {
	c, ctx := integrationClient(t)
	raw, err := c.ExposureLevels(ctx, "SPY", spyAt)
	if err != nil {
		t.Fatal(err)
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	r := &LevelsResponse{}
	if err := json.Unmarshal(buf, r); err != nil {
		t.Fatalf("decode into LevelsResponse: %v", err)
	}

	if r.Symbol != "SPY" {
		t.Errorf("Symbol=%q", r.Symbol)
	}
	if r.UnderlyingPrice == nil || *r.UnderlyingPrice <= 0 {
		t.Errorf("UnderlyingPrice=%v", r.UnderlyingPrice)
	}
	if r.AsOf == "" {
		t.Error("AsOf empty")
	}
	if r.Levels == nil {
		t.Fatal("Levels nil")
	}
	for label, ptr := range map[string]*float64{
		"GammaFlip":        r.Levels.GammaFlip,
		"MaxPositiveGamma": r.Levels.MaxPositiveGamma,
		"MaxNegativeGamma": r.Levels.MaxNegativeGamma,
		"CallWall":         r.Levels.CallWall,
		"PutWall":          r.Levels.PutWall,
		"HighestOiStrike":  r.Levels.HighestOiStrike,
		// ZeroDteMagnet — most SDKs miss this assertion. SPY @ 2024-08-05
		// 15:30 ET had 0DTE active.
		"ZeroDteMagnet": r.Levels.ZeroDteMagnet,
	} {
		if ptr == nil {
			t.Errorf("Levels.%s nil", label)
		}
	}
}

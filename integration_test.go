package flashalphahistorical

import (
	"context"
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
	"positive_gamma": {}, "negative_gamma": {}, "transition": {},
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

func TestIntegrationExposureSummary(t *testing.T) {
	c, ctx := integrationClient(t)
	s, err := c.ExposureSummary(ctx, "SPY", spyAt)
	if err != nil {
		t.Fatal(err)
	}
	regime, _ := s["regime"].(string)
	if _, ok := regimes[regime]; !ok {
		t.Errorf("regime=%q not in known set", regime)
	}
	if _, ok := s["gamma_flip"].(float64); !ok {
		t.Errorf("gamma_flip missing/non-number: %v", s["gamma_flip"])
	}
	exp, _ := s["exposures"].(map[string]interface{})
	for _, k := range []string{"net_gex", "net_dex", "net_vex", "net_chex"} {
		if _, ok := exp[k].(float64); !ok {
			t.Errorf("exposures.%s missing/non-number", k)
		}
	}
	hedging, _ := s["hedging_estimate"].(map[string]interface{})
	up, _ := hedging["spot_up_1pct"].(map[string]interface{})
	dn, _ := hedging["spot_down_1pct"].(map[string]interface{})
	upShares, _ := up["dealer_shares_to_trade"].(float64)
	dnShares, _ := dn["dealer_shares_to_trade"].(float64)
	if upShares != -dnShares {
		t.Errorf("hedging not symmetric: up=%v down=%v", upShares, dnShares)
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

func TestIntegrationVrp(t *testing.T) {
	c, ctx := integrationClient(t)
	v, err := c.Vrp(ctx, "SPY", spyAt)
	if err != nil {
		t.Fatal(err)
	}
	core, _ := v["vrp"].(map[string]interface{})
	for _, k := range []string{"atm_iv", "rv_5d", "rv_10d", "rv_20d", "rv_30d",
		"vrp_5d", "vrp_10d", "vrp_20d", "vrp_30d"} {
		if _, ok := core[k]; !ok {
			t.Errorf("vrp.%s missing", k)
		}
	}
	macro, _ := v["macro"].(map[string]interface{})
	if got, _ := macro["hy_spread"].(float64); got != 3.5 {
		t.Errorf("hy_spread=%v, want 3.5", macro["hy_spread"])
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

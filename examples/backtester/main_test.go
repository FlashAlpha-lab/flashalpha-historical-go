// Offline tests: a stub replaces the API, so no key or network is needed.
//
//	go test ./...
package main

import (
	"encoding/csv"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// stubData is three sessions of canned API responses. The put spread opens on day 2 and hits the profit
// target on day 3.
type stubData struct {
	days  map[string]stubDay
	marks map[string]map[stubContract][2]float64 // marks per date for the two legs: {bid, ask}
}

type stubDay struct {
	regime string
	vrp20d float64
}

type stubContract struct {
	optType string
	strike  float64
}

func newStubData() *stubData {
	return &stubData{
		days: map[string]stubDay{
			"2024-03-04": {"positive_gamma", 2.0},
			"2024-03-05": {"positive_gamma", 2.0},
			"2024-03-06": {"positive_gamma", 2.0},
		},
		marks: map[string]map[stubContract][2]float64{
			"2024-03-06": {{"P", 490.0}: {0.30, 0.34}, {"P", 485.0}: {0.10, 0.12}},
		},
	}
}

func ptr[T any](v T) *T { return &v }

func (s *stubData) StockQuote(symbol, at string) (*StockQuote, error) {
	if _, ok := s.days[at[:10]]; !ok {
		return &StockQuote{Mid: ptr(500.0), LastUpdate: "2024-02-29T16:00:00"}, nil // holiday: an earlier session's quote
	}
	return &StockQuote{Mid: ptr(500.0), LastUpdate: at}, nil
}

func (s *stubData) ExposureSummary(symbol, at string) (*ExposureSummary, error) {
	return &ExposureSummary{
		Regime:    ptr(s.days[at[:10]].regime),
		GammaFlip: ptr(495.0),
		Exposures: &Exposures{NetGEX: ptr(1e9)},
	}, nil
}

func (s *stubData) VRP(symbol, at string) (*VRP, error) {
	return &VRP{
		Core: &VRPCore{ATMIV: ptr(14.0), RV20d: ptr(12.0), VRP20d: ptr(s.days[at[:10]].vrp20d),
			Percentile: ptr(60.0), ZScore: ptr(0.5)},
		GexConditioned: &GexConditioned{HarvestScore: ptr(70.0)},
	}, nil
}

func (s *stubData) Chain(symbol, at, expiry string) ([]OptionQuote, error) {
	if expiry != "2024-04-05" {
		return nil, nil
	}
	row := func(optType string, strike, bid, ask, mid, delta float64) OptionQuote {
		return OptionQuote{Type: optType, Strike: strike, Expiry: expiry, Bid: ptr(bid), Ask: ptr(ask), Mid: ptr(mid), Delta: ptr(delta)}
	}
	return []OptionQuote{
		row("P", 490.0, 1.00, 1.10, 1.05, -0.21),
		row("P", 485.0, 0.40, 0.46, 0.43, -0.12),
		row("P", 480.0, 0.20, 0.24, 0.22, -0.07),
		row("C", 510.0, 1.00, 1.10, 1.05, 0.20),
	}, nil
}

func (s *stubData) Option(symbol, at, expiry string, strike float64, optType string) (*OptionQuote, error) {
	m, ok := s.marks[at[:10]][stubContract{optType, strike}]
	if !ok {
		return nil, nil
	}
	return &OptionQuote{Bid: ptr(m[0]), Ask: ptr(m[1]), Mid: ptr((m[0] + m[1]) / 2)}, nil
}

func approx(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6*math.Max(1, math.Abs(want)) {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestFillPriceMovesFromMidTowardTheFarSide(t *testing.T) {
	q := OptionQuote{Bid: ptr(1.00), Ask: ptr(1.20)}
	approx(t, "sell at slippage 0", fillPrice(q, -1, 0.0), 1.10)
	approx(t, "sell at slippage 1", fillPrice(q, -1, 1.0), 1.00)
	approx(t, "buy at slippage 0.5", fillPrice(q, +1, 0.5), 1.15)
}

func TestSelectLegsBuildsAPutCreditSpread(t *testing.T) {
	chain, _ := newStubData().Chain("SPY", "2024-03-05T15:45:00", "2024-04-05")
	legs := selectLegs(chain, Snapshot{}, defaultConfig())
	if len(legs) != 2 || legs[0].Strike != 490 || legs[0].Side != -1 || legs[1].Strike != 485 || legs[1].Side != +1 {
		t.Fatalf("legs = %+v, want sell 490 / buy 485", legs)
	}
}

func TestRunOpensAndClosesOneTradeWithExactAccounting(t *testing.T) {
	cfg := defaultConfig()
	cfg.Start, cfg.End = "2024-03-04", "2024-03-06"
	cfg.MinDTE, cfg.MaxDTE, cfg.TargetDTE = 25, 40, 31
	dir := t.TempDir()
	summary, err := run(cfg, newStubData(), dir)
	if err != nil {
		t.Fatal(err)
	}
	trades := readCSV(t, filepath.Join(dir, "trades.csv"))
	if summary.Trades != 1 || len(trades) != 1 {
		t.Fatalf("summary.Trades = %d, CSV rows = %d, want 1 and 1", summary.Trades, len(trades))
	}
	tr := trades[0]
	// Entry on 03-05 (2 positive-gamma sessions, VRP > 0), halfway fills: sell 490 at 1.025, buy 485 at 0.445.
	// Credit = (1.025 - 0.445) * 100 - 2 legs * 0.65 = 56.70
	if tr["entry_date"] != "2024-03-05" {
		t.Errorf("entry_date = %s, want 2024-03-05", tr["entry_date"])
	}
	approx(t, "credit", parseFloat(t, tr["credit"]), 56.70)
	// Day 3 mid value of the spread: 0.32 - 0.11 = 0.21 -> open profit 56.70 - 21 = 35.70 >= 50% of credit.
	// Close halfway: buy 490 at 0.33, sell 485 at 0.105 -> -22.50, less 1.30 commission: 56.70 - 22.50 - 1.30 = 32.90
	if tr["exit_reason"] != "profit_target" {
		t.Errorf("exit_reason = %s, want profit_target", tr["exit_reason"])
	}
	approx(t, "pnl", parseFloat(t, tr["pnl"]), 32.90)
	if tr["legs"] != "-P490 +P485" || tr["expiry"] != "2024-04-05" {
		t.Errorf("legs = %q, expiry = %q, want \"-P490 +P485\" and 2024-04-05", tr["legs"], tr["expiry"])
	}
}

func TestStrikeTextKeepsEveryDigit(t *testing.T) {
	for strike, want := range map[float64]string{512.0: "512", 517.5: "517.5", 21012.25: "21012.25"} {
		if got := strikeText(strike); got != want {
			t.Errorf("strikeText(%v) = %q, want %q", strike, got, want)
		}
	}
}

func TestHolidaysAreSkipped(t *testing.T) {
	cfg := defaultConfig()
	cfg.Start, cfg.End = "2024-03-01", "2024-03-04" // 03-01 returns an earlier quote, 03-02/03 are a weekend
	summary, err := run(cfg, newStubData(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if summary.Sessions != 1 {
		t.Errorf("sessions = %d, want 1", summary.Sessions)
	}
}

// readCSV returns the data rows of a CSV file as maps keyed by the header.
func readCSV(t *testing.T, path string) []map[string]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]string
	for _, rec := range records[1:] {
		row := make(map[string]string, len(rec))
		for i, v := range rec {
			row[records[0][i]] = v
		}
		rows = append(rows, row)
	}
	return rows
}

func parseFloat(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

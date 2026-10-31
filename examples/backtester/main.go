// FlashAlpha options backtester: a complete, runnable template.
//
// Replays the FlashAlpha Historical API day by day, decides entries and exits from the same endpoint outputs you
// would read live, fills trades on real historical bid/ask quotes, and writes trades.csv and equity.csv.
//
// Edit the three marked functions to change the strategy:
//
//	entryRules()  - when to open a position (reads FlashAlpha signals)
//	selectLegs()  - which option contracts to trade (reads the option chain)
//	exitRules()   - when to close (reads the position's marks and FlashAlpha signals)
//
// Requirements: Go 1.21+, standard library only. An Alpha plan or higher (Historical API).
//
//	export FLASHALPHA_API_KEY=your_key
//	go run . -symbol SPY -start 2024-01-02 -end 2024-06-28
//
// Every API response is cached in .fa_cache/, so a rerun with changed rules costs no API calls for days already
// fetched. The cache layout matches the Python version, so both can share one cache folder.
package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

const baseURL = "https://historical.flashalpha.com"

// ============================================================================================================
// Settings
// ============================================================================================================

// Config holds every strategy and engine setting. Each field is also a command-line flag.
type Config struct {
	Symbol       string
	Start        string
	End          string
	DecisionTime string // ET; every decision and fill uses quotes at this minute
	// Trade selection (a put credit spread by default)
	TargetDTE    int // pick the expiry closest to this many calendar days out...
	MinDTE       int // ...within this window
	MaxDTE       int
	ShortDelta   float64 // sell the put closest to this absolute delta
	Width        float64 // buy the put this many points lower
	Contracts    int
	MaxPositions int // open positions allowed at once
	// Exits
	ProfitTarget float64 // close when 50% of the entry credit is captured
	StopLoss     float64 // close when the loss reaches 2x the entry credit
	ExitDTE      int     // close with this many calendar days left
	// Costs
	Slippage   float64 // 0 = fill at mid, 1 = fill at the bid or ask, 0.5 = halfway
	Commission float64 // per contract per leg, each side
}

func defaultConfig() Config {
	return Config{
		Symbol:       "SPY",
		Start:        "2024-01-02",
		End:          "2024-06-28",
		DecisionTime: "15:45",
		TargetDTE:    35,
		MinDTE:       25,
		MaxDTE:       50,
		ShortDelta:   0.20,
		Width:        5.0,
		Contracts:    1,
		MaxPositions: 1,
		ProfitTarget: 0.50,
		StopLoss:     2.00,
		ExitDTE:      21,
		Slippage:     0.5,
		Commission:   0.65,
	}
}

// ============================================================================================================
// FlashAlpha data: a small cached client for the Historical API
// ============================================================================================================

// FlashAlphaData is a Historical API client that caches every response on disk.
type FlashAlphaData struct {
	baseURL  string
	apiKey   string
	cacheDir string
	http     *http.Client
	Calls    int // HTTP requests made this run (cache hits are free)
}

// NewFlashAlphaData returns a client that caches responses in cacheDir.
func NewFlashAlphaData(apiKey, cacheDir, base string) (*FlashAlphaData, error) {
	if apiKey == "" {
		return nil, errors.New("Set FLASHALPHA_API_KEY (Alpha plan or higher).")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	return &FlashAlphaData{
		baseURL:  strings.TrimRight(base, "/"),
		apiKey:   apiKey,
		cacheDir: cacheDir,
		http:     &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// Get fetches path with the query params, using the disk cache, and decodes the JSON into out.
// It returns false when the API has no data (404).
func (fa *FlashAlphaData) Get(path string, params map[string]string, out any) (bool, error) {
	body, err := fa.getRaw(path, params)
	if err != nil {
		return false, err
	}
	if string(body) == "null" {
		return false, nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return false, fmt.Errorf("decoding %s: %w", path, err)
	}
	return true, nil
}

// getRaw returns the response body, from the cache when possible. A 404 is cached as "null".
func (fa *FlashAlphaData) getRaw(path string, params map[string]string) ([]byte, error) {
	hit := filepath.Join(fa.cacheDir, cacheKey(path, params)+".json")
	if body, err := os.ReadFile(hit); err == nil {
		return body, nil
	}
	query := url.Values{}
	for k, v := range params {
		query.Set(k, v)
	}
	requestURL := fa.baseURL + path + "?" + query.Encode()
	var body []byte
	for attempt := 0; ; attempt++ {
		if attempt == 5 {
			return nil, fmt.Errorf("FlashAlpha API kept failing on %s", path)
		}
		resp, data, err := fa.fetch(requestURL)
		if err != nil { // connection error or timeout: wait and retry, not counted as a call
			time.Sleep(backoff(attempt))
			continue
		}
		fa.Calls++
		switch resp.StatusCode {
		case http.StatusOK:
			if !json.Valid(data) {
				return nil, fmt.Errorf("FlashAlpha API returned invalid JSON on %s", path)
			}
			body = data
		case http.StatusNotFound:
			body = []byte("null")
		case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
			http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			time.Sleep(retryDelay(resp.Header.Get("Retry-After"), attempt))
			continue
		default:
			if len(data) > 200 {
				data = data[:200]
			}
			return nil, fmt.Errorf("FlashAlpha API error %d on %s: %s", resp.StatusCode, path, data)
		}
		break
	}
	if err := os.WriteFile(hit, body, 0o644); err != nil {
		return nil, err
	}
	return body, nil
}

// fetch makes one GET request and reads the whole body.
func (fa *FlashAlphaData) fetch(requestURL string) (*http.Response, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("X-Api-Key", fa.apiKey)
	resp, err := fa.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return resp, data, nil
}

// retryDelay honours a Retry-After header given in whole seconds, else backs off 1, 2, 4, 8, 16 seconds.
func retryDelay(retryAfter string, attempt int) time.Duration {
	if retryAfter != "" && strings.Trim(retryAfter, "0123456789") == "" {
		if n, err := strconv.Atoi(retryAfter); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return backoff(attempt)
}

func backoff(attempt int) time.Duration {
	return time.Duration(1<<attempt) * time.Second
}

// cacheKey is the SHA-1 of Python's json.dumps([path, sorted(params.items())]), so the Go and Python
// versions read and write the same cache files.
func cacheKey(path string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("[" + pyJSONString(path) + ", [")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("[" + pyJSONString(k) + ", " + pyJSONString(params[k]) + "]")
	}
	b.WriteString("]]")
	sum := sha1.Sum([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// StockQuote is the part of /v1/stockquote the backtester reads.
type StockQuote struct {
	Mid        *float64 `json:"mid"`
	LastUpdate string   `json:"lastUpdate"`
}

// ExposureSummary is the part of /v1/exposure/summary the backtester reads.
type ExposureSummary struct {
	Regime    *string    `json:"regime"`
	GammaFlip *float64   `json:"gamma_flip"`
	Exposures *Exposures `json:"exposures"`
}

// Exposures is the "exposures" object of /v1/exposure/summary.
type Exposures struct {
	NetGEX *float64 `json:"net_gex"`
}

// VRP is the part of /v1/vrp the backtester reads.
type VRP struct {
	Core           *VRPCore        `json:"vrp"`
	GexConditioned *GexConditioned `json:"gex_conditioned"`
}

// VRPCore is the "vrp" object of /v1/vrp.
type VRPCore struct {
	ATMIV      *float64 `json:"atm_iv"`
	RV20d      *float64 `json:"rv_20d"`
	VRP20d     *float64 `json:"vrp_20d"`
	Percentile *float64 `json:"percentile"`
	ZScore     *float64 `json:"z_score"`
}

// GexConditioned is the "gex_conditioned" object of /v1/vrp.
type GexConditioned struct {
	HarvestScore *float64 `json:"harvest_score"`
}

// OptionQuote is one contract from /v1/optionquote. Pointer fields are nil when the API sends null.
type OptionQuote struct {
	Type         string   `json:"type"` // "C" or "P"
	Expiry       string   `json:"expiry"`
	Strike       float64  `json:"strike"`
	Bid          *float64 `json:"bid"`
	Ask          *float64 `json:"ask"`
	Mid          *float64 `json:"mid"`
	ImpliedVol   *float64 `json:"implied_vol"`
	Delta        *float64 `json:"delta"`
	Gamma        *float64 `json:"gamma"`
	Theta        *float64 `json:"theta"`
	Vega         *float64 `json:"vega"`
	OpenInterest *float64 `json:"open_interest"`
	Volume       *float64 `json:"volume"`
}

// DataSource is what the engine reads. FlashAlphaData implements it against the API; the tests use a stub.
// Each method returns nil (or an empty chain) when there is no data.
type DataSource interface {
	StockQuote(symbol, at string) (*StockQuote, error)
	ExposureSummary(symbol, at string) (*ExposureSummary, error)
	VRP(symbol, at string) (*VRP, error)
	Chain(symbol, at, expiry string) ([]OptionQuote, error)
	Option(symbol, at, expiry string, strike float64, optType string) (*OptionQuote, error)
}

// strikeText writes a strike as the API and the trade log do: 512 -> "512", 517.5 -> "517.5",
// 21012.25 -> "21012.25" (Python's format(strike, ".10g")).
func strikeText(strike float64) string {
	return strconv.FormatFloat(strike, 'g', 10, 64)
}

// StockQuote returns /v1/stockquote/{symbol} at a time, or nil when there is no data.
func (fa *FlashAlphaData) StockQuote(symbol, at string) (*StockQuote, error) {
	var q StockQuote
	ok, err := fa.Get("/v1/stockquote/"+symbol, map[string]string{"at": at}, &q)
	if !ok || err != nil {
		return nil, err
	}
	return &q, nil
}

// ExposureSummary returns /v1/exposure/summary/{symbol} at a time, or nil when there is no data.
func (fa *FlashAlphaData) ExposureSummary(symbol, at string) (*ExposureSummary, error) {
	var ex ExposureSummary
	ok, err := fa.Get("/v1/exposure/summary/"+symbol, map[string]string{"at": at}, &ex)
	if !ok || err != nil {
		return nil, err
	}
	return &ex, nil
}

// VRP returns /v1/vrp/{symbol} at a time, or nil when there is no data.
func (fa *FlashAlphaData) VRP(symbol, at string) (*VRP, error) {
	var v VRP
	ok, err := fa.Get("/v1/vrp/"+symbol, map[string]string{"at": at}, &v)
	if !ok || err != nil {
		return nil, err
	}
	return &v, nil
}

// Chain returns every contract of one expiry, or an empty slice when there is no data.
func (fa *FlashAlphaData) Chain(symbol, at, expiry string) ([]OptionQuote, error) {
	var chain []OptionQuote
	_, err := fa.Get("/v1/optionquote/"+symbol, map[string]string{"at": at, "expiry": expiry}, &chain)
	return chain, err
}

// Option returns one contract, or nil when there is no data.
func (fa *FlashAlphaData) Option(symbol, at, expiry string, strike float64, optType string) (*OptionQuote, error) {
	params := map[string]string{"at": at, "expiry": expiry, "strike": strikeText(strike), "type": optType}
	var q OptionQuote
	ok, err := fa.Get("/v1/optionquote/"+symbol, params, &q)
	if !ok || err != nil {
		return nil, err
	}
	return &q, nil
}

// Snapshot is everything the rules can read for one decision time.
// Pointer fields are nil when the API returned null or left the field out, so a missing value is never
// mistaken for zero.
type Snapshot struct {
	Date string
	At   string
	Spot float64
	// /v1/exposure/summary
	GammaRegime *string // "positive_gamma" or "negative_gamma"
	NetGEX      *float64
	GammaFlip   *float64
	// /v1/vrp
	ATMIV         *float64 // vol points
	RV20d         *float64
	VRP20d        *float64 // implied minus 20-day realized, vol points
	VRPPercentile *float64
	VRPZ          *float64
	HarvestScore  *float64
}

// RegimeIs reports whether the snapshot's dealer gamma regime equals r.
func (s Snapshot) RegimeIs(r string) bool {
	return s.GammaRegime != nil && *s.GammaRegime == r
}

// buildSnapshot collects everything the rules can read for one decision time. Returns nil on a non-trading day.
//
// Add fields from any other FlashAlpha endpoint here (exposure levels, narrative, max pain, 0DTE, surface...).
func buildSnapshot(fa DataSource, symbol, at string) (*Snapshot, error) {
	quote, err := fa.StockQuote(symbol, at)
	if err != nil {
		return nil, err
	}
	if quote == nil || !strings.HasPrefix(quote.LastUpdate, at[:10]) {
		return nil, nil // weekend or market holiday: the archive returned an earlier session's quote
	}
	if quote.Mid == nil {
		return nil, fmt.Errorf("stock quote for %s at %s has no mid price", symbol, at)
	}
	ex, err := fa.ExposureSummary(symbol, at)
	if err != nil {
		return nil, err
	}
	v, err := fa.VRP(symbol, at)
	if err != nil {
		return nil, err
	}
	snap := &Snapshot{Date: at[:10], At: at, Spot: *quote.Mid}
	if ex != nil {
		snap.GammaRegime = ex.Regime
		snap.GammaFlip = ex.GammaFlip
		if ex.Exposures != nil {
			snap.NetGEX = ex.Exposures.NetGEX
		}
	}
	if v != nil {
		if c := v.Core; c != nil {
			snap.ATMIV, snap.RV20d, snap.VRP20d = c.ATMIV, c.RV20d, c.VRP20d
			snap.VRPPercentile, snap.VRPZ = c.Percentile, c.ZScore
		}
		if v.GexConditioned != nil {
			snap.HarvestScore = v.GexConditioned.HarvestScore
		}
	}
	return snap, nil
}

// ============================================================================================================
// STRATEGY: edit these three functions
// ============================================================================================================

// entryRules is the ENTRY RULES. Return true to open a position at this decision time.
//
// snap is today's snapshot (see buildSnapshot); history holds the earlier sessions' snapshots, oldest first.
// Default: implied vol above 20-day realized (VRP > 0) and dealers long gamma for 2 sessions running.
func entryRules(snap Snapshot, history []Snapshot, cfg Config) bool {
	if len(history) == 0 {
		return false // need yesterday's regime too
	}
	yesterday := history[len(history)-1]
	return snap.VRP20d != nil && *snap.VRP20d > 0 &&
		yesterday.RegimeIs("positive_gamma") && snap.RegimeIs("positive_gamma")
}

// selectLegs is WHICH CONTRACTS. Return the legs to trade, Side -1 = sell, +1 = buy; nil means no trade.
//
// Default: put credit spread - sell the put closest to cfg.ShortDelta, buy the put cfg.Width points lower.
func selectLegs(chain []OptionQuote, snap Snapshot, cfg Config) []Leg {
	var puts []OptionQuote
	for _, o := range chain {
		if o.Type == "P" && o.Delta != nil && o.Bid != nil && *o.Bid > 0 {
			puts = append(puts, o)
		}
	}
	if len(puts) == 0 {
		return nil
	}
	distance := func(o OptionQuote) float64 { return math.Abs(math.Abs(*o.Delta) - cfg.ShortDelta) }
	short := puts[0]
	for _, o := range puts[1:] {
		if distance(o) < distance(short) {
			short = o
		}
	}
	var long *OptionQuote
	for i, o := range puts {
		if o.Strike <= short.Strike-cfg.Width && (long == nil || o.Strike > long.Strike) {
			long = &puts[i]
		}
	}
	if long == nil {
		return nil
	}
	return []Leg{
		{Type: "P", Strike: short.Strike, Expiry: short.Expiry, Side: -1},
		{Type: "P", Strike: long.Strike, Expiry: long.Expiry, Side: +1},
	}
}

// exitRules is the EXIT RULES. Return a reason string to close the position at this decision time, or "" to hold.
//
// pos.PnLMid is the open profit of the whole position in dollars at mid. pos.Credit is the net premium taken in,
// after commission: positive for a credit, negative for a debit, so the targets below use its size.
// Default: take 50% of the credit, stop at 2x the credit, close at 21 days left, or close after 2 sessions
// of negative dealer gamma.
func exitRules(pos *Position, snap Snapshot, history []Snapshot, cfg Config) string {
	if pos.PnLMid >= cfg.ProfitTarget*math.Abs(pos.Credit) {
		return "profit_target"
	}
	if pos.PnLMid <= -cfg.StopLoss*math.Abs(pos.Credit) {
		return "stop_loss"
	}
	if pos.DTE(snap.Date) <= cfg.ExitDTE {
		return "time_exit"
	}
	if len(history) > 0 && history[len(history)-1].RegimeIs("negative_gamma") && snap.RegimeIs("negative_gamma") {
		return "negative_gamma_2_sessions"
	}
	return ""
}

// ============================================================================================================
// Engine (no changes needed below to run a new strategy)
// ============================================================================================================

// Leg is one option contract in a position.
type Leg struct {
	Type   string // "C" or "P"
	Strike float64
	Expiry string // YYYY-MM-DD
	Side   int    // -1 = sell, +1 = buy
}

// Position is an open trade.
type Position struct {
	Legs      []Leg
	EntryDate string
	Credit    float64       // net premium for the whole position in dollars after commission (negative = debit)
	Marks     []OptionQuote // last quote per leg; always has a bid and an ask
	PnLMid    float64
}

// Expiry is the earliest leg expiry.
func (p *Position) Expiry() string {
	e := p.Legs[0].Expiry
	for _, leg := range p.Legs[1:] {
		if leg.Expiry < e {
			e = leg.Expiry
		}
	}
	return e
}

// DTE is the number of calendar days from date to the position's expiry.
func (p *Position) DTE(date string) int {
	return daysBetween(date, p.Expiry())
}

// fillPrice is the price for buying (side +1) or selling (side -1) one contract: mid moved toward the ask or bid.
// The quote must have a bid and an ask.
//
// The float64(...) conversions around products stop the compiler fusing them into a multiply-add (Go does
// this on arm64), which would change the last bit of the result and break exact agreement with Python.
func fillPrice(q OptionQuote, side int, slippage float64) float64 {
	bid, ask := *q.Bid, *q.Ask
	mid := (bid + ask) / 2
	if side > 0 {
		return mid + float64(slippage*(ask-mid))
	}
	return mid - float64(slippage*(mid-bid))
}

// findExpiry returns the expiry closest to cfg.TargetDTE inside [MinDTE, MaxDTE] that has a chain
// (Friday, else Thursday), with its chain. It returns "" when none has data.
func findExpiry(fa DataSource, symbol, at string, cfg Config) (string, []OptionQuote, error) {
	today := mustDate(at[:10])
	var fridays []time.Time
	for d := cfg.MinDTE; d <= cfg.MaxDTE; d++ {
		if day := today.AddDate(0, 0, d); day.Weekday() == time.Friday {
			fridays = append(fridays, day)
		}
	}
	away := func(d time.Time) int { return absInt(int(d.Sub(today).Hours()/24) - cfg.TargetDTE) }
	sort.SliceStable(fridays, func(i, j int) bool { return away(fridays[i]) < away(fridays[j]) })
	for _, friday := range fridays {
		for _, day := range []time.Time{friday, friday.AddDate(0, 0, -1)} { // Thursday when the Friday is a holiday
			expiry := day.Format(dateLayout)
			chain, err := fa.Chain(symbol, at, expiry)
			if err != nil {
				return "", nil, err
			}
			if len(chain) > 0 {
				return expiry, chain, nil
			}
		}
	}
	return "", nil, nil
}

// tradingDays lists the weekdays from start to end inclusive. Holidays are skipped later by buildSnapshot.
func tradingDays(start, end string) ([]string, error) {
	day, err := time.Parse(dateLayout, start)
	if err != nil {
		return nil, fmt.Errorf("bad -start: %w", err)
	}
	stop, err := time.Parse(dateLayout, end)
	if err != nil {
		return nil, fmt.Errorf("bad -end: %w", err)
	}
	var days []string
	for ; !day.After(stop); day = day.AddDate(0, 0, 1) {
		if wd := day.Weekday(); wd != time.Saturday && wd != time.Sunday {
			days = append(days, day.Format(dateLayout))
		}
	}
	return days, nil
}

// Trade is one row of trades.csv. Money is rounded to cents.
type Trade struct {
	EntryDate  string
	ExitDate   string
	Expiry     string
	Legs       string
	Credit     float64
	PnL        float64
	ExitReason string
	DaysHeld   int
}

// EquityRow is one row of equity.csv. Money is rounded to cents.
type EquityRow struct {
	Date          string
	Realized      float64
	Unrealized    float64
	Equity        float64
	OpenPositions int
	GammaRegime   *string
	VRP20d        *float64
}

func run(cfg Config, fa DataSource, outDir string) (Summary, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Summary{}, err
	}
	days, err := tradingDays(cfg.Start, cfg.End)
	if err != nil {
		return Summary{}, err
	}
	n := float64(cfg.Contracts)
	var open []*Position
	var trades []Trade
	var equity []EquityRow
	var history []Snapshot
	realized := 0.0
	for _, date := range days {
		at := date + "T" + cfg.DecisionTime + ":00"
		snapPtr, err := buildSnapshot(fa, cfg.Symbol, at)
		if err != nil {
			return Summary{}, err
		}
		if snapPtr == nil {
			continue
		}
		snap := *snapPtr

		// 1. Mark and exit open positions
		var stillOpen []*Position
		for _, pos := range open {
			valueMid := 0.0
			onExpiry := pos.DTE(date) <= 0 // expiry day, or the first session after it when that day had no data
			for i, leg := range pos.Legs {
				if !onExpiry { // on expiry the position settles at intrinsic, so no fresh quote is needed
					q, err := fa.Option(cfg.Symbol, at, leg.Expiry, leg.Strike, leg.Type)
					if err != nil {
						return Summary{}, err
					}
					if q != nil && q.Bid != nil && q.Ask != nil && *q.Ask > 0 {
						pos.Marks[i] = *q
					}
				}
				mid := (*pos.Marks[i].Bid + *pos.Marks[i].Ask) / 2
				valueMid += float64(float64(leg.Side) * mid * 100 * n)
			}
			pos.PnLMid = pos.Credit + valueMid // credit received plus the (negative) value of what we hold
			reason := "expiry"
			if !onExpiry {
				reason = exitRules(pos, snap, history, cfg)
			}
			if reason == "" {
				stillOpen = append(stillOpen, pos)
				continue
			}
			closeValues := make([]float64, len(pos.Legs))
			costs := 0.0
			if onExpiry { // settle at intrinsic value against the spot at the decision time
				for i, leg := range pos.Legs {
					intrinsic := snap.Spot - leg.Strike
					if leg.Type == "P" {
						intrinsic = leg.Strike - snap.Spot
					}
					closeValues[i] = float64(leg.Side) * pyMax(0.0, intrinsic)
				}
			} else {
				for i, leg := range pos.Legs {
					closeValues[i] = float64(leg.Side) * fillPrice(pos.Marks[i], -leg.Side, cfg.Slippage)
				}
				costs = float64(cfg.Commission * float64(len(pos.Legs)) * n)
			}
			pnl := pos.Credit + float64(pySum(closeValues)*100*n) - costs
			realized += pnl
			trades = append(trades, Trade{
				EntryDate:  pos.EntryDate,
				ExitDate:   date,
				Expiry:     pos.Expiry(),
				Legs:       legsText(pos.Legs),
				Credit:     pyRound(pos.Credit, 2),
				PnL:        pyRound(pnl, 2),
				ExitReason: reason,
				DaysHeld:   daysBetween(pos.EntryDate, date),
			})
		}
		open = stillOpen

		// 2. Entries
		if len(open) < cfg.MaxPositions && entryRules(snap, history, cfg) {
			pos, err := openPosition(fa, at, snap, cfg)
			if err != nil {
				return Summary{}, err
			}
			if pos != nil {
				open = append(open, pos)
			}
		}

		pnls := make([]float64, len(open))
		for i, p := range open {
			pnls[i] = p.PnLMid
		}
		unrealized := pySum(pnls)
		equity = append(equity, EquityRow{
			Date:          date,
			Realized:      pyRound(realized, 2),
			Unrealized:    pyRound(unrealized, 2),
			Equity:        pyRound(realized+unrealized, 2),
			OpenPositions: len(open),
			GammaRegime:   snap.GammaRegime,
			VRP20d:        snap.VRP20d,
		})
		history = append(history, snap)
	}

	if err := writeTrades(filepath.Join(outDir, "trades.csv"), trades); err != nil {
		return Summary{}, err
	}
	if err := writeEquity(filepath.Join(outDir, "equity.csv"), equity); err != nil {
		return Summary{}, err
	}
	return summarize(trades, equity), nil
}

// openPosition picks an expiry and legs and fills them on the chain quotes. It returns nil when there is no trade.
func openPosition(fa DataSource, at string, snap Snapshot, cfg Config) (*Position, error) {
	_, chain, err := findExpiry(fa, cfg.Symbol, at, cfg)
	if err != nil || len(chain) == 0 {
		return nil, err
	}
	legs := selectLegs(chain, snap, cfg)
	if len(legs) == 0 {
		return nil, nil
	}
	type contract struct {
		Type   string
		Strike float64
		Expiry string
	}
	byKey := make(map[contract]OptionQuote, len(chain))
	for _, o := range chain {
		byKey[contract{o.Type, o.Strike, o.Expiry}] = o // a duplicate row replaces the earlier one, as in Python
	}
	var missing []Leg
	for _, leg := range legs {
		if _, ok := byKey[contract{leg.Type, leg.Strike, leg.Expiry}]; !ok {
			missing = append(missing, leg)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("selectLegs must pick contracts from the chain it receives: %+v", missing)
	}
	n := float64(cfg.Contracts)
	quotes := make([]OptionQuote, len(legs))
	openValues := make([]float64, len(legs))
	mids := make([]float64, len(legs))
	for i, leg := range legs {
		q := byKey[contract{leg.Type, leg.Strike, leg.Expiry}]
		if q.Bid == nil || q.Ask == nil {
			return nil, fmt.Errorf("no bid/ask for %s%s %s at %s", leg.Type, strikeText(leg.Strike), leg.Expiry, at)
		}
		if _, err := time.Parse(dateLayout, leg.Expiry); err != nil {
			return nil, fmt.Errorf("leg expiry %q is not YYYY-MM-DD", leg.Expiry)
		}
		quotes[i] = q
		openValues[i] = float64(leg.Side) * fillPrice(q, leg.Side, cfg.Slippage)
		mids[i] = float64(leg.Side) * (*q.Bid + *q.Ask) / 2
	}
	credit := float64(-pySum(openValues)*100*n) - float64(cfg.Commission*float64(len(legs))*n)
	return &Position{
		Legs:      legs,
		EntryDate: snap.Date,
		Credit:    credit,
		Marks:     quotes,
		PnLMid:    credit + float64(pySum(mids)*100*n),
	}, nil
}

func legsText(legs []Leg) string {
	parts := make([]string, len(legs))
	for i, l := range legs {
		sign := "-"
		if l.Side > 0 {
			sign = "+"
		}
		parts[i] = sign + l.Type + strikeText(l.Strike)
	}
	return strings.Join(parts, " ")
}

func writeTrades(path string, trades []Trade) error {
	rows := make([][]string, 0, len(trades)+1)
	if len(trades) > 0 {
		rows = append(rows, []string{"entry_date", "exit_date", "expiry", "legs", "credit", "pnl", "exit_reason", "days_held"})
	}
	for _, t := range trades {
		rows = append(rows, []string{t.EntryDate, t.ExitDate, t.Expiry, t.Legs, pyRepr(t.Credit), pyRepr(t.PnL),
			t.ExitReason, strconv.Itoa(t.DaysHeld)})
	}
	return writeCSV(path, rows)
}

func writeEquity(path string, equity []EquityRow) error {
	rows := make([][]string, 0, len(equity)+1)
	if len(equity) > 0 {
		rows = append(rows, []string{"date", "realized", "unrealized", "equity", "open_positions", "gamma_regime", "vrp_20d"})
	}
	for _, e := range equity {
		regime := ""
		if e.GammaRegime != nil {
			regime = *e.GammaRegime
		}
		vrp := ""
		if e.VRP20d != nil {
			vrp = formatAPINumber(*e.VRP20d)
		}
		rows = append(rows, []string{e.Date, pyRepr(e.Realized), pyRepr(e.Unrealized), pyRepr(e.Equity),
			strconv.Itoa(e.OpenPositions), regime, vrp})
	}
	return writeCSV(path, rows)
}

// writeCSV writes rows with CRLF line endings, like Python's csv module. No rows gives an empty file.
func writeCSV(path string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	w.UseCRLF = true
	if err := w.WriteAll(rows); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Summary is the run summary printed at the end. Nil fields print as null.
type Summary struct {
	Sessions    int         `json:"sessions"`
	Trades      int         `json:"trades"`
	WinRate     *pyFloat    `json:"win_rate"`
	TotalPnL    pyFloat     `json:"total_pnl"`
	AvgWin      *pyFloat    `json:"avg_win"`
	AvgLoss     *pyFloat    `json:"avg_loss"`
	MaxDrawdown pyFloat     `json:"max_drawdown"`
	Sharpe      *pyFloat    `json:"sharpe"`
	ExitReasons exitReasons `json:"exit_reasons"`
	OpenAtEnd   int         `json:"open_at_end"`
}

func summarize(trades []Trade, equity []EquityRow) Summary {
	var pnls, wins, losses []float64
	var reasons exitReasons
	for _, t := range trades {
		pnls = append(pnls, t.PnL)
		if t.PnL > 0 {
			wins = append(wins, t.PnL)
		} else {
			losses = append(losses, t.PnL) // break-even trades count as losses
		}
		reasons = reasons.add(t.ExitReason)
	}
	curve := make([]float64, len(equity))
	for i, e := range equity {
		curve[i] = e.Equity
	}
	daily := make([]float64, len(curve))
	prev := 0.0
	for i, v := range curve {
		daily[i] = v - prev
		prev = v
	}
	sd := 0.0
	if len(daily) > 1 {
		mean := pySum(daily) / float64(len(daily))
		squares := make([]float64, len(daily))
		for i, x := range daily {
			squares[i] = float64((x - mean) * (x - mean))
		}
		sd = math.Sqrt(pySum(squares) / float64(len(daily)-1))
	}
	peak, maxDD := 0.0, 0.0
	for _, v := range curve {
		peak = pyMax(peak, v)
		maxDD = pyMax(maxDD, peak-v)
	}

	s := Summary{
		Sessions:    len(equity),
		Trades:      len(trades),
		TotalPnL:    pyFloat(pyRound(pySum(pnls), 2)),
		MaxDrawdown: pyFloat(pyRound(maxDD, 2)),
		ExitReasons: reasons,
	}
	if len(pnls) > 0 {
		s.WinRate = roundedPtr(float64(len(wins))/float64(len(pnls)), 3)
	}
	if len(wins) > 0 {
		s.AvgWin = roundedPtr(pySum(wins)/float64(len(wins)), 2)
	}
	if len(losses) > 0 {
		s.AvgLoss = roundedPtr(pySum(losses)/float64(len(losses)), 2)
	}
	if sd > 0 {
		s.Sharpe = roundedPtr(pySum(daily)/float64(len(daily))/sd*math.Sqrt(252), 2)
	}
	if len(equity) > 0 {
		s.OpenAtEnd = equity[len(equity)-1].OpenPositions
	}
	return s
}

// exitReasons counts trades per exit reason, in order of first appearance.
type exitReasons []reasonCount

type reasonCount struct {
	Reason string
	Count  int
}

func (r exitReasons) add(reason string) exitReasons {
	for i := range r {
		if r[i].Reason == reason {
			r[i].Count++
			return r
		}
	}
	return append(r, reasonCount{reason, 1})
}

// MarshalJSON writes the counts as one JSON object, keeping their order.
func (r exitReasons) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, rc := range r {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(pyJSONString(rc.Reason) + ":" + strconv.Itoa(rc.Count))
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// ============================================================================================================
// Number formatting and arithmetic that match Python exactly
// ============================================================================================================

// pyFloat marshals to JSON the way Python prints a float (58.3, 0.0, 1e-05).
type pyFloat float64

// MarshalJSON writes the value as Python's repr() would.
func (f pyFloat) MarshalJSON() ([]byte, error) {
	return []byte(pyRepr(float64(f))), nil
}

func roundedPtr(x float64, ndigits int) *pyFloat {
	f := pyFloat(pyRound(x, ndigits))
	return &f
}

// pyRound matches Python's round(x, ndigits): round the exact binary value to the nearest decimal, ties to even.
// strconv's fixed-precision formatting is correctly rounded with the same tie rule, so the results agree.
func pyRound(x float64, ndigits int) float64 {
	if math.IsInf(x, 0) || math.IsNaN(x) {
		return x
	}
	r, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'f', ndigits, 64), 64)
	return r
}

// pyRepr formats a float like Python's repr(): shortest round-trip digits, ".0" on whole numbers, and
// exponent notation below 1e-4 or from 1e16.
func pyRepr(x float64) string {
	switch {
	case math.IsNaN(x):
		return "nan"
	case math.IsInf(x, 1):
		return "inf"
	case math.IsInf(x, -1):
		return "-inf"
	}
	e := strconv.FormatFloat(x, 'e', -1, 64)
	if exp, _ := strconv.Atoi(e[strings.IndexByte(e, 'e')+1:]); exp < -4 || exp >= 16 {
		return e
	}
	s := strconv.FormatFloat(x, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// formatAPINumber prints a number taken straight from the API the way Python prints it after json.loads.
// The API writes whole numbers without a decimal point (3, not 3.0), Python keeps those as int, and so prints "3".
func formatAPINumber(x float64) string {
	if x == math.Trunc(x) && math.Abs(x) < 1e16 {
		return strconv.FormatFloat(x, 'f', 0, 64)
	}
	return pyRepr(x)
}

// pySum adds floats the way Python 3.12+ sum() does (Neumaier compensated summation), so totals agree with
// the Python version to the last bit.
func pySum(xs []float64) float64 {
	var sum, c float64
	for _, x := range xs {
		t := sum + x
		if math.Abs(sum) >= math.Abs(x) {
			c += (sum - t) + x
		} else {
			c += (x - t) + sum
		}
		sum = t
	}
	if c != 0 && !math.IsInf(c, 0) && !math.IsNaN(c) {
		sum += c
	}
	return sum
}

// pyMax matches Python's max(a, b): a unless b is strictly greater.
func pyMax(a, b float64) float64 {
	if b > a {
		return b
	}
	return a
}

// pyJSONString quotes s like Python's json.dumps (ASCII only, non-ASCII as \uXXXX).
func pyJSONString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r < 0x20 || (r > 0x7e && r <= 0xffff):
			fmt.Fprintf(&b, `\u%04x`, r)
		case r > 0xffff:
			r1, r2 := utf16.EncodeRune(r)
			fmt.Fprintf(&b, `\u%04x\u%04x`, r1, r2)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// ============================================================================================================
// Dates
// ============================================================================================================

const dateLayout = "2006-01-02"

// mustDate parses a YYYY-MM-DD date. Dates reaching it have already been validated.
func mustDate(s string) time.Time {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		panic(fmt.Sprintf("invalid date %q: %v", s, err))
	}
	return t
}

// daysBetween is the number of calendar days from a to b (both YYYY-MM-DD).
func daysBetween(a, b string) int {
	return int(mustDate(b).Sub(mustDate(a)).Hours() / 24)
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ============================================================================================================
// Command line
// ============================================================================================================

func main() {
	cfg := defaultConfig()
	flag.StringVar(&cfg.Symbol, "symbol", cfg.Symbol, "underlying symbol")
	flag.StringVar(&cfg.Start, "start", cfg.Start, "first session, YYYY-MM-DD")
	flag.StringVar(&cfg.End, "end", cfg.End, "last session, YYYY-MM-DD")
	flag.StringVar(&cfg.DecisionTime, "decision-time", cfg.DecisionTime, "ET time of every decision and fill, HH:MM")
	flag.IntVar(&cfg.TargetDTE, "target-dte", cfg.TargetDTE, "pick the expiry closest to this many calendar days out")
	flag.IntVar(&cfg.MinDTE, "min-dte", cfg.MinDTE, "shortest expiry allowed, calendar days")
	flag.IntVar(&cfg.MaxDTE, "max-dte", cfg.MaxDTE, "longest expiry allowed, calendar days")
	flag.Float64Var(&cfg.ShortDelta, "short-delta", cfg.ShortDelta, "sell the put closest to this absolute delta")
	flag.Float64Var(&cfg.Width, "width", cfg.Width, "buy the put this many points lower")
	flag.IntVar(&cfg.Contracts, "contracts", cfg.Contracts, "contracts per leg")
	flag.IntVar(&cfg.MaxPositions, "max-positions", cfg.MaxPositions, "open positions allowed at once")
	flag.Float64Var(&cfg.ProfitTarget, "profit-target", cfg.ProfitTarget, "close at this fraction of the entry credit captured")
	flag.Float64Var(&cfg.StopLoss, "stop-loss", cfg.StopLoss, "close when the loss reaches this multiple of the entry credit")
	flag.IntVar(&cfg.ExitDTE, "exit-dte", cfg.ExitDTE, "close with this many calendar days left")
	flag.Float64Var(&cfg.Slippage, "slippage", cfg.Slippage, "0 = fill at mid, 1 = fill at the bid or ask")
	flag.Float64Var(&cfg.Commission, "commission", cfg.Commission, "dollars per contract per leg, each side")
	out := flag.String("out", "results", "folder for trades.csv and equity.csv")
	cacheDir := flag.String("cache", ".fa_cache", "folder for cached API responses")
	flag.Parse()

	fa, err := NewFlashAlphaData(os.Getenv("FLASHALPHA_API_KEY"), *cacheDir, baseURL)
	if err != nil {
		fail(err)
	}
	summary, err := run(cfg, fa, *out)
	if err != nil {
		fail(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(summary); err != nil {
		fail(err)
	}
	fmt.Printf("API calls this run: %d. Trades: %s. Equity: %s\n",
		fa.Calls, filepath.Join(*out, "trades.csv"), filepath.Join(*out, "equity.csv"))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

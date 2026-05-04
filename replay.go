package flashalphahistorical

import (
	"context"
	"errors"
	"sort"
	"time"
)

// fullCloseHolidays is the set of NYSE full-close holidays 2018-2026 (UTC date keys).
// Early-close days (1pm) are NOT here — the API returns valid minute-level data
// up to the actual close, so they don't need to be skipped.
var fullCloseHolidays = func() map[string]struct{} {
	dates := []string{
		// 2018
		"2018-01-01", "2018-01-15", "2018-02-19", "2018-03-30", "2018-05-28",
		"2018-07-04", "2018-09-03", "2018-11-22", "2018-12-05", "2018-12-25",
		// 2019
		"2019-01-01", "2019-01-21", "2019-02-18", "2019-04-19", "2019-05-27",
		"2019-07-04", "2019-09-02", "2019-11-28", "2019-12-25",
		// 2020
		"2020-01-01", "2020-01-20", "2020-02-17", "2020-04-10", "2020-05-25",
		"2020-07-03", "2020-09-07", "2020-11-26", "2020-12-25",
		// 2021
		"2021-01-01", "2021-01-18", "2021-02-15", "2021-04-02", "2021-05-31",
		"2021-07-05", "2021-09-06", "2021-11-25", "2021-12-24",
		// 2022
		"2022-01-17", "2022-02-21", "2022-04-15", "2022-05-30", "2022-06-20",
		"2022-07-04", "2022-09-05", "2022-11-24", "2022-12-26",
		// 2023
		"2023-01-02", "2023-01-16", "2023-02-20", "2023-04-07", "2023-05-29",
		"2023-06-19", "2023-07-04", "2023-09-04", "2023-11-23", "2023-12-25",
		// 2024
		"2024-01-01", "2024-01-15", "2024-02-19", "2024-03-29", "2024-05-27",
		"2024-06-19", "2024-07-04", "2024-09-02", "2024-11-28", "2024-12-25",
		// 2025
		"2025-01-01", "2025-01-09", "2025-01-20", "2025-02-17", "2025-04-18",
		"2025-05-26", "2025-06-19", "2025-07-04", "2025-09-01", "2025-11-27",
		"2025-12-25",
		// 2026
		"2026-01-01", "2026-01-19", "2026-02-16", "2026-04-03", "2026-05-25",
		"2026-06-19", "2026-07-03", "2026-09-07", "2026-11-26", "2026-12-25",
	}
	out := make(map[string]struct{}, len(dates))
	for _, d := range dates {
		out[d] = struct{}{}
	}
	return out
}()

// IsTradingDay reports whether t falls on a NYSE trading day (weekday and not
// a known full-close holiday). Time-of-day is ignored.
func IsTradingDay(t time.Time) bool {
	switch t.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	_, holiday := fullCloseHolidays[t.Format(AtFormatDate)]
	return !holiday
}

// IterDays returns one time.Time per trading day in [start, end] inclusive,
// stamped at 16:00 (the API's session close). Both bounds are interpreted as
// dates — time-of-day on the inputs is ignored.
func IterDays(start, end time.Time) []time.Time {
	s := truncateToDate(start)
	e := truncateToDate(end)
	var out []time.Time
	for d := s; !d.After(e); d = d.AddDate(0, 0, 1) {
		if IsTradingDay(d) {
			out = append(out, time.Date(d.Year(), d.Month(), d.Day(), 16, 0, 0, 0, d.Location()))
		}
	}
	return out
}

// IterMinutes returns ET wall-clock minute timestamps inside RTH for every
// trading day in [start, end]. Default cadence is 1 minute (390 stamps/day).
// Pass stepMinutes to coarsen the cadence.
func IterMinutes(start, end time.Time, stepMinutes int) []time.Time {
	if stepMinutes <= 0 {
		panic("flashalpha-historical: stepMinutes must be positive")
	}
	var out []time.Time
	for _, dayClose := range IterDays(start, end) {
		d := truncateToDate(dayClose)
		open := time.Date(d.Year(), d.Month(), d.Day(), 9, 30, 0, 0, d.Location())
		close := time.Date(d.Year(), d.Month(), d.Day(), 16, 0, 0, 0, d.Location())
		for cur := open; !cur.After(close); cur = cur.Add(time.Duration(stepMinutes) * time.Minute) {
			out = append(out, cur)
		}
	}
	return out
}

func truncateToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// AtEndpoint is the function signature for any client method that takes a
// symbol + ET wall-clock `at` string. Used by Replay and Backtester.
type AtEndpoint func(ctx context.Context, c *Client, symbol, at string) (map[string]interface{}, error)

// EndpointStockSummary calls (*Client).StockSummary.
func EndpointStockSummary(ctx context.Context, c *Client, symbol, at string) (map[string]interface{}, error) {
	return c.StockSummary(ctx, symbol, at)
}

// EndpointExposureSummary calls (*Client).ExposureSummary.
func EndpointExposureSummary(ctx context.Context, c *Client, symbol, at string) (map[string]interface{}, error) {
	return c.ExposureSummary(ctx, symbol, at)
}

// EndpointVrp calls (*Client).Vrp.
func EndpointVrp(ctx context.Context, c *Client, symbol, at string) (map[string]interface{}, error) {
	return c.Vrp(ctx, symbol, at)
}

// ReplayStep is one yielded step of a replay run.
type ReplayStep struct {
	At       string
	Response map[string]interface{}
}

// ReplayOptions configures a Replay call.
type ReplayOptions struct {
	// SkipMissing, when true (default), silently skips 404-class data gaps
	// (no_data, symbol_not_found, insufficient_data). Other errors abort.
	SkipMissing bool
	// OnError is invoked when SkipMissing swallows an error. Optional.
	OnError func(at time.Time, err error)
}

// Replay walks an endpoint over a sequence of timestamps, returning one step
// per successful call. By default skips data-gap days silently. The slice is
// returned in encounter order; for streaming use ReplayChan.
func Replay(
	ctx context.Context,
	client *Client,
	endpoint AtEndpoint,
	symbol string,
	timestamps []time.Time,
	opts *ReplayOptions,
) ([]ReplayStep, error) {
	if opts == nil {
		opts = &ReplayOptions{SkipMissing: true}
	}
	var out []ReplayStep
	for _, ts := range timestamps {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		atStr := FormatAt(ts)
		resp, err := endpoint(ctx, client, symbol, atStr)
		if err != nil {
			if opts.SkipMissing && isDataGap(err) {
				if opts.OnError != nil {
					opts.OnError(ts, err)
				}
				continue
			}
			return out, err
		}
		out = append(out, ReplayStep{At: atStr, Response: resp})
	}
	return out, nil
}

// ReplayChan is a channel-yielding variant of Replay — useful for very long
// loops where you want to start consuming snapshots before the run finishes.
// The returned channel is closed when the run completes; any error is sent
// on errCh exactly once before close.
func ReplayChan(
	ctx context.Context,
	client *Client,
	endpoint AtEndpoint,
	symbol string,
	timestamps []time.Time,
	opts *ReplayOptions,
) (<-chan ReplayStep, <-chan error) {
	out := make(chan ReplayStep)
	errCh := make(chan error, 1)
	go func() {
		defer close(out)
		steps, err := Replay(ctx, client, endpoint, symbol, timestamps, opts)
		for _, s := range steps {
			select {
			case <-ctx.Done():
				return
			case out <- s:
			}
		}
		if err != nil {
			errCh <- err
		}
		close(errCh)
	}()
	return out, errCh
}

func isDataGap(err error) bool {
	var ndErr *NoDataError
	var snfErr *SymbolNotFoundError
	var idErr *InsufficientDataError
	return errors.As(err, &ndErr) || errors.As(err, &snfErr) || errors.As(err, &idErr)
}

// BacktestStep is one snapshot + the strategy output for that step.
type BacktestStep struct {
	At       string
	Snapshot map[string]interface{}
	Output   interface{}
}

// Strategy is the user-supplied callback for each backtest step.
type Strategy func(at string, snap map[string]interface{}) interface{}

// Backtester is a minimal orchestrator — pulls a snapshot per step, feeds it
// to the strategy, collects the output. No fill simulation, no portfolio
// accounting.
type Backtester struct {
	Client      *Client
	Endpoint    AtEndpoint
	Symbol      string
	SkipMissing bool
}

// NewBacktester creates a Backtester with sensible defaults (StockSummary, SPY).
func NewBacktester(client *Client) *Backtester {
	return &Backtester{
		Client:      client,
		Endpoint:    EndpointStockSummary,
		Symbol:      "SPY",
		SkipMissing: true,
	}
}

// Run walks the timestamps, calling the endpoint and the strategy for each.
func (b *Backtester) Run(ctx context.Context, timestamps []time.Time, strategy Strategy) ([]BacktestStep, error) {
	steps, err := Replay(ctx, b.Client, b.Endpoint, b.Symbol, timestamps,
		&ReplayOptions{SkipMissing: b.SkipMissing})
	if err != nil {
		return nil, err
	}
	results := make([]BacktestStep, 0, len(steps))
	for _, s := range steps {
		results = append(results, BacktestStep{
			At:       s.At,
			Snapshot: s.Response,
			Output:   strategy(s.At, s.Response),
		})
	}
	return results, nil
}

// SortByAt sorts a slice of BacktestStep by At (lexicographic = chronological
// for ISO timestamps).
func SortByAt(steps []BacktestStep) {
	sort.SliceStable(steps, func(i, j int) bool { return steps[i].At < steps[j].At })
}

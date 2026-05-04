// Package flashalphahistorical provides a Go client for the FlashAlpha Historical API.
//
// Point-in-time replay of every live FlashAlpha analytics endpoint. Every
// analytics method takes a required `at` value (string or time.Time) and
// returns the same response shape as the live API at that moment in history.
//
// Base URL: https://historical.flashalpha.com
package flashalphahistorical

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// DefaultBaseURL is the production Historical API base URL.
const DefaultBaseURL = "https://historical.flashalpha.com"

const defaultTimeout = 60 * time.Second

// AtFormatMinute is the canonical minute-resolution layout used by the API.
const AtFormatMinute = "2006-01-02T15:04:05"

// AtFormatDate is the canonical date-resolution layout (defaults to 16:00 ET on the API).
const AtFormatDate = "2006-01-02"

// FormatAt formats a time.Time as the ET wall-clock string the API expects.
// The clock is taken as-is — callers should construct ETs in the ET frame.
func FormatAt(t time.Time) string { return t.Format(AtFormatMinute) }

// seg URL-escapes a single path segment.
func seg(s string) string { return url.PathEscape(s) }

// Client is a thread-safe HTTP client for the FlashAlpha Historical API.
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// NewClient creates a Client using the default base URL.
func NewClient(apiKey string) *Client {
	return NewClientWithURL(apiKey, DefaultBaseURL)
}

// NewClientWithURL creates a Client with a custom base URL.
func NewClientWithURL(apiKey, baseURL string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		http:    &http.Client{Timeout: defaultTimeout},
	}
}

// SetHTTPClient swaps in a custom *http.Client (timeout, transport, etc.).
func (c *Client) SetHTTPClient(h *http.Client) { c.http = h }

// ── internal ─────────────────────────────────────────────────────────────────

func (c *Client) get(ctx context.Context, path string, params url.Values) (map[string]interface{}, error) {
	rawURL := c.baseURL + path
	if len(params) > 0 {
		rawURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("flashalpha-historical: build request: %w", err)
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("flashalpha-historical: request: %w", err)
	}
	defer resp.Body.Close()

	return c.handle(resp)
}

func (c *Client) handle(resp *http.Response) (map[string]interface{}, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("flashalpha-historical: read body: %w", err)
	}

	var parsed map[string]interface{}
	if len(body) > 0 {
		_ = json.Unmarshal(body, &parsed)
	}

	if resp.StatusCode == http.StatusOK {
		if parsed == nil {
			parsed = make(map[string]interface{})
		}
		return parsed, nil
	}

	// Error path — extract code + message
	code := stringField(parsed, "error")
	message := stringField(parsed, "message")
	if message == "" {
		message = stringField(parsed, "detail")
	}
	if message == "" {
		message = string(body)
	}

	base := &APIError{
		StatusCode: resp.StatusCode,
		Code:       code,
		Message:    message,
		Response:   parsed,
	}

	switch resp.StatusCode {
	case http.StatusBadRequest:
		if code == "invalid_at" {
			return nil, &InvalidAtError{APIError: base}
		}
		return nil, base
	case http.StatusUnauthorized:
		return nil, &AuthenticationError{APIError: base}
	case http.StatusForbidden:
		return nil, &TierRestrictedError{
			APIError:     base,
			CurrentPlan:  stringField(parsed, "current_plan"),
			RequiredPlan: stringField(parsed, "required_plan"),
		}
	case http.StatusNotFound:
		switch code {
		case "no_coverage":
			return nil, &NoCoverageError{APIError: base}
		case "symbol_not_found":
			return nil, &SymbolNotFoundError{APIError: base}
		case "insufficient_data":
			return nil, &InsufficientDataError{APIError: base}
		default:
			return nil, &NoDataError{APIError: base}
		}
	case http.StatusTooManyRequests:
		retryAfter := 0
		if v := resp.Header.Get("Retry-After"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				retryAfter = n
			}
		}
		return nil, &RateLimitError{APIError: base, RetryAfter: retryAfter}
	}

	if resp.StatusCode >= 500 {
		return nil, &ServerError{APIError: base}
	}
	return nil, base
}

func stringField(m map[string]interface{}, k string) string {
	if v, ok := m[k]; ok {
		if s, ok2 := v.(string); ok2 {
			return s
		}
	}
	return ""
}

// ── helper option setters ────────────────────────────────────────────────────

// Option configures a method call (filters like `expiration`, `min_oi`, etc.).
type Option func(url.Values)

// WithExpiration adds an `expiration=YYYY-MM-DD` filter.
func WithExpiration(s string) Option {
	return func(v url.Values) { v.Set("expiration", s) }
}

// WithMinOI adds a `min_oi=N` filter.
func WithMinOI(n int) Option {
	return func(v url.Values) { v.Set("min_oi", strconv.Itoa(n)) }
}

// WithStrikeRange adds a `strike_range=X` filter (zero-DTE).
func WithStrikeRange(x float64) Option {
	return func(v url.Values) {
		v.Set("strike_range", strconv.FormatFloat(x, 'f', -1, 64))
	}
}

// WithExpiry adds an `expiry=YYYY-MM-DD` filter (option_quote).
func WithExpiry(s string) Option {
	return func(v url.Values) { v.Set("expiry", s) }
}

// WithStrike adds a `strike=X` filter (option_quote).
func WithStrike(x float64) Option {
	return func(v url.Values) {
		v.Set("strike", strconv.FormatFloat(x, 'f', -1, 64))
	}
}

// WithType adds a `type=C|Call|P|Put` filter (option_quote).
func WithType(t string) Option {
	return func(v url.Values) { v.Set("type", t) }
}

func buildAtParams(at string, opts ...Option) url.Values {
	v := url.Values{}
	v.Set("at", at)
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// ── Coverage ────────────────────────────────────────────────────────────────

// Tickers lists every symbol with historical coverage. Pass a non-empty
// symbol to get a single coverage object (returns NoCoverageError if missing).
func (c *Client) Tickers(ctx context.Context, symbol string) (map[string]interface{}, error) {
	v := url.Values{}
	if symbol != "" {
		v.Set("symbol", symbol)
	}
	return c.get(ctx, "/v1/tickers", v)
}

// ── Market Data ─────────────────────────────────────────────────────────────

// StockQuote returns stock bid/ask/mid/last at the requested minute.
func (c *Client) StockQuote(ctx context.Context, ticker, at string) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/stockquote/"+seg(ticker), url.Values{"at": []string{at}})
}

// OptionQuote returns option quote(s) + greeks + OI at the requested minute.
// Pass WithExpiry / WithStrike / WithType for filters; with all three the
// response is a single object instead of an array.
func (c *Client) OptionQuote(ctx context.Context, ticker, at string, opts ...Option) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/optionquote/"+seg(ticker), buildAtParams(at, opts...))
}

// Surface returns the 50×50 IV surface grid. May raise InsufficientDataError
// for sparse historical days.
func (c *Client) Surface(ctx context.Context, symbol, at string) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/surface/"+seg(symbol), url.Values{"at": []string{at}})
}

// ── Exposure Analytics ──────────────────────────────────────────────────────

// Gex returns gamma exposure by strike. Use WithExpiration / WithMinOI for filters.
func (c *Client) Gex(ctx context.Context, symbol, at string, opts ...Option) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/exposure/gex/"+seg(symbol), buildAtParams(at, opts...))
}

// Dex returns delta exposure by strike.
func (c *Client) Dex(ctx context.Context, symbol, at string, opts ...Option) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/exposure/dex/"+seg(symbol), buildAtParams(at, opts...))
}

// Vex returns vanna exposure by strike.
func (c *Client) Vex(ctx context.Context, symbol, at string, opts ...Option) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/exposure/vex/"+seg(symbol), buildAtParams(at, opts...))
}

// Chex returns charm exposure by strike.
func (c *Client) Chex(ctx context.Context, symbol, at string, opts ...Option) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/exposure/chex/"+seg(symbol), buildAtParams(at, opts...))
}

// ExposureSummary returns the full composite dashboard.
func (c *Client) ExposureSummary(ctx context.Context, symbol, at string) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/exposure/summary/"+seg(symbol), url.Values{"at": []string{at}})
}

// ExposureLevels returns key technical levels (gamma flip, walls, magnet).
func (c *Client) ExposureLevels(ctx context.Context, symbol, at string) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/exposure/levels/"+seg(symbol), url.Values{"at": []string{at}})
}

// Narrative returns verbal analysis + prior-day GEX comparison + VIX context.
func (c *Client) Narrative(ctx context.Context, symbol, at string) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/exposure/narrative/"+seg(symbol), url.Values{"at": []string{at}})
}

// ZeroDte returns 0DTE-specific analytics.
func (c *Client) ZeroDte(ctx context.Context, symbol, at string, opts ...Option) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/exposure/zero-dte/"+seg(symbol), buildAtParams(at, opts...))
}

// ── Max Pain ────────────────────────────────────────────────────────────────

// MaxPain returns strike-by-strike pain curve, OI breakdown, and dealer alignment.
func (c *Client) MaxPain(ctx context.Context, symbol, at string, opts ...Option) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/maxpain/"+seg(symbol), buildAtParams(at, opts...))
}

// ── Composite ───────────────────────────────────────────────────────────────

// StockSummary returns the full composite snapshot.
func (c *Client) StockSummary(ctx context.Context, symbol, at string) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/stock/"+seg(symbol)+"/summary", url.Values{"at": []string{at}})
}

// ── Volatility ──────────────────────────────────────────────────────────────

// Volatility returns the realized vol ladder + IV-RV spreads + skew + term.
func (c *Client) Volatility(ctx context.Context, symbol, at string) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/volatility/"+seg(symbol), url.Values{"at": []string{at}})
}

// AdvVolatility returns SVI parameters, variance surface, and arbitrage flags.
func (c *Client) AdvVolatility(ctx context.Context, symbol, at string) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/adv_volatility/"+seg(symbol), url.Values{"at": []string{at}})
}

// ── VRP ─────────────────────────────────────────────────────────────────────

// Vrp returns the variance-risk-premium dashboard with date-bounded percentiles.
func (c *Client) Vrp(ctx context.Context, symbol, at string) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/vrp/"+seg(symbol), url.Values{"at": []string{at}})
}

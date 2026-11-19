package flashalphahistorical

// Expiry impact and strike dependency replay (SPY, QQQ):
//
//   - ExpiryImpact / ExpiryImpactTyped         - GET /v1/exposure/expiry-impact/{symbol}?at=...
//   - StrikeDependency / StrikeDependencyTyped - GET /v1/exposure/strike-dependency/{symbol}?at=...
//
// Both typed responses share GammaContextMetadata. Requires Alpha+. Other symbols
// return 422 unsupported_symbol.

import (
	"context"
	"net/url"
	"strconv"
)

// ── Shared models ────────────────────────────────────────────────────────────

// GammaContextReason is one machine-coded reason behind DataStatus.
type GammaContextReason struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// ContractCount is the number of excluded contracts. Nil when the reason is not
	// about excluded contracts.
	ContractCount *int `json:"contract_count"`
}

// GammaContextMetadata is shared by ExpiryImpactResponse and StrikeDependencyResponse.
// It is embedded, so its fields are promoted (resp.DataStatus, resp.SnapshotID).
type GammaContextMetadata struct {
	Symbol          string  `json:"symbol"`
	UnderlyingPrice float64 `json:"underlying_price"`
	// SnapshotID is a SHA-256 fingerprint of the captured inputs; identical inputs give
	// identical IDs.
	SnapshotID     string  `json:"snapshot_id"`
	GeneratedAt    string  `json:"generated_at"`
	QuoteAt        *string `json:"quote_at"`
	GreeksOldestAt *string `json:"greeks_oldest_at"`
	GreeksNewestAt *string `json:"greeks_newest_at"`
	// CalculationVersion identifies the calculation, for example gamma-context-v2.
	CalculationVersion string `json:"calculation_version"`
	// PricingBasis names how gamma was priced, for example
	// historical_quote_repriced_dated_inputs.
	PricingBasis string `json:"pricing_basis"`
	// DataStatus is partial in normal operation (the open-interest vintage is unknown),
	// stale when an input predates the market-session freshness cutoff, and unavailable
	// when no contract qualifies.
	DataStatus string `json:"data_status"`
	// Reasons carries machine codes and excluded-contract counts.
	Reasons               []GammaContextReason `json:"reasons"`
	OiAsOf                *string              `json:"oi_as_of"`
	OiVintageStatus       string               `json:"oi_vintage_status"`
	EligibleContractCount int                  `json:"eligible_contract_count"`
	RejectedContractCount int                  `json:"rejected_contract_count"`
	ObservedMinStrike     *float64             `json:"observed_min_strike"`
	ObservedMaxStrike     *float64             `json:"observed_max_strike"`
	// GexUnit is usd_delta_notional_per_1pct_move: every *Usd field is USD per 1% move.
	GexUnit string `json:"gex_unit"`
	// SignConvention is calls_positive_puts_negative. The sign is a convention, not
	// observed dealer inventory.
	SignConvention string `json:"sign_convention"`
	// AnalysisType is expiry_removal_sensitivity (expiry impact) or strike_attribution
	// (strike dependency).
	AnalysisType string `json:"analysis_type"`
}

// GammaTotals is gross, net, call and put GEX (USD per 1% move) and a contract count.
// Gross sums magnitudes; net applies calls positive / puts negative.
type GammaTotals struct {
	GrossGexUsd   float64 `json:"gross_gex_usd"`
	NetGexUsd     float64 `json:"net_gex_usd"`
	CallGexUsd    float64 `json:"call_gex_usd"`
	PutGexUsd     float64 `json:"put_gex_usd"`
	ContractCount int     `json:"contract_count"`
	// Status is set on ExpiryImpactResponse.ExcludingExpiry only: available,
	// zero_gross_exposure or no_remaining_contracts.
	Status *string `json:"status,omitempty"`
}

// GammaStrikeImpact is one strike's current, excluded and remaining gross/net GEX.
type GammaStrikeImpact struct {
	Strike               float64 `json:"strike"`
	DistanceFromSpotPct  float64 `json:"distance_from_spot_pct"`
	CurrentGrossGexUsd   float64 `json:"current_gross_gex_usd"`
	CurrentNetGexUsd     float64 `json:"current_net_gex_usd"`
	ExcludedGrossGexUsd  float64 `json:"excluded_gross_gex_usd"`
	ExcludedNetGexUsd    float64 `json:"excluded_net_gex_usd"`
	RemainingGrossGexUsd float64 `json:"remaining_gross_gex_usd"`
	RemainingNetGexUsd   float64 `json:"remaining_net_gex_usd"`
	// ExcludedShareAtStrikePct is the excluded expiry's share of this strike's gross GEX.
	ExcludedShareAtStrikePct *float64 `json:"excluded_share_at_strike_pct"`
}

// GammaRemovalImpact compares the chain before and after the expiry is excluded.
type GammaRemovalImpact struct {
	// RemovedGrossSharePct is 100 x excluded gross / current gross. Nil when current
	// gross is zero.
	RemovedGrossSharePct  *float64 `json:"removed_gross_share_pct"`
	RetainedGrossSharePct *float64 `json:"retained_gross_share_pct"`
	// NetGexChangeUsd is remaining net minus current net (minus the excluded net).
	NetGexChangeUsd float64 `json:"net_gex_change_usd"`
	// NetSignBefore and NetSignAfter are the sign of the modeled net total before and
	// after exclusion.
	NetSignBefore  string `json:"net_sign_before"`
	NetSignAfter   string `json:"net_sign_after"`
	NetSignChanged *bool  `json:"net_sign_changed"`
	// LargestGrossReductions is the top limit strikes by gross removed.
	LargestGrossReductions []GammaStrikeImpact `json:"largest_gross_reductions"`
	// StrongestRemainingConcentrations is the top limit strikes by gross remaining.
	StrongestRemainingConcentrations []GammaStrikeImpact `json:"strongest_remaining_concentrations"`
}

// GammaContextFact is a plain-language statement with its supporting values.
type GammaContextFact struct {
	Code string `json:"code"`
	Text string `json:"text"`
	// SupportingValues is a free-form object whose keys vary by Code.
	SupportingValues map[string]interface{} `json:"supporting_values"`
}

// GammaExpiryRow is one expiry's part of the gamma at a strike
// (StrikeDependencyResponse.ByExpiry). The embedded GammaTotals carries that expiry's
// gross, net, call and put GEX and contract count.
type GammaExpiryRow struct {
	Expiry               string   `json:"expiry"`
	ValuationCutoffAt    string   `json:"valuation_cutoff_at"`
	LastTradingAt        string   `json:"last_trading_at"`
	LastTradingStatus    string   `json:"last_trading_status"`
	SettlementAt         *string  `json:"settlement_at"`
	SettlementStatus     string   `json:"settlement_status"`
	CalendarDaysToExpiry int      `json:"calendar_days_to_expiry"`
	GrossSharePct        *float64 `json:"gross_share_pct"`
	// ShareOfStrikeGrossPct is this expiry's share of the strike's gross GEX.
	ShareOfStrikeGrossPct *float64 `json:"share_of_strike_gross_pct,omitempty"`
	GammaTotals
}

// ── exposure/expiry-impact ───────────────────────────────────────────────────

// ExpiryImpactResponse is the body of GET /v1/exposure/expiry-impact/{symbol}.
type ExpiryImpactResponse struct {
	// ResponseEnvelope carries data_as_of, archive_as_of and endpoint_version.
	ResponseEnvelope
	// GammaContextMetadata carries the symbol, spot, snapshot, data status and units.
	GammaContextMetadata
	SelectedExpiry string `json:"selected_expiry"`
	// SelectionSource says how SelectedExpiry was chosen, for example nearest_active.
	SelectionSource   string   `json:"selection_source"`
	AvailableExpiries []string `json:"available_expiries"`
	// Current is the whole eligible chain.
	Current GammaTotals `json:"current"`
	// ExcludedExpiry is the selected expiry.
	ExcludedExpiry GammaTotals `json:"excluded_expiry"`
	// ExcludingExpiry is the chain without the selected expiry. Its Status is set.
	ExcludingExpiry GammaTotals        `json:"excluding_expiry"`
	Impact          GammaRemovalImpact `json:"impact"`
	// Strikes is every eligible strike with current, excluded and remaining gross/net GEX.
	Strikes []GammaStrikeImpact `json:"strikes"`
	Facts   []GammaContextFact  `json:"facts"`
}

// ExpiryImpact compares the current SPY or QQQ gamma distribution with the same
// captured inputs after excluding one expiry. Spot, Greeks and open interest are held
// fixed, so this measures how much of today's concentration depends on one maturity.
// It is not a forecast of post-expiry prices, dealer hedging or the next session's
// chain.
//
// `expiry` (yyyy-MM-dd, optional, WithExpiry) defaults to the nearest active eligible
// expiry and must be one of `available_expiries`. `limit` (1-20, default 5, WithLimit)
// sets the rows in `impact.largest_gross_reductions` and
// `impact.strongest_remaining_concentrations`; totals and `strikes` always use the
// full eligible chain.
//
// Replays the same calculation at `at` on historical.flashalpha.com. A date-only `at`
// uses 15:55 ET. Results are model reconstructions from archived quotes; original
// publication and OI revision times are unverified. Requires Alpha or higher. Other
// symbols return 422 `unsupported_symbol`. Use ExpiryImpactTyped for the typed response.
func (c *Client) ExpiryImpact(ctx context.Context, symbol, at string, opts ...Option) (map[string]interface{}, error) {
	return c.get(ctx, "/v1/exposure/expiry-impact/"+seg(symbol), buildAtParams(at, opts...))
}

// ExpiryImpactTyped is the strongly typed variant of ExpiryImpact.
func (c *Client) ExpiryImpactTyped(ctx context.Context, symbol, at string, opts ...Option) (*ExpiryImpactResponse, error) {
	raw, err := c.ExpiryImpact(ctx, symbol, at, opts...)
	if err != nil {
		return nil, err
	}
	out := &ExpiryImpactResponse{}
	if err := decodeTyped("expiry impact", raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ── exposure/strike-dependency ───────────────────────────────────────────────

// StrikeDependencyResponse is the body of GET /v1/exposure/strike-dependency/{symbol}.
type StrikeDependencyResponse struct {
	// ResponseEnvelope carries data_as_of, archive_as_of and endpoint_version.
	ResponseEnvelope
	// GammaContextMetadata carries the symbol, spot, snapshot, data status and units.
	GammaContextMetadata
	Strike              float64 `json:"strike"`
	DistanceFromSpotPct float64 `json:"distance_from_spot_pct"`
	GrossGexUsd         float64 `json:"gross_gex_usd"`
	NetGexUsd           float64 `json:"net_gex_usd"`
	CallGexUsd          float64 `json:"call_gex_usd"`
	PutGexUsd           float64 `json:"put_gex_usd"`
	ContractCount       int     `json:"contract_count"`
	// ByExpiry attributes the strike's gamma to each expiry.
	ByExpiry               []GammaExpiryRow `json:"by_expiry"`
	DominantExpiry         *string          `json:"dominant_expiry"`
	DominantExpirySharePct *float64         `json:"dominant_expiry_share_pct"`
}

// StrikeDependency attributes the gamma at one exact observed SPY or QQQ strike to its
// expiries: does the level depend on a single maturity? It reports concentration at
// that strike, not a support/resistance probability.
//
// `strike` is required and must be an exact listed strike; the request is never
// snapped to a nearby strike (404 `strike_not_found`, returned as NoDataError).
//
// Replays the same calculation at `at` on historical.flashalpha.com. A date-only `at`
// uses 15:55 ET. Results are model reconstructions from archived quotes; original
// publication and OI revision times are unverified. Requires Alpha or higher. Other
// symbols return 422 `unsupported_symbol`. Use StrikeDependencyTyped for the typed
// response.
func (c *Client) StrikeDependency(ctx context.Context, symbol, at string, strike float64) (map[string]interface{}, error) {
	v := url.Values{}
	v.Set("at", at)
	v.Set("strike", strconv.FormatFloat(strike, 'f', -1, 64))
	return c.get(ctx, "/v1/exposure/strike-dependency/"+seg(symbol), v)
}

// StrikeDependencyTyped is the strongly typed variant of StrikeDependency.
func (c *Client) StrikeDependencyTyped(ctx context.Context, symbol, at string, strike float64) (*StrikeDependencyResponse, error) {
	raw, err := c.StrikeDependency(ctx, symbol, at, strike)
	if err != nil {
		return nil, err
	}
	out := &StrikeDependencyResponse{}
	if err := decodeTyped("strike dependency", raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

package flashalphahistorical

// Typed response model for `GET /v1/exposure/summary/{symbol}?at=...`.
//
// The Historical API returns the same response shape as the live API; the
// only difference is every analytics endpoint requires an `at` query
// parameter.
//
// All numeric fields are *float64 / *string so that nil represents values
// the API could not compute (insufficient data, market closed,
// "backtest_mode" gaps, etc.).
//
// Direction casing: /v1/exposure/summary/ and /v1/exposure/zero-dte/ both
// return lowercase "buy" / "sell". Docs and typed models use that casing
// consistently.

// ExposureSummaryResponse is the typed body of
// GET /v1/exposure/summary/{symbol}?at=...
//
// One round-trip returns net dealer Greeks (gamma/delta/vanna/charm) across
// the entire chain at the requested historical minute, the gamma-flip strike,
// the dealer hedging-flow estimate at +/- 1% spot moves, verbal regime
// narratives, and a 0DTE attribution.
type ExposureSummaryResponse struct {
	// Underlying symbol echoed from the request path (e.g. "SPY").
	Symbol string `json:"symbol"`
	// Spot mid at the as-of minute, in dollars. Reference price for all
	// GEX/DEX/VEX/CHEX dollarisation.
	UnderlyingPrice *float64 `json:"underlying_price"`
	// The as-of stamp the API actually used (snapped to the available minute).
	AsOf string `json:"as_of"`
	// Note: as_of_requested exists on /v1/exposure/{gex,dex,narrative} but
	// NOT on /v1/exposure/summary. Don't add it to this struct even though it
	// would be defensive — the field genuinely isn't returned for this endpoint.
	// Strike where net dealer gamma exposure crosses zero. Spot ABOVE the
	// flip = positive-gamma regime (mean-reversion). Spot BELOW = negative-
	// gamma regime (trend amplification).
	GammaFlip *float64 `json:"gamma_flip"`
	// Dealer-positioning regime classifier. Confirmed values:
	//   "positive_gamma" | "negative_gamma" | "unknown"
	// "unknown" is returned when there's no gamma flip / no usable options data.
	Regime string `json:"regime"`
	// Net Greek totals across the entire chain. See ExposureSummaryExposures.
	Exposures *ExposureSummaryExposures `json:"exposures"`
	// Plain-English narrative for each Greek regime — safe to surface verbatim.
	Interpretation *ExposureSummaryInterpretation `json:"interpretation"`
	// Estimated dealer hedging flow at +/- 1% spot moves.
	HedgingEstimate *ExposureSummaryHedgingEstimate `json:"hedging_estimate"`
	// Same-day-expiration contribution to total GEX.
	ZeroDte *ExposureSummaryZeroDte `json:"zero_dte"`
}

// ExposureSummaryExposures aggregates net GEX/DEX/VEX/CHEX across the chain.
//
// Each value is computed as Σ greek × OI × multiplier × spot_factor over
// every contract in the chain at the historical minute. Sign convention:
// positive means dealers were net long that Greek, negative means net short.
type ExposureSummaryExposures struct {
	// Net gamma exposure in dollars per 1% spot move. Positive (dealers
	// long gamma) → moves dampened, mean-reversion likely. Negative (short
	// gamma) → moves amplified, trend-following likely.
	NetGex *float64 `json:"net_gex"`
	// Net delta exposure in dollars. Sign is the direction of the dealer
	// hedge book against options inventory.
	NetDex *float64 `json:"net_dex"`
	// Net vanna exposure in dollars per 1-vol-point. Positive = dealers
	// benefit from vol compression (vanna-driven supportive bid).
	NetVex *float64 `json:"net_vex"`
	// Net charm exposure in dollars per day. Positive = dealers must BUY
	// into close to stay neutral (supportive); negative = SELL into close.
	NetChex *float64 `json:"net_chex"`
}

// ExposureSummaryInterpretation holds the verbal gamma/vanna/charm regime
// interpretations. Generated server-side from the numeric exposures and
// macro context; safe to surface verbatim in customer-facing UIs.
type ExposureSummaryInterpretation struct {
	// E.g. "Dealers long gamma — moves dampened, mean reversion likely".
	Gamma string `json:"gamma"`
	// E.g. "Vol up = dealers buy delta — downside dampened if vol spikes".
	Vanna string `json:"vanna"`
	// E.g. "Time decay pushing dealers to sell — pressure into close".
	Charm string `json:"charm"`
}

// ExposureSummaryHedgingMove is one side (up or down) of a dealer-hedging
// estimate. Direction is "buy" or "sell" (lowercase on both this endpoint
// and zero-dte).
//
// Estimates the order flow dealers would have generated to remain delta-
// neutral if spot moved by 1% from the as-of minute. Use this as a sizing
// reference for intraday momentum / mean-reversion setups.
type ExposureSummaryHedgingMove struct {
	// Estimated shares dealers must trade. Positive = buy, negative = sell.
	// SpotUp1Pct and SpotDown1Pct are equal in magnitude with opposite signs.
	DealerSharesToTrade *float64 `json:"dealer_shares_to_trade"`
	// Lowercase "buy" or "sell" — convenience label.
	Direction string `json:"direction"`
	// |DealerSharesToTrade| × spot. Useful for cross-symbol comparison.
	NotionalUsd *float64 `json:"notional_usd"`
}

// ExposureSummaryHedgingEstimate holds the estimated dealer hedging flow at
// +/- 1% spot moves. The two sides are symmetric (equal magnitude, opposite
// signs) — linearised from net_dex.
type ExposureSummaryHedgingEstimate struct {
	// Hedging flow if spot rises 1%.
	SpotUp1Pct *ExposureSummaryHedgingMove `json:"spot_up_1pct"`
	// Hedging flow if spot falls 1%. Equal magnitude to SpotUp1Pct, opposite sign.
	SpotDown1Pct *ExposureSummaryHedgingMove `json:"spot_down_1pct"`
}

// ExposureSummaryZeroDte is the same-day-expiration contribution to total GEX.
//
// 0DTE GEX is often the dominant intraday driver — gamma compresses to a
// delta function as expiry approaches, so even a small notional 0DTE book
// can swamp the rest of the chain in dealer-flow terms.
type ExposureSummaryZeroDte struct {
	// Net GEX contribution from same-day-expiration contracts only.
	NetGex *float64 `json:"net_gex"`
	// 0DTE share of full-chain GEX as a percentage. >50% means that minute's
	// 0DTE expiry was driving the dealer book.
	PctOfTotalGex *float64 `json:"pct_of_total_gex"`
	// ISO date of the 0DTE expiry if one existed (yyyy-MM-dd).
	Expiration *string `json:"expiration"`
}

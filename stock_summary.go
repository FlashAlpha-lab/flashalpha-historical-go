package flashalphahistorical

// Typed response model for `GET /v1/stock/{symbol}/summary?at=...`.
//
// Composite snapshot of price, volatility, options flow, dealer exposure, and
// macro context for an underlying at the requested historical minute. Same
// response shape as the live API — every analytics endpoint on the historical
// API requires an `at` query parameter, and AsOf is snapped to the available
// minute.
//
// Nullability conventions:
//   - All numeric fields are *float64 / *int / *string so nil represents
//     values the API could not compute (insufficient data, market closed,
//     "backtest_mode" gaps, etc.).
//   - Top-level Exposure is nil when the symbol had no options/greeks at the
//     requested minute.
//   - Macro fields can each be nil independently when an external feed
//     (CBOE VIX/VVIX/SKEW/MOVE, Polygon SPX, FRED) was unavailable for the
//     historical minute.
//
// IMPORTANT — sign convention diff vs zero-dte:
//   On THIS endpoint, Exposure.HedgingEstimate.{SpotUp1Pct,SpotDown1Pct}
//   .DealerShares is the MAGNITUDE (always non-negative). The signed direction
//   is carried by the sibling Direction string ("buy" / "sell"). The
//   /v1/exposure/zero-dte response uses signed values for its hedging buckets
//   instead — don't conflate them.

// StockSummaryResponse is the typed body of GET /v1/stock/{symbol}/summary?at=...
type StockSummaryResponse struct {
	// Symbol is the underlying ticker echoed from the request path.
	Symbol string `json:"symbol"`
	// UnderlyingPrice is the spot mid in dollars at the as-of minute — the
	// reference price for all dollar-denominated fields below.
	UnderlyingPrice *float64 `json:"underlying_price"`
	// AsOf is the ET wall-clock timestamp the API actually used (snapped to
	// the available minute — may differ from the requested `at` value when
	// the request lands inside a gap).
	AsOf string `json:"as_of"`
	// MarketOpen is true if NYSE was open at the as-of minute.
	MarketOpen bool `json:"market_open"`
	// Price is the bid/ask/mid/last quote block. See StockSummaryPrice.
	Price *StockSummaryPrice `json:"price"`
	// Volatility is the IV / historical-vol / VRP / skew / term-structure
	// block. See StockSummaryVolatility.
	Volatility *StockSummaryVolatility `json:"volatility"`
	// OptionsFlow is the chain-wide OI + volume aggregates. See
	// StockSummaryOptionsFlow.
	OptionsFlow *StockSummaryOptionsFlow `json:"options_flow"`
	// Exposure is the dealer-Greek block. Nil when no options/greeks data
	// was available for this symbol at the as-of minute.
	Exposure *StockSummaryExposure `json:"exposure"`
	// Macro is the macro-context block. Individual fields can be nil when
	// the underlying external data source was unavailable.
	Macro *StockSummaryMacro `json:"macro"`
}

// StockSummaryPrice is the bid/ask/mid/last quote block.
type StockSummaryPrice struct {
	// Bid is the NBBO bid price at the as-of minute.
	Bid *float64 `json:"bid"`
	// Ask is the NBBO ask price.
	Ask *float64 `json:"ask"`
	// Mid is the NBBO mid (Bid + Ask) / 2 — the canonical reference price.
	Mid *float64 `json:"mid"`
	// Last is the last trade price.
	Last *float64 `json:"last"`
	// LastUpdate is the ET wall-clock timestamp of the last quote update.
	LastUpdate *string `json:"last_update"`
}

// StockSummaryVolatility is the IV / historical-vol / VRP / skew / term-
// structure block.
//
// All vol fields are PERCENT — e.g. AtmIv = 18.45 means 18.45% annualised IV,
// NOT 0.1845 decimal.
type StockSummaryVolatility struct {
	// AtmIv is the at-the-money implied volatility (annualised %, e.g. 18.45 = 18.45%).
	AtmIv *float64 `json:"atm_iv"`
	// Hv20 is the trailing 20-day realized vol (annualised %).
	Hv20 *float64 `json:"hv_20"`
	// Hv60 is the trailing 60-day realized vol (annualised %).
	Hv60 *float64 `json:"hv_60"`
	// Vrp is the variance risk premium = AtmIv - Hv20 (percentage points).
	Vrp *float64 `json:"vrp"`
	// Skew25d is the 25-delta wing skew block.
	Skew25d *StockSummarySkew25d `json:"skew_25d"`
	// IvTermStructure is the at-the-money IV at each available expiration.
	IvTermStructure []StockSummaryIvTermPoint `json:"iv_term_structure"`
}

// StockSummarySkew25d is the 25-delta wing skew block.
type StockSummarySkew25d struct {
	// Expiry is the ISO date (yyyy-MM-dd) used for the skew measurement.
	Expiry *string `json:"expiry"`
	// DaysToExpiry is calendar days from AsOf to Expiry.
	DaysToExpiry *int `json:"days_to_expiry"`
	// Put25dIv is the IV at the 25-delta put wing (annualised %).
	Put25dIv *float64 `json:"put_25d_iv"`
	// AtmIv is the at-the-money IV at this expiry (annualised %).
	AtmIv *float64 `json:"atm_iv"`
	// Call25dIv is the IV at the 25-delta call wing (annualised %).
	Call25dIv *float64 `json:"call_25d_iv"`
	// Skew25d is Put25dIv - Call25dIv in vol points.
	Skew25d *float64 `json:"skew_25d"`
	// SmileRatio is (Put25dIv + Call25dIv) / (2 * AtmIv).
	SmileRatio *float64 `json:"smile_ratio"`
}

// StockSummaryIvTermPoint is one point on the IV term structure curve.
type StockSummaryIvTermPoint struct {
	// Expiry is the ISO date (yyyy-MM-dd) of this expiration.
	Expiry *string `json:"expiry"`
	// Iv is the at-the-money IV at this expiry (annualised %).
	Iv *float64 `json:"iv"`
	// DaysToExpiry is calendar days from AsOf to Expiry.
	DaysToExpiry *int `json:"days_to_expiry"`
}

// StockSummaryOptionsFlow is the chain-wide OI + volume aggregate block.
type StockSummaryOptionsFlow struct {
	// TotalCallOi is total open interest across all call contracts.
	TotalCallOi *int `json:"total_call_oi"`
	// TotalPutOi is total open interest across all put contracts.
	TotalPutOi *int `json:"total_put_oi"`
	// TotalCallVolume is total session volume across all calls.
	TotalCallVolume *int `json:"total_call_volume"`
	// TotalPutVolume is total session volume across all puts.
	TotalPutVolume *int `json:"total_put_volume"`
	// PcRatioOi is TotalPutOi / TotalCallOi.
	PcRatioOi *float64 `json:"pc_ratio_oi"`
	// PcRatioVolume is TotalPutVolume / TotalCallVolume.
	PcRatioVolume *float64 `json:"pc_ratio_volume"`
	// ActiveExpirations is the count of expirations with non-zero OI.
	ActiveExpirations *int `json:"active_expirations"`
}

// StockSummaryExposure is the dealer-Greek block.
//
// Nil at the top level when no options data was loaded for this symbol at
// the as-of minute.
type StockSummaryExposure struct {
	// NetGex is net dealer gamma exposure ($/1% spot move).
	NetGex *float64 `json:"net_gex"`
	// NetDex is net dealer delta exposure ($).
	NetDex *float64 `json:"net_dex"`
	// NetVex is net dealer vanna exposure ($/1-vol-point).
	NetVex *float64 `json:"net_vex"`
	// NetChex is net dealer charm exposure ($/day).
	NetChex *float64 `json:"net_chex"`
	// GammaFlip is the strike where net dealer gamma crosses zero.
	GammaFlip *float64 `json:"gamma_flip"`
	// CallWall is the strike with the largest absolute call GEX.
	CallWall *float64 `json:"call_wall"`
	// PutWall is the strike with the largest absolute put GEX.
	PutWall *float64 `json:"put_wall"`
	// MaxPain is the strike where total option-holder intrinsic value is minimized.
	MaxPain *float64 `json:"max_pain"`
	// HighestOiStrike is the strike with the largest total OI.
	HighestOiStrike *float64 `json:"highest_oi_strike"`
	// Regime is the dealer-positioning classifier:
	//   "positive_gamma" | "negative_gamma" | "undetermined"
	Regime string `json:"regime"`
	// Interpretation holds the verbal Greek-regime narratives — safe to surface verbatim.
	Interpretation *StockSummaryInterpretation `json:"interpretation"`
	// HedgingEstimate is the estimated dealer hedging flow at +/- 1% spot moves.
	//
	// IMPORTANT: on this endpoint DealerShares is the MAGNITUDE. The signed
	// direction is carried by Direction. Differs from /v1/exposure/zero-dte.
	HedgingEstimate *StockSummaryHedgingEstimate `json:"hedging_estimate"`
	// ZeroDte is the same-day-expiry attribution block.
	ZeroDte *StockSummaryExposureZeroDte `json:"zero_dte"`
	// TopStrikes is the per-strike top of the dealer-gamma footprint.
	TopStrikes []StockSummaryTopStrike `json:"top_strikes"`
	// OiWeightedDte is the OI-weighted average DTE across the chain.
	OiWeightedDte *float64 `json:"oi_weighted_dte"`
}

// StockSummaryInterpretation holds the verbal Greek-regime narratives.
type StockSummaryInterpretation struct {
	// Gamma is the gamma-regime narrative.
	Gamma string `json:"gamma"`
	// Vanna is the vanna-regime narrative.
	Vanna string `json:"vanna"`
	// Charm is the charm-regime narrative.
	Charm string `json:"charm"`
}

// StockSummaryHedgingMove is one side (up or down) of the dealer-hedging estimate.
//
// IMPORTANT: DealerShares is the MAGNITUDE (always non-negative). The signed
// direction is in Direction.
type StockSummaryHedgingMove struct {
	// DealerShares is the MAGNITUDE of underlying shares dealers must trade.
	DealerShares *float64 `json:"dealer_shares"`
	// Direction is "buy" or "sell" — combine with DealerShares for the signed flow.
	Direction string `json:"direction"`
	// NotionalUsd is DealerShares × spot.
	NotionalUsd *float64 `json:"notional_usd"`
}

// StockSummaryHedgingEstimate holds the estimated dealer hedging flow at +/- 1% spot moves.
type StockSummaryHedgingEstimate struct {
	// SpotUp1Pct is the hedging flow if spot rises 1%.
	SpotUp1Pct *StockSummaryHedgingMove `json:"spot_up_1pct"`
	// SpotDown1Pct is the hedging flow if spot falls 1%.
	SpotDown1Pct *StockSummaryHedgingMove `json:"spot_down_1pct"`
}

// StockSummaryExposureZeroDte is the same-day-expiry contribution block.
type StockSummaryExposureZeroDte struct {
	// NetGex is net 0DTE gamma exposure ($/1% spot move).
	NetGex *float64 `json:"net_gex"`
	// PctOfTotal is the 0DTE share of full-chain net GEX (%).
	PctOfTotal *float64 `json:"pct_of_total"`
	// Expiration is the ISO date (yyyy-MM-dd) of the 0DTE expiry.
	Expiration *string `json:"expiration"`
}

// StockSummaryTopStrike is one row in Exposure.TopStrikes.
type StockSummaryTopStrike struct {
	// Strike is the strike price.
	Strike *float64 `json:"strike"`
	// NetGex is dealer gamma exposure at this strike ($/1% spot move).
	NetGex *float64 `json:"net_gex"`
	// CallOi is open interest on the call side.
	CallOi *int `json:"call_oi"`
	// PutOi is open interest on the put side.
	PutOi *int `json:"put_oi"`
	// TotalOi is CallOi + PutOi.
	TotalOi *int `json:"total_oi"`
}

// StockSummaryMacro is the macro-context block. Each field is independently nullable.
type StockSummaryMacro struct {
	// Vix is the CBOE VIX index level/change/change%.
	Vix *StockSummaryMacroQuote `json:"vix"`
	// Vvix is the VVIX (vol of vol).
	Vvix *StockSummaryMacroQuote `json:"vvix"`
	// Skew is the CBOE SKEW index.
	Skew *StockSummaryMacroQuote `json:"skew"`
	// Spx is the S&P 500 index.
	Spx *StockSummaryMacroQuote `json:"spx"`
	// Move is the ICE BofA MOVE index (Treasury vol).
	Move *StockSummaryMacroQuote `json:"move"`
	// VixTermStructure is the VIX9D / VIX / VIX3M / VIX6M curve.
	VixTermStructure *StockSummaryVixTermStructure `json:"vix_term_structure"`
	// VixFutures is the VIX-futures basis block.
	VixFutures *StockSummaryVixFutures `json:"vix_futures"`
	// FearAndGreed is the CNN Fear & Greed score / rating.
	FearAndGreed *StockSummaryFearAndGreed `json:"fear_and_greed"`
}

// StockSummaryMacroQuote is the value/change/change% triple for each macro index.
type StockSummaryMacroQuote struct {
	// Value is the current index level.
	Value *float64 `json:"value"`
	// Change is the absolute change vs prior session close.
	Change *float64 `json:"change"`
	// ChangePct is the percent change vs prior session close.
	ChangePct *float64 `json:"change_pct"`
}

// StockSummaryVixTermStructure is the VIX term-structure block.
type StockSummaryVixTermStructure struct {
	// Levels are the per-tenor VIX levels.
	Levels *StockSummaryVixTermLevels `json:"levels"`
	// NearSlopePct is (Vix3m - Vix) / Vix * 100.
	NearSlopePct *float64 `json:"near_slope_pct"`
	// Structure is "contango" | "backwardation" | "flat".
	Structure *string `json:"structure"`
}

// StockSummaryVixTermLevels is the VIX9D / VIX / VIX3M / VIX6M tenor levels.
type StockSummaryVixTermLevels struct {
	// Vix9d is CBOE VIX9D.
	Vix9d *float64 `json:"vix_9d"`
	// Vix is CBOE VIX.
	Vix *float64 `json:"vix"`
	// Vix3m is CBOE VIX3M.
	Vix3m *float64 `json:"vix_3m"`
	// Vix6m is CBOE VIX6M.
	Vix6m *float64 `json:"vix_6m"`
}

// StockSummaryVixFutures is the VIX-futures basis block.
//
// IMPORTANT: Basis on this endpoint is APPROXIMATED from VIX3M vs VIX spot —
// it is NOT computed from actual front-month VIX-futures prices.
type StockSummaryVixFutures struct {
	// FrontMonth is the proxy front-month VIX futures level (uses VIX3M).
	FrontMonth *float64 `json:"front_month"`
	// Spot is the VIX spot level.
	Spot *float64 `json:"spot"`
	// Spread is FrontMonth - Spot.
	Spread *float64 `json:"spread"`
	// BasisPct is (FrontMonth - Spot) / Spot * 100.
	BasisPct *float64 `json:"basis_pct"`
	// Basis is "contango" | "backwardation" | "flat".
	Basis *string `json:"basis"`
}

// StockSummaryFearAndGreed is the CNN Fear & Greed indicator.
type StockSummaryFearAndGreed struct {
	// Score is the 0-100 composite (0 = extreme fear, 100 = extreme greed).
	Score *int `json:"score"`
	// Rating is the descriptive label.
	Rating *string `json:"rating"`
}

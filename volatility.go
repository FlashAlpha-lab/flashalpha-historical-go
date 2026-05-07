package flashalphahistorical

// Typed response model for `GET /v1/volatility/{symbol}?at=...` (Growth+).
//
// Same shape as the live API, replayed at the requested historical minute.
//
// Comprehensive volatility analytics for a single underlying. Combines:
//   - the realized-vol ladder (5d/10d/20d/30d/60d),
//   - at-the-money implied vol,
//   - IV-RV spreads (a.k.a. variance risk premium summary, distinct from
//     the dedicated /v1/vrp endpoint),
//   - skew profiles by expiry (10/25-delta wings + smile + tail convexity),
//   - term-structure slope and state,
//   - IV dispersion across strikes/expiries,
//   - GEX and theta bucketed by DTE,
//   - put/call profile by expiry and by moneyness,
//   - OI concentration metrics (top-N % and Herfindahl),
//   - dealer hedging scenarios (shares to hedge ±X% spot moves), and
//   - liquidity (ATM vs wing average bid-ask spreads).
//
// Nullability convention: all numeric fields are *float64 / *int / *string
// so nil represents values the API could not compute (insufficient
// strikes/expiries, sparse OI). Slice fields default to empty (not nil) when
// nothing matches.
//
// Requires Growth+ plan; returns 403 tier_restricted for Basic/Free.

// VolatilityResponse is the typed body of GET /v1/volatility/{symbol}?at=...
type VolatilityResponse struct {
	// Symbol is the underlying ticker echoed from the request path.
	Symbol string `json:"symbol"`
	// UnderlyingPrice is the spot mid at AsOf.
	UnderlyingPrice *float64 `json:"underlying_price"`
	// AsOf is the ET wall-clock timestamp this snapshot was computed for.
	AsOf string `json:"as_of"`
	// MarketOpen is true if NYSE was open at AsOf.
	MarketOpen *bool `json:"market_open"`
	// RealizedVol is the trailing-window realized-vol ladder.
	RealizedVol *VolatilityRealized `json:"realized_vol"`
	// AtmIv is the at-the-money implied volatility (annualised %).
	AtmIv *float64 `json:"atm_iv"`
	// IvRvSpreads is the IV-RV spread block plus a verbal assessment.
	IvRvSpreads *VolatilityIvRvSpreads `json:"iv_rv_spreads"`
	// SkewProfiles is the per-expiry skew profile (10/25-delta wings + smile).
	SkewProfiles []VolatilitySkewProfile `json:"skew_profiles"`
	// TermStructure is the near/far slope and state classification.
	TermStructure *VolatilityTermStructure `json:"term_structure"`
	// IvDispersion measures cross-expiry / cross-strike IV dispersion.
	IvDispersion *VolatilityIvDispersion `json:"iv_dispersion"`
	// GexByDte is the gamma exposure bucketed by days-to-expiry.
	GexByDte []VolatilityGexByDte `json:"gex_by_dte"`
	// ThetaByDte is the theta exposure bucketed by days-to-expiry.
	ThetaByDte []VolatilityThetaByDte `json:"theta_by_dte"`
	// PutCallProfile is the OI/volume put-call profile sliced both ways.
	PutCallProfile *VolatilityPutCallProfile `json:"put_call_profile"`
	// OiConcentration is the top-N OI concentration and Herfindahl index.
	OiConcentration *VolatilityOiConcentration `json:"oi_concentration"`
	// HedgingScenarios are the dealer-hedge scenarios for ±X% spot moves.
	HedgingScenarios []VolatilityHedgingScenario `json:"hedging_scenarios"`
	// Liquidity is the ATM vs wing average bid-ask spread liquidity block.
	Liquidity *VolatilityLiquidity `json:"liquidity"`
}

// VolatilityRealized is the trailing-window realized-vol ladder.
//
// Each field is annualised % (e.g. 18.5 = 18.5%) computed from close-to-close
// log returns over the named trailing window of trading days.
type VolatilityRealized struct {
	// Rv5d is the realized vol over trailing 5 trading days.
	Rv5d *float64 `json:"rv_5d"`
	// Rv10d is the realized vol over trailing 10 trading days.
	Rv10d *float64 `json:"rv_10d"`
	// Rv20d is the realized vol over trailing 20 trading days.
	Rv20d *float64 `json:"rv_20d"`
	// Rv30d is the realized vol over trailing 30 trading days.
	Rv30d *float64 `json:"rv_30d"`
	// Rv60d is the realized vol over trailing 60 trading days.
	Rv60d *float64 `json:"rv_60d"`
}

// VolatilityIvRvSpreads is the IV-RV spread block.
//
// Each Vrp* field is AtmIv minus the matching Rv*d (annualised %). Positive
// means options are pricing more vol than realized → premium for selling
// vol. Assessment is a server-generated verbal label safe to surface verbatim.
type VolatilityIvRvSpreads struct {
	// Vrp5d is AtmIv - RealizedVol.Rv5d (annualised %).
	Vrp5d *float64 `json:"vrp_5d"`
	// Vrp10d is AtmIv - RealizedVol.Rv10d (annualised %).
	Vrp10d *float64 `json:"vrp_10d"`
	// Vrp20d is AtmIv - RealizedVol.Rv20d (annualised %).
	Vrp20d *float64 `json:"vrp_20d"`
	// Vrp30d is AtmIv - RealizedVol.Rv30d (annualised %).
	Vrp30d *float64 `json:"vrp_30d"`
	// Assessment is a plain-English summary of the spread regime.
	// Safe to surface verbatim.
	Assessment *string `json:"assessment"`
}

// VolatilitySkewProfile is one row of the per-expiry skew profile.
//
// Skew25d > 0 means the 25-delta put is bid relative to the 25-delta call
// (downside-skewed smile, the typical equity-index pattern).
type VolatilitySkewProfile struct {
	// Expiry is the option expiration date in YYYY-MM-DD form.
	Expiry *string `json:"expiry"`
	// DaysToExpiry is the integer days from AsOf to Expiry.
	DaysToExpiry *int `json:"days_to_expiry"`
	// Put10dIv is the IV at the 10-delta put wing (annualised %).
	Put10dIv *float64 `json:"put_10d_iv"`
	// Put25dIv is the IV at the 25-delta put wing.
	Put25dIv *float64 `json:"put_25d_iv"`
	// AtmIv is the IV at the at-the-money strike for this expiry.
	AtmIv *float64 `json:"atm_iv"`
	// Call25dIv is the IV at the 25-delta call wing.
	Call25dIv *float64 `json:"call_25d_iv"`
	// Call10dIv is the IV at the 10-delta call wing.
	Call10dIv *float64 `json:"call_10d_iv"`
	// Skew25d is Put25dIv - Call25dIv (the headline 25Δ skew).
	Skew25d *float64 `json:"skew_25d"`
	// SmileRatio is a curvature ratio of wing IVs vs ATM.
	SmileRatio *float64 `json:"smile_ratio"`
	// TailConvexity describes far-tail (10Δ) curvature relative to the smile.
	TailConvexity *float64 `json:"tail_convexity"`
}

// VolatilityTermStructure is the near/far slope and overall state of the
// IV term structure.
type VolatilityTermStructure struct {
	// NearSlopePct is the % slope of IV across near-tenor expiries.
	NearSlopePct *float64 `json:"near_slope_pct"`
	// FarSlopePct is the % slope across far-tenor expiries.
	FarSlopePct *float64 `json:"far_slope_pct"`
	// State is the overall classification (e.g. "contango", "backwardation",
	// "flat"). Safe to surface verbatim.
	State *string `json:"state"`
}

// VolatilityIvDispersion measures how dispersed IV is across the surface.
type VolatilityIvDispersion struct {
	// CrossExpiry is dispersion across expiries at fixed moneyness.
	CrossExpiry *float64 `json:"cross_expiry"`
	// CrossStrike is dispersion across strikes at fixed expiry.
	CrossStrike *float64 `json:"cross_strike"`
}

// VolatilityGexByDte is one bucket of gamma exposure grouped by days-to-expiry.
type VolatilityGexByDte struct {
	// Bucket is the DTE bucket label (e.g. "0", "1-7", "8-30", "31-60").
	Bucket *string `json:"bucket"`
	// NetGex is the net dealer gamma exposure (dollars per 1% spot move) in
	// this bucket.
	NetGex *float64 `json:"net_gex"`
	// PctOfTotal is this bucket's share of total |GEX| across all buckets (0-100).
	PctOfTotal *float64 `json:"pct_of_total"`
	// ContractCount is the count of contracts contributing to this bucket.
	ContractCount *int `json:"contract_count"`
}

// VolatilityThetaByDte is one bucket of theta grouped by days-to-expiry.
type VolatilityThetaByDte struct {
	// Bucket is the DTE bucket label.
	Bucket *string `json:"bucket"`
	// NetTheta is the net dealer theta (dollars per day) in this bucket.
	NetTheta *float64 `json:"net_theta"`
	// ContractCount is the count of contracts contributing to this bucket.
	ContractCount *int `json:"contract_count"`
}

// VolatilityPutCallProfile is the OI/volume put-call profile sliced by
// expiry and by moneyness.
type VolatilityPutCallProfile struct {
	// ByExpiry is the per-expiry put-call breakdown.
	ByExpiry []VolatilityPutCallByExpiry `json:"by_expiry"`
	// ByMoneyness is the chain-wide breakdown by ITM/ATM/OTM bucket.
	ByMoneyness *VolatilityPutCallByMoneyness `json:"by_moneyness"`
}

// VolatilityPutCallByExpiry is one row of the per-expiry put-call breakdown.
//
// Note: on Historical, CallVolume / PutVolume / PcRatioVolume reflect the
// minute-resolution volume snapshot (which may be 0 for sparse-volume
// names); use CallOi / PutOi / PcRatioOi as the primary positioning view.
type VolatilityPutCallByExpiry struct {
	// Expiry is the option expiration date (YYYY-MM-DD).
	Expiry *string `json:"expiry"`
	// CallOi is the total call open interest at this expiry.
	CallOi *int `json:"call_oi"`
	// PutOi is the total put open interest at this expiry.
	PutOi *int `json:"put_oi"`
	// PcRatioOi is PutOi / CallOi for this expiry.
	PcRatioOi *float64 `json:"pc_ratio_oi"`
	// CallVolume is the total call volume at this expiry.
	CallVolume *int `json:"call_volume"`
	// PutVolume is the total put volume at this expiry.
	PutVolume *int `json:"put_volume"`
	// PcRatioVolume is PutVolume / CallVolume for this expiry.
	PcRatioVolume *float64 `json:"pc_ratio_volume"`
}

// VolatilityPutCallByMoneyness is the chain-wide put-call OI breakdown by
// moneyness bucket (ITM / ATM / OTM).
type VolatilityPutCallByMoneyness struct {
	// OtmCallOi is total call OI in out-of-the-money strikes.
	OtmCallOi *int `json:"otm_call_oi"`
	// AtmCallOi is total call OI in at-the-money strikes.
	AtmCallOi *int `json:"atm_call_oi"`
	// ItmCallOi is total call OI in in-the-money strikes.
	ItmCallOi *int `json:"itm_call_oi"`
	// OtmPutOi is total put OI in out-of-the-money strikes.
	OtmPutOi *int `json:"otm_put_oi"`
	// AtmPutOi is total put OI in at-the-money strikes.
	AtmPutOi *int `json:"atm_put_oi"`
	// ItmPutOi is total put OI in in-the-money strikes.
	ItmPutOi *int `json:"itm_put_oi"`
}

// VolatilityOiConcentration measures how concentrated open interest is at
// the top strikes — useful for spotting pin candidates.
type VolatilityOiConcentration struct {
	// Top3Pct is the % of total OI sitting in the top-3 strikes (0-100).
	Top3Pct *float64 `json:"top_3_pct"`
	// Top5Pct is the % of total OI in the top-5 strikes.
	Top5Pct *float64 `json:"top_5_pct"`
	// Top10Pct is the % of total OI in the top-10 strikes.
	Top10Pct *float64 `json:"top_10_pct"`
	// Herfindahl is the Herfindahl-Hirschman index of OI concentration.
	Herfindahl *float64 `json:"herfindahl"`
}

// VolatilityHedgingScenario is one dealer-hedge scenario for a given spot move.
//
// Direction is "buy" or "sell" (qualitative); DealerShares is the magnitude
// of the dealer hedge in shares; NotionalUsd is its dollar notional.
type VolatilityHedgingScenario struct {
	// MovePct is the spot move scenario (e.g. -1, +1, +2 percent).
	MovePct *float64 `json:"move_pct"`
	// DealerShares is the magnitude (always non-negative) of dealer hedge.
	DealerShares *float64 `json:"dealer_shares"`
	// Direction is "buy" or "sell" — the signed direction of the hedge.
	Direction *string `json:"direction"`
	// NotionalUsd is the dollar notional of DealerShares at current spot.
	NotionalUsd *float64 `json:"notional_usd"`
}

// VolatilityLiquidity is the ATM vs wing average bid-ask spread liquidity block.
type VolatilityLiquidity struct {
	// AtmAvgSpreadPct is the average ATM bid-ask spread as a % of mid.
	AtmAvgSpreadPct *float64 `json:"atm_avg_spread_pct"`
	// WingAvgSpreadPct is the average wing (10-25Δ) bid-ask spread as a % of mid.
	WingAvgSpreadPct *float64 `json:"wing_avg_spread_pct"`
	// AtmContracts is the count of ATM contracts considered.
	AtmContracts *int `json:"atm_contracts"`
	// WingContracts is the count of wing contracts considered.
	WingContracts *int `json:"wing_contracts"`
}
